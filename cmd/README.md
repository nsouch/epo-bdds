# epo-bdds-cli

Command-line interface for the EPO BDDS API. Returns JSON on stdout, errors on stderr.

## Installation

```bash
cd cmd
go build -o epo-bdds-cli .
```

## Authentication

Credentials are read from environment variables:

```bash
export EPO_BDDS_USERNAME="my.account@example.com"
export EPO_BDDS_PASSWORD="mypassword"
```

Free products are accessible without credentials.

## Synopsis

```
epo-bdds-cli [global flags] <command> [command flags]
```

### Global flags

| Flag | Values | Default | Description |
|------|--------|---------|-------------|
| `-log-level` | `debug`, `info`, `warn`, `error` | `error` | Log verbosity level |
| `-log-format` | `json`, `text` | `text` | Log format |
| `-log-file` | path | stderr | Log output file |

---

## Commands

### `list-products` — List products

```bash
epo-bdds-cli list-products
```

Output:

```json
[
  {
    "product_id": 5,
    "name": "14.11 EPO worldwide legal event data (INPADOC) - front file",
    "description": "The product 14.11 contains legal event data and includes records from over 60 international patent authorities. INPADOC back file is available annually ."
  },
  {
    "product_id": 3,
    "name": "14.7  EPO worldwide bibliographic data (DOCDB) - front file",
    "description": "DOCDB - EPO worldwide bibliographic data is an extraction in XML format of our master documentation database with worldwide coverage containing bibliographic data, abstracts and citations (but no full text). This is a front file, back file data is available under the DOCDB back file folder."
  },
  {
    "product_id": 32,
    "name": "14.12 EP full-text data",
    "description": "EP full-text data contains all EP-A and EP-B publications published by EPO from the 1970s to date. Newly published ZIP files with PDF/A and XML will be accessible every Wednesday at 14:00 CET/CEST."
  }
]
```

_(12 products total in the actual response)_

---

### `get-product` — Product details with its deliveries

```bash
epo-bdds-cli get-product -id 5
```

Output:

```json
{
  "product_id": 5,
  "name": "14.11 EPO worldwide legal event data (INPADOC) - front file",
  "description": "The product 14.11 contains legal event data and includes records from over 60 international patent authorities. INPADOC back file is available annually .",
  "deliveries": [
    {
      "delivery_id": 3134,
      "delivery_name": "14.11 INPADOC - EPO worldwide legal event data 2026/013",
      "delivery_publication_datetime": "2026-03-24T09:00:00+01:00",
      "files": [
        {
          "file_id": 9195,
          "file_name": "legstat_xml_202613.zip",
          "file_size": "318.5 MB",
          "file_checksum": "8AB0B4AD3D3623DF3797D63DA6770EE9CB849E04",
          "file_publication_datetime": "2026-03-24T09:00:00+01:00"
        },
        {
          "file_id": 9196,
          "file_name": "statistics_authority_code_202613.xlsx",
          "file_size": "450.1 kB",
          "file_checksum": "B4AEF885E8665A491EA0C878CEE86B4B82007C76",
          "file_publication_datetime": "2026-03-24T09:00:00+01:00"
        }
      ]
    }
  ]
}
```

_(The `delivery_expiry_datetime` field is omitted when absent. Product 5 has 66 deliveries in the actual response.)_

---

### `find-product` — Search for a product by name

```bash
epo-bdds-cli find-product -name "14.11 EPO worldwide legal event data (INPADOC) - front file"
```

Output:

```json
{
  "product_id": 5,
  "name": "14.11 EPO worldwide legal event data (INPADOC) - front file",
  "description": "The product 14.11 contains legal event data and includes records from over 60 international patent authorities. INPADOC back file is available annually ."
}
```

---

### `latest-delivery` — Latest delivery for a product

```bash
epo-bdds-cli latest-delivery -id 5
```

Output:

