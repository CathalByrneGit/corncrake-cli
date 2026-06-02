# corncrake-cli

[![CI](https://github.com/CathalByrneGit/corncrake-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/CathalByrneGit/corncrake-cli/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Terminal tool for mapping, validating, formatting, and submitting statutory survey returns.

Ships with support for the **CSO EHECS** survey. Works with any tenant registered in the SDK. A single static binary — no installation, no runtime, no dependencies.

> Independent open-source project. Not affiliated with or endorsed by the CSO.

Part of the [Corncrake toolchain](#ecosystem).

---

## Install

```bash
git clone https://github.com/CathalByrneGit/corncrake-cli.git
cd corncrake-cli
go build -ldflags="-s -w" -o corncrake-cli .
```

Cross-compile:
```bash
GOOS=windows GOARCH=amd64 go build -o corncrake-cli.exe .
GOOS=darwin  GOARCH=arm64 go build -o corncrake-cli-mac .
```

---

## Commands

```
corncrake-cli <command> [flags]

  map       Auto-map CSV columns to survey fields
  validate  Validate a CSV against schema and statutory rules
  submit    Validate and submit to the API — or produce XML
  export    Convert a mapped CSV directly to XML
  get       Fetch a previously submitted return from the API
  tenants   List registered survey schemes
```

---

## Typical workflow

### Step 1 — Map your columns

```bash
corncrake-cli map payroll_q1.csv
```

Reads your CSV headers, uses Jaro-Winkler fuzzy matching to assign source columns to target fields, saves `mapping.json`.

Fix unmapped fields:
```bash
corncrake-cli map payroll_q1.csv --set "OccupationCode=job_code"
```

Interactive TUI mapper:
```bash
corncrake-cli map payroll_q1.csv --interactive
```

---

### Step 2 — Validate

```bash
corncrake-cli validate payroll_q1.csv \
  --mapping mapping.json \
  --holding CSO123456 --quarter 1 --year 2026
```

Exit code `0` = valid, `1` = errors. Suitable for CI pipelines.

Interactive report with statutory rule explanations:
```bash
corncrake-cli validate payroll_q1.csv --mapping mapping.json \
  --holding CSO123456 --quarter 1 --year 2026 --interactive
```

---

### Step 3 — Submit or export

**Submit to the API:**
```bash
corncrake-cli submit payroll_q1.csv \
  --mapping mapping.json \
  --holding CSO123456 --quarter 1 --year 2026 \
  --token $CORNCRAKE_TOKEN
```

**Produce XML instead** (no token required, backwards compatible with lodgedata.cso.ie):
```bash
corncrake-cli submit payroll_q1.csv \
  --mapping mapping.json \
  --holding CSO123456 --quarter 1 --year 2026 \
  --xml
```

**XML to stdout:**
```bash
corncrake-cli submit payroll_q1.csv --mapping mapping.json \
  --holding CSO123456 --quarter 1 --year 2026 \
  --xml --xml-out -
```

**Interactive TUI confirmation:**
```bash
corncrake-cli submit payroll_q1.csv --mapping mapping.json \
  --holding CSO123456 --quarter 1 --year 2026 \
  --token $CORNCRAKE_TOKEN --interactive
```

**Dry run — validate only:**
```bash
corncrake-cli submit payroll_q1.csv --mapping mapping.json \
  --holding CSO123456 --quarter 1 --year 2026 --dry-run
```

---

### Fetch a previous submission

**Human-readable summary (default):**
```bash
corncrake-cli get \
  --submission 550e8400-e29b-41d4-a716-446655440000 \
  --holding CSO123456 --year 2026 --quarter 1 \
  --run RUN-2026-Q1-abc12345 \
  --token $CORNCRAKE_TOKEN
```

**Also export as XML:**
```bash
corncrake-cli get --submission <id> ... --token $CORNCRAKE_TOKEN --xml
```

**Interactive three-tab viewer (summary / employees / XML):**
```bash
corncrake-cli get --submission <id> ... --token $CORNCRAKE_TOKEN --interactive
```

---

## Command reference

### `submit`

| Flag | Default | Description |
|------|---------|-------------|
| `--mapping` | `mapping.json` | Mapping file |
| `--holding` | | **(required)** |
| `--quarter` | | **(required)** |
| `--year` | | **(required)** |
| `--return-type` | `ORIGINAL` | `ORIGINAL` or `AMENDED` |
| `--token` | `$CORNCRAKE_TOKEN` | Not required with `--xml` |
| `--xml` | | Produce XML instead of submitting |
| `--xml-out` | `EHECS_Q<n>_<year>.xml` | XML path (`-` for stdout) |
| `--dry-run` | | Validate only, no output |
| `--interactive` | | TUI confirmation screen |
| `--api-url` | production | Override for PIT environment |

### `get`

| Flag | Default | Description |
|------|---------|-------------|
| `--submission` | | UUID **(required)** |
| `--holding` | | **(required)** |
| `--year` | | **(required)** |
| `--quarter` | | **(required)** |
| `--run` | | Run reference **(required)** |
| `--token` | `$CORNCRAKE_TOKEN` | **(required)** |
| `--xml` | | Also export as XML |
| `--xml-out` | `<id>.xml` | XML path (`-` for stdout) |
| `--interactive` | | TUI three-tab viewer |

---

## Interactive TUI screens

All activated with `--interactive`. Non-interactive mode is unchanged for CI/scripting.

**Mapper** — full-screen table, colour-coded match scores, ranked dropdown per field. `↑↓` navigate · `enter` edit · `d` clear · `s` save.

**Validator** — scrollable PASS/WARN/FAIL list. `enter` expands a detail panel with the statutory rule in plain English and common root causes. `e`/`w` toggle filters.

**Submit** — confirmation screen with submission summary before the POST fires. Spinner while in flight, submission ID on success.

**Get** — three tabs: Summary (metadata KV), Employees (tabular with totals), XML (syntax coloured). `1`/`2`/`3` or `Tab` to switch · `↑↓`/`PgUp`/`PgDn` to scroll.

---

## Authentication

The `submit` and `get` commands require a Bearer JWT tied to your organisation's holding number.

```bash
export CORNCRAKE_TOKEN="eyJhbGci..."
corncrake-cli submit payroll.csv --mapping mapping.json \
  --holding CSO123456 --quarter 1 --year 2026
```

Token is **not required** for `--xml`, `--dry-run`, `map`, `validate`, or `export`.

Contact **ehecs@cso.ie** to request a token.

---

## API environments

| Environment | URL |
|-------------|-----|
| Production | `https://api.cso.ie/corncrake/v1` |
| PIT (testing) | `https://api-pit.cso.ie/corncrake/v1` |

---

## Project structure

```
corncrake-cli/
├── main.go
├── cmd/
│   ├── map.go       validate.go       submit.go
│   ├── export.go    get.go            tenants.go
│   └── helpers.go
└── tui/
    ├── styles.go    mapper.go         validator.go
    ├── submit.go    get.go            tui_test.go
```

---

## Testing

```bash
go test ./...        # 29 tests
go test -race ./...
```

---

## Ecosystem

| Repo | Description |
|------|-------------|
| [`corncrake`](https://github.com/CathalByrneGit/corncrake) | REST API server |
| [`corncrake-sdk`](https://github.com/CathalByrneGit/corncrake-sdk) | Go library |
| [`corncrake-cli`](https://github.com/CathalByrneGit/corncrake-cli) | This tool |

---

## Licence

MIT — Independent open-source project, not affiliated with the CSO.
