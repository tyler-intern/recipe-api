# recipe-api

Go REST API for the Recipe Box full-stack app. This service is the only layer that talks to MongoDB; the [`recipe-web`](../recipe-web) frontend calls it over HTTP.

**Default URL:** `http://localhost:3001`

## Prerequisites

- Go 1.25+
- MongoDB running locally (port `27017`)

Start MongoDB via Homebrew:

```bash
brew services start mongodb-community@7.0
```

Or via Docker (from the repo root):

```bash
docker compose up -d
```

Verify it's up:

```bash
mongosh --eval "db.runCommand({ ping: 1 })"
# { ok: 1 }
```

## Setup

1. **Enter the folder**

   ```bash
   cd recipe-api
   ```

2. **Configure environment**

   ```bash
   cp .env.example .env
   ```

   Edit `.env`:

   | Variable | Example | Purpose |
   | -------- | ------- | ------- |
   | `PORT` | `3001` | HTTP listen port |
   | `MONGODB_URI` | `mongodb://127.0.0.1:27017/recipe_box` | MongoDB connection |

   Go does not load `.env` automatically. Before running, export the variables:

   ```bash
   set -a && source .env && set +a
   ```

3. **Download dependencies** (first run only)

   ```bash
   go mod download
   ```

## Run

```bash
go run .
```

You should see:

```
Connected to MongoDB
API listening on http://localhost:3001
```

**Smoke test:**

```bash
curl -s http://localhost:3001/health
# {"ok":true}
```

## API endpoints

| Action | Method | Path | Notes |
| ------ | ------ | ---- | ----- |
| Health | GET | `/health` | `{"ok":true}` |
| List | GET | `/recipes` | JSON array, newest first; optional `?tag=soup` filter |
| Create | POST | `/recipes` | Full body required → `201` |
| Get one | GET | `/recipes/:id` | Single recipe with nested arrays |
| Update | PATCH | `/recipes/:id` | Partial update; only sent fields change |
| Delete | DELETE | `/recipes/:id` | `204 No Content` |

**Status codes:** `200`, `201`, `204`, `400`, `404`, `405`, `500`

### curl examples

```bash
# List
curl -s http://localhost:3001/recipes

# Filter by tag
curl -s "http://localhost:3001/recipes?tag=soup"

# Create
curl -s -X POST http://localhost:3001/recipes \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Tomato soup",
    "prepTimeMinutes": 25,
    "tags": ["soup", "vegetarian"],
    "ingredients": [
      { "item": "canned tomatoes", "amount": "2", "unit": "cans" }
    ],
    "steps": ["Simmer.", "Blend until smooth."]
  }'

# Get one
curl -s http://localhost:3001/recipes/<id>

# Update
curl -s -X PATCH http://localhost:3001/recipes/<id> \
  -H "Content-Type: application/json" \
  -d '{"title":"Roasted tomato soup"}'

# Delete
curl -s -X DELETE http://localhost:3001/recipes/<id> -w "\n%{http_code}\n"
```

## Data model

Database: `recipe_box` · Collection: `recipes` · One document per recipe with embedded `ingredients` and `steps` arrays.

Full field reference: [`../docs/data-model.md`](../docs/data-model.md).

### Validation

- `title` required, non-empty after trim
- `ingredients` array length ≥ 1; each has non-empty `item`, `amount`, `unit`
- `steps` array length ≥ 1; each non-empty after trim
- `prepTimeMinutes` and `servings` ≥ 0 when provided

Validation failures return `400` with `{"error":"..."}`.

## CORS

Allows `http://localhost:3000` so the Next.js dev app can `fetch()` this API from the browser.

## Related

- [`../recipe-web`](../recipe-web) — Next.js UI (port `3000`)
- [`../docs/data-model.md`](../docs/data-model.md) — document schema

## Project layout

```
recipe-api/
├── main.go         # HTTP server, routes, handlers, validation
├── go.mod
├── go.sum
├── .env.example
└── README.md
```

## Troubleshooting

| Issue | Fix |
| ----- | --- |
| `bind: address already in use` | Another `go run` is still running. `lsof -i :3001`, then `kill <PID>` |
| `MONGODB_URI is not set` | `set -a && source .env && set +a` before `go run .` |
| `connect: connection refused` on `:27017` | MongoDB not running. `brew services start mongodb-community@7.0` |
| Empty list but Compass shows data | Wrong database name. Check `MONGODB_URI` ends with `/recipe_box` |
# recipe-api