```json
{
  "product_id": 5,
  "name": "14.11 EPO worldwide legal event data (INPADOC) - front file",
  "description": "The product 14.11 contains legal event data and includes records from over 60 international patent authorities. INPADOC back file is available annually .",
  "deliveries": [
    {
      "delivery_id": 3134,
      "delivery_name": "14.11 INPADOC - EPO worldwide legal event data 2026/013",
      "delivery_publication_datetime": "2026-03-24T09:00:00+01:00",
      "files": [
        {
          "file_id": 9195,
          "file_name": "legstat_xml_202613.zip",
          "file_size": "318.5 MB",
          "file_checksum": "8AB0B4AD3D3623DF3797D63DA6770EE9CB849E04",
          "file_publication_datetime": "2026-03-24T09:00:00+01:00"
        },
        ...
        {
          "file_id": 9198,
          "file_name": "INPADOC_coverage_202613.xlsx",
          "file_size": "870.8 kB",
          "file_checksum": "F18723C6425425E2D25377DF57A75D7D61B104A8",
          "file_publication_datetime": "2026-03-24T09:00:00+01:00"
        }
      ]
    }
  ]
}
```

---

### `download-file` — Download a file

```bash
epo-bdds-cli download-file \
  -product 5 \
  -delivery 3134 \
  -file 9195 \
  -output /data/legstat_xml_202613.zip
```

Optional flag `-checksum <SHA1>`: if provided, the stream checksum is verified before writing to disk.

```bash
epo-bdds-cli download-file \
  -product 5 \
  -delivery 3134 \
  -file 9195 \
  -output /data/legstat_xml_202613.zip \
  -checksum 8AB0B4AD3D3623DF3797D63DA6770EE9CB849E04
```

Output:

```json
{
  "product_id": 5,
  "delivery_id": 3134,
  "file_id": 9195,
  "output": "/data/legstat_xml_202613.zip",
  "size_bytes": 318466085,
  "duration_ms": 8681,
  "checksum": "8AB0B4AD3D3623DF3797D63DA6770EE9CB849E04"
}
```

The download computes a SHA1 checksum on the fly (stream) and re-verifies it from disk to detect any write corruption.

---

## Error codes

On error, the program writes to stderr with a non-zero exit code:

```json
{"error": "descriptive message", "code": "not_found"}
```

| Code | Meaning |
|------|---------|
| `auth_error` | Invalid credentials or expired token (HTTP 401/403) |
| `not_found` | Resource not found (HTTP 404) |
| `rate_limited` | Quota exceeded, retry later |
| `checksum_mismatch` | Stream checksum does not match the provided `-checksum` |
| `download_corrupted` | Disk checksum differs from stream checksum (write corruption) |
| `error` | Any other error |

### Examples

**File not found** — downloading with an unknown file ID (the request is retried 3 times before failing):

```bash
$ ./epo-bdds-cli -log-level debug download-file -product 5 -delivery 3134 -file 9999 -output /data/out.zip
time=2026-03-29T11:22:46.224+02:00 level=DEBUG msg="api response" ... status=404 duration_ms=2328
time=2026-03-29T11:22:46.224+02:00 level=WARN msg="retrying request" attempt=1 max_retries=3 error="file not found: 5/3134/9999"
time=2026-03-29T11:22:47.319+02:00 level=WARN msg="retrying request" attempt=2 max_retries=3 error="file not found: 5/3134/9999"
time=2026-03-29T11:22:49.420+02:00 level=WARN msg="retrying request" attempt=3 max_retries=3 error="file not found: 5/3134/9999"
time=2026-03-29T11:22:52.523+02:00 level=ERROR msg="download failed" error="failed after 3 retries: file not found: 5/3134/9999"
{"code":"error","error":"failed after 3 retries: file not found: 5/3134/9999"}
```

**Access denied** — requesting a paid product without credentials (HTTP 401 after retries):

