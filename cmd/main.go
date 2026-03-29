package main

import (
	"context"
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	bdds "github.com/patent-dev/epo-bdds"
)

// BddsClient is the interface used by CLI commands, allowing mock injection in tests.
type BddsClient interface {
	ListProducts(ctx context.Context) ([]*bdds.Product, error)
	GetProduct(ctx context.Context, productID int) (*bdds.ProductWithDeliveries, error)
	GetProductByName(ctx context.Context, name string) (*bdds.Product, error)
	GetLatestDelivery(ctx context.Context, productID int) (*bdds.Delivery, error)
	DownloadFileWithProgress(ctx context.Context, productID, deliveryID, fileID int, dst io.Writer, progressFn func(bytesWritten, totalBytes int64)) error
}

// --- JSON output types with snake_case tags ---

type productResponse struct {
	ID          int    `json:"product_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type fileResponse struct {
	FileID                  int       `json:"file_id"`
	FileName                string    `json:"file_name"`
	FileSize                string    `json:"file_size"`
	FileChecksum            string    `json:"file_checksum"`
	FilePublicationDatetime time.Time `json:"file_publication_datetime"`
}

type deliveryResponse struct {
	DeliveryID                  int            `json:"delivery_id"`
	DeliveryName                string         `json:"delivery_name"`
	DeliveryPublicationDatetime time.Time      `json:"delivery_publication_datetime"`
	DeliveryExpiryDatetime      *time.Time     `json:"delivery_expiry_datetime,omitempty"`
	Files                       []fileResponse `json:"files"`
}

type productWithDeliveriesResponse struct {
	ID          int                `json:"product_id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Deliveries  []deliveryResponse `json:"deliveries"`
}

