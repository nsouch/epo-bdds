package bdds

import (
	"encoding/json"
	"io"
)

// obfuscateUsername keeps the first 2 and last 2 characters of s,
// replacing the middle with X's. Short strings are fully masked.
func obfuscateUsername(s string) string {
	r := []rune(s)
	n := len(r)
	if n <= 4 {
		return "XXXX"
	}
	return string(r[:2]) + "XXXX" + string(r[n-2:])
}

// readJSON reads and unmarshals JSON from a reader
func readJSON(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}

// progressReader wraps an io.Reader to track progress
type progressReader struct {
	reader     io.Reader
	total      int64
	current    int64
	progressFn func(bytesWritten, totalBytes int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)
	if pr.progressFn != nil {
		pr.progressFn(pr.current, pr.total)
	}
	return n, err
}