```bash
$ ./epo-bdds-cli -log-level debug get-product -id 9999
time=2026-03-29T11:24:41.430+02:00 level=DEBUG msg="api response" ... status=401 duration_ms=250
time=2026-03-29T11:24:41.430+02:00 level=WARN msg="retrying request" attempt=1 max_retries=3 error="unexpected status 401: {\"code\":9000,\"message\":\"Access Denied\",...}"
time=2026-03-29T11:24:42.539+02:00 level=WARN msg="retrying request" attempt=2 max_retries=3 error="unexpected status 401: ..."
time=2026-03-29T11:24:44.664+02:00 level=WARN msg="retrying request" attempt=3 max_retries=3 error="unexpected status 401: ..."
time=2026-03-29T11:24:47.779+02:00 level=ERROR msg="get product failed" id=9999 error="failed after 3 retries: unexpected status 401: ..."
{"code":"error","error":"failed after 3 retries: unexpected status 401: {\"code\":9000,\"message\":\"Access Denied\",\"description\":\"\",\"timestamp\":\"2026-03-29T11:24:47.823+02:00\"}"}
```

**Missing required flag** — validation error before any API call is made (plain text on stderr, exit code 1):

```bash
$ ./epo-bdds-cli download-file -product 5 -delivery 3134
error: -file flag is required
Usage of download-file:
  -checksum string
        Expected SHA1 checksum for verification (optional)
  -delivery int
        Delivery ID (required)
  -file int
        File ID (required)
  -output string
        Output file path (required)
  -product int
        Product ID (required)
```

---

## Logging

By default logs are suppressed (level `error`, output stderr). To diagnose API calls:

```bash
# Detailed text logs on stderr
epo-bdds-cli -log-level debug list-products

# JSON logs to a file
epo-bdds-cli -log-level info -log-format json -log-file api.log list-products

# Debug logs on stderr + result to a file
epo-bdds-cli -log-level debug list-products > products.json
```

Example JSON log (one line per event):

```json
{"time":"2026-03-29T11:04:01.797+02:00","level":"INFO","msg":"executing command","command":"list-products"}
{"time":"2026-03-29T11:04:01.798+02:00","level":"INFO","msg":"authenticating","username":"my.account@example.com"}
{"time":"2026-03-29T11:04:02.346+02:00","level":"INFO","msg":"token obtained","expiry":"2026-03-29T12:04:02.346+02:00"}
{"time":"2026-03-29T11:04:02.346+02:00","level":"DEBUG","msg":"api request","method":"GET","url":"https://publication-bdds.apps.epo.org/bdds/bdds-bff-service/prod/api/products/"}
{"time":"2026-03-29T11:04:02.566+02:00","level":"DEBUG","msg":"api response","method":"GET","url":"https://publication-bdds.apps.epo.org/bdds/bdds-bff-service/prod/api/products/","status":200,"duration_ms":219}
```

---

## Composed examples

### Find a product and download its latest delivery

```bash
# 1. Get the product ID
PRODUCT_ID=$(epo-bdds-cli find-product -name "14.11 EPO worldwide legal event data (INPADOC) - front file" | jq '.product_id')

# 2. Get the latest delivery ID and first file ID
DELIVERY=$(epo-bdds-cli latest-delivery -id "$PRODUCT_ID")
DELIVERY_ID=$(echo "$DELIVERY" | jq '.deliveries[0].delivery_id')
FILE_ID=$(echo "$DELIVERY"     | jq '.deliveries[0].files[0].file_id')
FILE_NAME=$(echo "$DELIVERY"   | jq -r '.deliveries[0].files[0].file_name')

# 3. Download
epo-bdds-cli download-file \
  -product "$PRODUCT_ID" \
  -delivery "$DELIVERY_ID" \
  -file "$FILE_ID" \
  -output "/data/$FILE_NAME"
```

### List available files in the latest delivery

```bash
epo-bdds-cli latest-delivery -id 5 | jq '.deliveries[0].files[] | {file_id, file_name, file_size}'
```

### Check access before downloading

```bash
if epo-bdds-cli list-products > /dev/null; then
  echo "Connection OK"
else
  echo "Authentication failed" >&2
  exit 1
fi
```