type findProductResponse struct {
	ID          int    `json:"product_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type downloadResponse struct {
	ProductID  int    `json:"product_id"`
	DeliveryID int    `json:"delivery_id"`
	FileID     int    `json:"file_id"`
	Output     string `json:"output"`
	SizeBytes  int64  `json:"size_bytes"`
	DurationMs int64  `json:"duration_ms"`
	Checksum   string `json:"checksum"`
}

// --- Converters ---

func toDeliveryResponse(d *bdds.Delivery) deliveryResponse {
	dr := deliveryResponse{
		DeliveryID:                  d.DeliveryID,
		DeliveryName:                d.DeliveryName,
		DeliveryPublicationDatetime: d.DeliveryPublicationDatetime,
		DeliveryExpiryDatetime:      d.DeliveryExpiryDatetime,
		Files:                       make([]fileResponse, len(d.Files)),
	}
	for i, f := range d.Files {
		dr.Files[i] = fileResponse{
			FileID:                  f.FileID,
			FileName:                f.FileName,
			FileSize:                f.FileSize,
			FileChecksum:            f.FileChecksum,
			FilePublicationDatetime: f.FilePublicationDatetime,
		}
	}
	return dr
}

func toProductWithDeliveriesResponse(p *bdds.ProductWithDeliveries) productWithDeliveriesResponse {
	res := productWithDeliveriesResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Deliveries:  make([]deliveryResponse, len(p.Deliveries)),
	}
	for i, d := range p.Deliveries {
		res.Deliveries[i] = toDeliveryResponse(d)
	}
	return res
}

// --- Usage ---

const usageText = `Usage: epo-bdds-cli [global flags] <command> [command flags]

Global flags:
  -log-level string    Log level: debug, info, warn, error (default "error")
  -log-format string   Log format: json, text (default "text")
  -log-file string     Log file path (default: stderr)
  -format string       Output format: json, csv (default "json")
  -csv-separator string  Field separator for CSV output (default ",")

Commands:
  list-products
      List all available products.

  get-product -id <int>
      Get product details including all deliveries.

  find-product -name <string>
      Find a product by name (case-insensitive).

  latest-delivery -id <int>
      Get the latest delivery for a product.

  download-file -product <int> -delivery <int> -file <int> -output <path>
      Download a file to disk and return metadata as JSON.

Authentication:
  Set EPO_BDDS_USERNAME and EPO_BDDS_PASSWORD environment variables.
  Free products work without credentials.
`

// --- Entry points ---

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// clientFactoryFn builds a BddsClient from a logger.
type clientFactoryFn func(logger *slog.Logger) (BddsClient, error)

// defaultClientFactory reads credentials from the environment.
func defaultClientFactory(logger *slog.Logger) (BddsClient, error) {
	config := bdds.DefaultConfig()
	config.Username = os.Getenv("EPO_BDDS_USERNAME")
	config.Password = os.Getenv("EPO_BDDS_PASSWORD")
	config.Logger = logger
	return bdds.NewClient(config)
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithFactory(args, stdout, stderr, defaultClientFactory)
}

func runWithFactory(args []string, stdout, stderr io.Writer, makeClient clientFactoryFn) int {
	gf := flag.NewFlagSet("epo-bdds-cli", flag.ContinueOnError)
	gf.SetOutput(stderr)
	gf.Usage = func() { fmt.Fprint(stderr, usageText) }
	logLevel := gf.String("log-level", "error", "Log level: debug, info, warn, error")
	logFormat := gf.String("log-format", "text", "Log format: json, text")
	logFilePath := gf.String("log-file", "", "Log file path (default: stderr)")
	format := gf.String("format", "json", "Output format: json, csv")
	csvSeparator := gf.String("csv-separator", ",", `Field separator for CSV output (default ",")`)

	if err := gf.Parse(args); err != nil {
		return 1
	}
	if gf.NArg() == 0 {
		fmt.Fprint(stderr, usageText)
		return 1
	}

	logger, closer, err := initLogger(*logLevel, *logFormat, *logFilePath, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if closer != nil {
		defer closer.Close()
	}

	if *format != "json" && *format != "csv" {
		fmt.Fprintf(stderr, "error: invalid format %q: must be json or csv\n", *format)
		return 1
	}

	sepRunes := []rune(*csvSeparator)
	if len(sepRunes) != 1 {
		fmt.Fprintf(stderr, "error: -csv-separator must be a single character\n")
		return 1
	}
	sep := sepRunes[0]
	if sep == '"' || sep == '\r' || sep == '\n' || sep == 0 {
		fmt.Fprintf(stderr, "error: -csv-separator %q is not a valid field separator\n", sep)
		return 1
	}

	command := gf.Arg(0)
	cmdArgs := gf.Args()[1:]

	client, err := makeClient(logger)
	if err != nil {
		return writeError(stderr, "error", err.Error())
	}

	logger.Info("executing command", "command", command, "args", cmdArgs, "full_command", strings.Join(args, " "))

	switch command {
	case "list-products":
		return runListProducts(client, cmdArgs, stdout, stderr, *format, sep, logger)
	case "get-product":
		return runGetProduct(client, cmdArgs, stdout, stderr, *format, sep, logger)
	case "find-product":
		return runFindProduct(client, cmdArgs, stdout, stderr, *format, sep, logger)
	case "latest-delivery":
		return runLatestDelivery(client, cmdArgs, stdout, stderr, *format, sep, logger)
	case "download-file":
		return runDownloadFile(client, cmdArgs, stdout, stderr, *format, sep, logger)
	default:
		fmt.Fprintf(stderr, "unknown command: %q\n\n", command)
		fmt.Fprint(stderr, usageText)
		return 1
	}
}

// initLogger creates a slog.Logger.
// When filePath is empty, logs are written to defaultWriter (typically stderr).
func initLogger(level, format, filePath string, defaultWriter io.Writer) (*slog.Logger, io.Closer, error) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		return nil, nil, fmt.Errorf("invalid log level %q: must be debug, info, warn, or error", level)
	}

	var w io.Writer = defaultWriter
	var closer io.Closer
	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot open log file: %w", err)
		}
		w = f
		closer = f
	}

	opts := &slog.HandlerOptions{Level: logLevel}
	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(w, opts)
	case "text":
		handler = slog.NewTextHandler(w, opts)
	default:
		if closer != nil {
			_ = closer.Close()
		}
		return nil, nil, fmt.Errorf("invalid log format %q: must be json or text", format)
	}

	return slog.New(handler), closer, nil
}

// writeJSON marshals v as indented JSON to w. Returns 0 on success, 1 on error.
func writeJSON(w io.Writer, v any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return 1
	}
	return 0
}

// writeError writes {"error":message,"code":code} as JSON to w and returns exit code 1.
func writeError(w io.Writer, code, message string) int {
	enc := json.NewEncoder(w)
	_ = enc.Encode(map[string]string{"error": message, "code": code})
	return 1
}

// errorCode maps bdds error types to JSON error code strings.
func errorCode(err error) string {
	switch err.(type) {
	case *bdds.AuthError:
		return "auth_error"
	case *bdds.NotFoundError:
		return "not_found"
	case *bdds.RateLimitError:
		return "rate_limited"
	default:
		return "error"
	}
}

// --- Sub-command implementations ---

func runListProducts(client BddsClient, args []string, stdout, stderr io.Writer, format string, sep rune, logger *slog.Logger) int {
	fs := flag.NewFlagSet("list-products", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}

	logger.Info("listing products")
	products, err := client.ListProducts(context.Background())
	if err != nil {
		logger.Error("list products failed", "error", err)
		return writeError(stderr, errorCode(err), err.Error())
	}

	out := make([]productResponse, len(products))
	for i, p := range products {
		out[i] = productResponse{ID: p.ID, Name: p.Name, Description: p.Description}
	}
	logger.Info("list products success", "count", len(products))
	return writeOutput(stdout, format, sep, out)
}

func runGetProduct(client BddsClient, args []string, stdout, stderr io.Writer, format string, sep rune, logger *slog.Logger) int {
	fs := flag.NewFlagSet("get-product", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.Int("id", 0, "Product ID (required)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == 0 {
		fmt.Fprintln(stderr, "error: -id flag is required")
		fs.Usage()
		return 1
	}

	logger.Info("getting product", "id", *id)
	product, err := client.GetProduct(context.Background(), *id)
	if err != nil {
		logger.Error("get product failed", "id", *id, "error", err)
		return writeError(stderr, errorCode(err), err.Error())
	}
	logger.Info("get product success", "id", *id, "deliveries", len(product.Deliveries))
	return writeOutput(stdout, format, sep, toProductWithDeliveriesResponse(product))
}

func runFindProduct(client BddsClient, args []string, stdout, stderr io.Writer, format string, sep rune, logger *slog.Logger) int {
	fs := flag.NewFlagSet("find-product", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "Product name (required)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *name == "" {
		fmt.Fprintln(stderr, "error: -name flag is required")
		fs.Usage()
		return 1
	}

	logger.Info("finding product", "name", *name)
	product, err := client.GetProductByName(context.Background(), *name)
	if err != nil {
		logger.Error("find product failed", "name", *name, "error", err)
		return writeError(stderr, errorCode(err), err.Error())
	}
	logger.Info("find product success", "id", product.ID)
	return writeOutput(stdout, format, sep, findProductResponse{
		ID:          product.ID,
		Name:        product.Name,
		Description: product.Description,
	})
}

func runLatestDelivery(client BddsClient, args []string, stdout, stderr io.Writer, format string, sep rune, logger *slog.Logger) int {
	fs := flag.NewFlagSet("latest-delivery", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.Int("id", 0, "Product ID (required)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == 0 {
		fmt.Fprintln(stderr, "error: -id flag is required")
		fs.Usage()
		return 1
	}

	logger.Info("getting latest delivery", "product_id", *id)
	product, err := client.GetProduct(context.Background(), *id)
	if err != nil {
		logger.Error("get product failed", "product_id", *id, "error", err)
		return writeError(stderr, errorCode(err), err.Error())
	}

	if len(product.Deliveries) == 0 {
		return writeError(stderr, "not_found", fmt.Sprintf("product %d has no deliveries", *id))
	}

	latest := product.Deliveries[0]
	for _, d := range product.Deliveries[1:] {
		if d.DeliveryPublicationDatetime.After(latest.DeliveryPublicationDatetime) {
			latest = d
		}
	}

	logger.Info("get latest delivery success", "delivery_id", latest.DeliveryID)
	return writeOutput(stdout, format, sep, productWithDeliveriesResponse{
		ID:          product.ID,
		Name:        product.Name,
		Description: product.Description,
		Deliveries:  []deliveryResponse{toDeliveryResponse(latest)},
	})
}

func runDownloadFile(client BddsClient, args []string, stdout, stderr io.Writer, format string, sep rune, logger *slog.Logger) int {
	fs := flag.NewFlagSet("download-file", flag.ContinueOnError)
	fs.SetOutput(stderr)
	productID := fs.Int("product", 0, "Product ID (required)")
	deliveryID := fs.Int("delivery", 0, "Delivery ID (required)")
	fileID := fs.Int("file", 0, "File ID (required)")
	output := fs.String("output", "", "Output file path (required)")
	expectedChecksum := fs.String("checksum", "", "Expected SHA1 checksum for verification (optional)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *productID == 0 {
		fmt.Fprintln(stderr, "error: -product flag is required")
		fs.Usage()
		return 1
	}
	if *deliveryID == 0 {
		fmt.Fprintln(stderr, "error: -delivery flag is required")
		fs.Usage()
		return 1
	}
	if *fileID == 0 {
		fmt.Fprintln(stderr, "error: -file flag is required")
		fs.Usage()
		return 1
	}
	if *output == "" {
		fmt.Fprintln(stderr, "error: -output flag is required")
		fs.Usage()
		return 1
	}

	f, err := os.Create(*output)
	if err != nil {
		return writeError(stderr, "error", fmt.Sprintf("cannot create output file: %v", err))
	}
	defer f.Close()

	logger.Info("downloading file",
		"product", *productID,
		"delivery", *deliveryID,
		"file", *fileID,
		"output", *output,
	)
	hasher := sha1.New()
	start := time.Now()
	var lastPct int64
	err = client.DownloadFileWithProgress(
		context.Background(),
		*productID, *deliveryID, *fileID, io.MultiWriter(f, hasher),
		func(written, total int64) {
			if total <= 0 {
				return
			}
			pct := written * 10 / total * 10 // nearest lower 10%
			if pct > lastPct {
				lastPct = pct
				logger.Debug("download progress", "percent", pct, "written", written, "total", total)
			}
		},
	)
	if err != nil {
		_ = os.Remove(*output)
		logger.Error("download failed", "error", err)
		return writeError(stderr, errorCode(err), err.Error())
	}

	streamChecksum := strings.ToUpper(hex.EncodeToString(hasher.Sum(nil)))
	expected := strings.ToUpper(*expectedChecksum)

	if expected != "" && !strings.EqualFold(streamChecksum, expected) {
		_ = os.Remove(*output)
		logger.Error("stream checksum mismatch",
			"expected", expected,
			"stream_checksum", streamChecksum,
		)
		return writeError(stderr, "checksum_mismatch",
			fmt.Sprintf("stream checksum mismatch: expected %s, got %s", expected, streamChecksum))
	}

	// Flush to disk, then re-hash from disk to detect write corruption.
	if err := f.Sync(); err != nil {
		_ = os.Remove(*output)
		return writeError(stderr, "error", fmt.Sprintf("cannot sync output file: %v", err))
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = os.Remove(*output)
		return writeError(stderr, "error", fmt.Sprintf("cannot seek output file: %v", err))
	}
	diskHasher := sha1.New()
	if _, err := io.Copy(diskHasher, f); err != nil {
		_ = os.Remove(*output)
		return writeError(stderr, "error", fmt.Sprintf("cannot hash output file: %v", err))
	}

	diskChecksum := strings.ToUpper(hex.EncodeToString(diskHasher.Sum(nil)))

	if diskChecksum != streamChecksum {
		_ = os.Remove(*output)
		logger.Error("disk write corruption detected",
			"stream_checksum", streamChecksum,
			"disk_checksum", diskChecksum,
		)
		return writeError(stderr, "download_corrupted",
			fmt.Sprintf("disk checksum %s differs from stream checksum %s", diskChecksum, streamChecksum))
	}

	computedChecksum := diskChecksum
	logger.Debug("expected checksum", "expected", expected)
	logger.Debug("stream checksum", "stream_checksum", streamChecksum)
	logger.Debug("disk checksum", "disk_checksum", diskChecksum)

	var sizeBytes int64
	if fi, statErr := f.Stat(); statErr == nil {
		sizeBytes = fi.Size()
	}
	duration := time.Since(start)

	logger.Info("download complete",
		"output", *output,
		"size_bytes", sizeBytes,
		"duration_ms", duration.Milliseconds(),
		"checksum", computedChecksum,
	)
	return writeOutput(stdout, format, sep, downloadResponse{
		ProductID:  *productID,
		DeliveryID: *deliveryID,
		FileID:     *fileID,
		Output:     *output,
		SizeBytes:  sizeBytes,
		DurationMs: duration.Milliseconds(),
		Checksum:   computedChecksum,
	})
}

// writeOutput writes v to w in the specified format ("json" or "csv").
func writeOutput(w io.Writer, format string, sep rune, v any) int {
	if format == "csv" {
		return writeCSV(w, sep, v)
	}
	return writeJSON(w, v)
}

// writeCSV serialises v as CSV to w.
// Nested structures (deliveries + files) are flattened to one row per file.
// A delivery with no files produces one row with empty file fields.
func writeCSV(w io.Writer, sep rune, v any) int {
	cw := csv.NewWriter(w)
	cw.Comma = sep
	switch val := v.(type) {
	case []productResponse:
		_ = cw.Write([]string{"product_id", "name", "description"})
		for _, p := range val {
			_ = cw.Write([]string{strconv.Itoa(p.ID), p.Name, p.Description})
		}
	case productWithDeliveriesResponse:
		_ = cw.Write([]string{
			"product_id", "name", "description",
			"delivery_id", "delivery_name", "delivery_publication_datetime", "delivery_expiry_datetime",
			"file_id", "file_name", "file_size", "file_checksum", "file_publication_datetime",
		})
		for _, d := range val.Deliveries {
			expiry := ""
			if d.DeliveryExpiryDatetime != nil {
				expiry = d.DeliveryExpiryDatetime.Format(time.RFC3339)
			}
			if len(d.Files) == 0 {
				_ = cw.Write([]string{
					strconv.Itoa(val.ID), val.Name, val.Description,
					strconv.Itoa(d.DeliveryID), d.DeliveryName, d.DeliveryPublicationDatetime.Format(time.RFC3339), expiry,
					"", "", "", "", "",
				})
				continue
			}
			for _, f := range d.Files {
				_ = cw.Write([]string{
					strconv.Itoa(val.ID), val.Name, val.Description,
					strconv.Itoa(d.DeliveryID), d.DeliveryName, d.DeliveryPublicationDatetime.Format(time.RFC3339), expiry,
					strconv.Itoa(f.FileID), f.FileName, f.FileSize, f.FileChecksum, f.FilePublicationDatetime.Format(time.RFC3339),
				})
			}
		}
	case findProductResponse:
		_ = cw.Write([]string{"product_id", "name", "description"})
		_ = cw.Write([]string{strconv.Itoa(val.ID), val.Name, val.Description})
	case downloadResponse:
		_ = cw.Write([]string{"product_id", "delivery_id", "file_id", "output", "size_bytes", "duration_ms", "checksum"})
		_ = cw.Write([]string{
			strconv.Itoa(val.ProductID), strconv.Itoa(val.DeliveryID), strconv.Itoa(val.FileID),
			val.Output, strconv.FormatInt(val.SizeBytes, 10), strconv.FormatInt(val.DurationMs, 10), val.Checksum,
		})
	default:
		_, _ = fmt.Fprintln(w, "csv: unsupported response type")
		return 1
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return 1
	}
	return 0
}
