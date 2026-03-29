#!/usr/bin/env bash
# capture_outputs.sh
# Runs each epo-bdds-cli command and saves the outputs to JSON files.
# Usage: EPO_BDDS_USERNAME=xxx EPO_BDDS_PASSWORD=yyy bash capture_outputs.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="$SCRIPT_DIR/downloads"

# --- Load .env file ---
# Looks for cmd/.env first, then the parent directory (repository root)
for env_file in "$SCRIPT_DIR/../.env"; do
  if [[ -f "$env_file" ]]; then
    echo "==> Loading credentials from $env_file"
    # Skip blank lines and comments; only export KEY=VALUE entries
    set -o allexport
    # shellcheck source=/dev/null
    source "$env_file"
    set +o allexport
    break
  fi
done

# Check that credentials are set (either from .env or the environment)
if [[ -z "${EPO_BDDS_USERNAME:-}" || -z "${EPO_BDDS_PASSWORD:-}" ]]; then
  echo "ERROR: EPO_BDDS_USERNAME and EPO_BDDS_PASSWORD are required." >&2
  echo "  Option 1: create .env in the repository root with:" >&2
  echo "    EPO_BDDS_USERNAME=your.account@example.com" >&2
  echo "    EPO_BDDS_PASSWORD=yourpassword" >&2
  echo "  Option 2: pass them as a command prefix:" >&2
  echo "    EPO_BDDS_USERNAME=xxx EPO_BDDS_PASSWORD=yyy bash capture_outputs.sh" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"
echo "==> Output will be saved to $OUT_DIR"

# --- Build ---
echo "==> Building epo-bdds-cli..."
cd "$SCRIPT_DIR"
go build -o epo-bdds-cli .
echo "    OK"

mkdir -p "$OUT_DIR"

# --- Helpers ---

run_cmd() {
  local label="$1"
  local out_file="$OUT_DIR/$2"
  local log_file="${out_file%.json}.log"
  shift 2
  echo "==> $label"
  echo "    cmd: epo-bdds-cli $*"
  if ./epo-bdds-cli \
      -log-level debug \
      -log-format text \
      -log-file "$log_file" \
      "$@" > "$out_file" 2>"$OUT_DIR/stderr_tmp"; then
    echo "    OK -> $out_file (logs: $log_file)"
  else
    echo "    ERROR (exit code $?):"
    cat "$OUT_DIR/stderr_tmp" >&2
    return 1
  fi
}

# --- list-products ---
run_cmd "list-products" "list_products.json" \
  list-products

# Pick the first available product for subsequent commands
PRODUCT_ID=$(jq '.[0].product_id' "$OUT_DIR/list_products.json")
PRODUCT_NAME=$(jq -r '.[0].name' "$OUT_DIR/list_products.json")
echo "    First product: id=$PRODUCT_ID name=\"$PRODUCT_NAME\""

# --- get-product ---
run_cmd "get-product (id=$PRODUCT_ID)" "get_product.json" \
  get-product -id "$PRODUCT_ID"

# --- find-product ---
run_cmd "find-product (name=\"$PRODUCT_NAME\")" "find_product.json" \
  find-product -name "$PRODUCT_NAME"

# --- latest-delivery ---
run_cmd "latest-delivery (product=$PRODUCT_ID)" "latest_delivery.json" \
  latest-delivery -id "$PRODUCT_ID"

# --- download-file (premier fichier de la dernière livraison) ---
DELIVERY_ID=$(jq '.deliveries[0].delivery_id' "$OUT_DIR/latest_delivery.json")
FILE_ID=$(jq '.deliveries[0].files[0].file_id' "$OUT_DIR/latest_delivery.json")
FILE_NAME=$(jq -r '.deliveries[0].files[0].file_name' "$OUT_DIR/latest_delivery.json")
FILE_CHECKSUM=$(jq -r '.deliveries[0].files[0].file_checksum' "$OUT_DIR/latest_delivery.json")
echo "    Latest delivery: delivery_id=$DELIVERY_ID file_id=$FILE_ID name=\"$FILE_NAME\" checksum=$FILE_CHECKSUM"

DOWNLOAD_OUTPUT="$OUT_DIR/$FILE_NAME"
run_cmd "download-file (product=$PRODUCT_ID delivery=$DELIVERY_ID file=$FILE_ID)" "download_file.json" \
  download-file \
    -product "$PRODUCT_ID" \
    -delivery "$DELIVERY_ID" \
    -file "$FILE_ID" \
    -output "$DOWNLOAD_OUTPUT" \
    -checksum "$FILE_CHECKSUM"
echo "    Downloaded file saved to $DOWNLOAD_OUTPUT"
