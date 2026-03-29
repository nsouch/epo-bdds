# epo-bdds-cli

Interface en ligne de commande pour l'API EPO BDDS. Retourne du JSON sur stdout, les erreurs sur stderr.

## Installation

```bash
cd cmd
go build -o epo-bdds-cli .
```

## Authentification

Les credentials sont lus depuis les variables d'environnement :

```bash
export EPO_BDDS_USERNAME="mon.compte@example.com"
export EPO_BDDS_PASSWORD="monmotdepasse"
```

Les produits gratuits sont accessibles sans credentials.

## Synopsis

```
epo-bdds-cli [global flags] <commande> [flags de la commande]
```

### Flags globaux

| Flag | Valeurs | Défaut | Description |
|------|---------|--------|-------------|
| `-log-level` | `debug`, `info`, `warn`, `error` | `error` | Niveau de verbosité des logs |
| `-log-format` | `json`, `text` | `text` | Format des logs |
| `-log-file` | chemin | stderr | Fichier de sortie des logs |

---

## Commandes

### `list-products` — Lister les produits

```bash
epo-bdds-cli list-products
```

Sortie :

```json
[
  {
    "id": 1,
    "name": "EP Full Text Data",
    "description": "Full text of EP patent applications"
  },
  {
    "id": 2,
    "name": "EP Citations",
    "description": "Citation data for EP publications"
  }
]
```

---

### `get-product` — Détails d'un produit avec ses livraisons

```bash
epo-bdds-cli get-product -id 1
```

Sortie :

```json
{
  "product_id": 1,
  "name": "EP Full Text Data",
  "description": "Full text of EP patent applications",
  "deliveries": [
    {
      "delivery_id": 42,
      "delivery_name": "2024-10-15",
      "delivery_publication_datetime": "2024-10-15T10:00:00Z",
      "delivery_expiry_datetime": "2025-10-15T10:00:00Z",
      "files": [
        {
          "file_id": 100,
          "file_name": "ep_fulltext_2024-10-15.zip",
          "file_size": "4.2 GB",
          "file_checksum": "sha256:a1b2c3...",
          "file_publication_datetime": "2024-10-15T10:00:00Z"
        }
      ]
    }
  ]
}
```

---

### `find-product` — Rechercher un produit par nom

```bash
epo-bdds-cli find-product -name "EP Full Text Data"
```

Sortie :

```json
{
  "product_id": 1,
  "name": "EP Full Text Data",
  "description": "Full text of EP patent applications"
}
```

---

### `latest-delivery` — Dernière livraison d'un produit

```bash
epo-bdds-cli latest-delivery -id 1
```

Sortie :

```json
{
  "delivery_id": 42,
  "delivery_name": "2024-10-15",
  "delivery_publication_datetime": "2024-10-15T10:00:00Z",
  "delivery_expiry_datetime": "2025-10-15T10:00:00Z",
  "files": [
    {
      "file_id": 100,
      "file_name": "ep_fulltext_2024-10-15.zip",
      "file_size": "4.2 GB",
      "file_checksum": "sha256:a1b2c3...",
      "file_publication_datetime": "2024-10-15T10:00:00Z"
    }
  ]
}
```

---

### `download-file` — Télécharger un fichier

```bash
epo-bdds-cli download-file \
  -product 1 \
  -delivery 42 \
  -file 100 \
  -output /data/ep_fulltext_2024-10-15.zip
```

Sortie :

```json
{
  "output": "/data/ep_fulltext_2024-10-15.zip",
  "size_bytes": 4509715456,
  "duration_ms": 34821
}
```

---

## Codes d'erreur

En cas d'erreur, le programme écrit sur stderr avec un code de sortie non nul :

```json
{"error": "message descriptif", "code": "not_found"}
```

| Code | Signification |
|------|---------------|
| `auth_error` | Credentials invalides ou token expiré (HTTP 401/403) |
| `not_found` | Ressource introuvable (HTTP 404) |
| `rate_limited` | Quota dépassé, réessayer plus tard |
| `error` | Toute autre erreur |

---

## Logging

Par défaut les logs sont supprimés (niveau `error`, sortie stderr). Pour diagnostiquer les appels API :

```bash
# Logs détaillés en texte sur stderr
epo-bdds-cli -log-level debug list-products

# Logs JSON dans un fichier
epo-bdds-cli -log-level info -log-format json -log-file api.log list-products

# Logs debug sur stderr + résultat dans un fichier
epo-bdds-cli -log-level debug list-products > products.json
```

Exemple de log JSON (une ligne par événement) :

```json
{"time":"2024-10-15T10:00:01Z","level":"INFO","msg":"authenticating","username":"mon.compte@example.com"}
{"time":"2024-10-15T10:00:01Z","level":"INFO","msg":"token obtained","expiry":"2024-10-15T11:00:01Z"}
{"time":"2024-10-15T10:00:01Z","level":"DEBUG","msg":"api request","method":"GET","url":"https://bdds.epo.org/api/products"}
{"time":"2024-10-15T10:00:02Z","level":"DEBUG","msg":"api response","method":"GET","url":"https://bdds.epo.org/api/products","status":200,"duration_ms":412}
```

---

## Exemples composés

### Trouver un produit et télécharger sa dernière livraison

```bash
# 1. Récupérer l'ID du produit
PRODUCT_ID=$(epo-bdds-cli find-product -name "EP Citations" | jq '.id')

# 2. Récupérer l'ID de la dernière livraison et du premier fichier
DELIVERY=$(epo-bdds-cli latest-delivery -id "$PRODUCT_ID")
DELIVERY_ID=$(echo "$DELIVERY" | jq '.delivery_id')
FILE_ID=$(echo "$DELIVERY"    | jq '.files[0].file_id')
FILE_NAME=$(echo "$DELIVERY"  | jq -r '.files[0].file_name')

# 3. Télécharger
epo-bdds-cli download-file \
  -product "$PRODUCT_ID" \
  -delivery "$DELIVERY_ID" \
  -file "$FILE_ID" \
  -output "/data/$FILE_NAME"
```

### Lister les fichiers disponibles dans la dernière livraison

```bash
epo-bdds-cli latest-delivery -id 1 | jq '.files[] | {file_id, file_name, file_size}'
```

### Vérifier l'accès avant un téléchargement

```bash
if epo-bdds-cli list-products > /dev/null; then
  echo "Connexion OK"
else
  echo "Échec d'authentification" >&2
  exit 1
fi
```
