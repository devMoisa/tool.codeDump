# tool.codeDump

A tiny, fast CLI to turn any folder of code into one readable, annotated text file.

Perfect for sharing context with LLMs, creating lightweight project snapshots, or exporting selective folders with rich metadata.

Fully configurable via a `.codedumprc` file in your project or globally in `$HOME`, with CLI flags that always take precedence.

---

## Demo

Add a short GIF showing init + run + output preview.

![Demo — tool.codeDump in action](docs/usage.gif)

Tip: drop your GIF at `docs/usage.gif` or update the path above.

---

## Features

- **Recursive scan**: Walk any target folder filtering by file extension
- **Single output**: Concatenate all matched files into one file
- **Rich metadata**: Per-file header with relative/absolute path, size, SHA256
- **Smart filtering**: Include substring, exclude multiple patterns
- **Go-friendly**: Optionally keep or strip the `package` line
- **Config-first**: `.codedumprc` for defaults; flags to override

---

## Installation

```bash
git clone https://github.com/devMoisa/tool.codeDump
cd tool.codeDump
go build -o codedump ./cmd/codedump
```

You now have a local `./codedump` binary. You can also install it globally with:

```bash
go install ./cmd/codedump
```

---

## Quick Start

1. Initialize config in the current directory

```bash
./codedump --init
```

This creates `.codedumprc`:

```ini
# .codedumprc
root=.
target=./models
out=models_tree.txt
ext=.go
exclude=_test.go,/.git/,/vendor/
include=
pkg=false
```

2. Adjust values as needed, then run:

```bash
./codedump
```

Your concatenated file is created at `./models_tree.txt` (relative to `root`).

---

## Configuration

Supported keys in `.codedumprc`:

- **root**: Base directory for resolving paths and writing `out`.
- **target**: Directory to recursively scan for files.
- **out**: Output file path (relative to `root`).
- **ext**: File extension filter (example: `.go`).
- **include**: Only include files whose content contains this substring (optional).
- **exclude**: Comma-separated substrings; any matching path is skipped.
- **pkg**: When `true`, keeps `package` lines in Go files.

CLI flags mirror these keys and override them when provided.

---

## CLI Flags

| Flag        | Description                                  |
| ----------- | -------------------------------------------- |
| `--init`    | Create a `.codedumprc` in the current folder |
| `--rc`      | Path to a custom RC file                     |
| `--root`    | Override root directory                      |
| `--target`  | Override target folder                       |
| `--out`     | Override output file name                    |
| `--ext`     | Override file extension filter               |
| `--include` | Only include files containing this substring |
| `--exclude` | Comma-separated substrings to skip           |
| `--pkg`     | Preserve `package` line                      |

---

## Examples

```bash
# Dump ./models into models_tree.txt
./codedump

# Dump ./internal/models to all_models.txt keeping package lines
./codedump --target ./internal/models --out all_models.txt --pkg

# Only include files that contain the word "DTO" and skip vendor
./codedump --include DTO --exclude /vendor/

# Use a custom RC path
./codedump --rc /path/to/.codedumprc
```

---

## Library usage

You can embed tool.codeDump in your own Go programs:

```go
package main

import (
    "fmt"
    "github.com/devMoisa/tool.codeDump/pkg/codedump"
)

func main() {
    cfg := codedump.DefaultConfig()
    cfg.Target = "./models"
    cfg.Out = "models_tree.txt"
    out, n, err := codedump.Dump(cfg)
    if err != nil { panic(err) }
    fmt.Printf("wrote %s with %d files\n", out, n)
}
```

---

## Output Format (sample)

Each file is preceded by a header describing the environment, target, and file details:

```1:27:codeDump_example.txt
// ===== CODEDUMP GENERATED =====
// #pwd: /Users/yourname/Documents/www/repo/tool.codeDump
// #generated_at: 2025-08-11T17:32:33-03:00
// #go_version: go1.24.3
// #goroot: /opt/homebrew/Cellar/go/1.24.3/libexec
// #root: /Users/yourname/Documents/www/repo/tool.codeDump
// #target: /Users/yourname/Documents/www/repo/tool.codeDump/models
// #out: /Users/yourname/Documents/www/repo/tool.codeDump/models_tree.txt
// =================================

// ===== BEGIN FILE =====
// #rel_path: models/account_payable.go
// #abs_path: /Users/yourname/Documents/www/repo/tool.codeDump/models/account_payable.go
// #size_bytes: 1000
// #sha256: 428d5ebb12fd9bc9946d6706b964f2511197af1f024b19d581a2b08c0e7448af

... (content omitted for brevity) ...
```

---

## How it works

- Scans `target` recursively collecting files ending with `ext`.
- Applies filters: `exclude` by path segments, `include` by substring search in content.
- Concatenates files to `out`, prefixing each with a structured, human-readable header.
- Optionally removes Go `package` lines unless `--pkg` is set or `pkg=true`.

---

## Troubleshooting

- Make sure you run `codedump` from a directory where `.codedumprc` is visible, or pass `--rc`.
- If nothing is found, verify `ext` and `target` values and that files actually match.
- For large folders, prefer tighter `exclude` filters to speed up scanning.

---

## License

MIT — do whatever helps your workflow. Contributions welcome.

# tool.codeDump
