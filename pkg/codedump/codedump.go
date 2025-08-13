package codedump

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/build"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// DefaultRCName is the default name for the RC/config file.
const DefaultRCName = ".codedumprc"

// Config holds the parameters for a dump run.
type Config struct {
	Root    string // where the final TXT will be saved
	Target  string // folder to scan
	Out     string // output file name (relative to Root)
	Ext     string // file extension to include
	Include string // optional substring filter (path/content)
	Exclude string // comma-separated substrings to skip (path)
	Pkg     bool   // keep "package" line if true
}

// DefaultConfig returns sane defaults for the tool.
func DefaultConfig() Config {
	return Config{
		Root:    ".",
		Target:  "./models",
		Out:     "models_tree.txt",
		Ext:     ".go",
		Exclude: "_test.go,/.git/,/vendor/",
		Pkg:     false,
	}
}

// Item represents one collected file.
type Item struct {
	rel  string
	abs  string
	sha  string
	size int64
}

// Dump generates the concatenated output and writes it to the configured Out path.
// It returns the absolute output path and the number of files written.
func Dump(c Config) (string, int, error) {
	wd, _ := os.Getwd()
	rootAbs := AbsFrom(wd, c.Root)
	targetAbs := AbsFrom(wd, c.Target)
	outAbs := AbsFrom(rootAbs, c.Out)

	items, err := Collect(targetAbs, c)
	if err != nil { return "", 0, err }

	var buf bytes.Buffer
	now := time.Now().Format(time.RFC3339)
	fmt.Fprintf(&buf, "// ===== CODEDUMP GENERATED =====\n")
	fmt.Fprintf(&buf, "// #pwd: %s\n", wd)
	fmt.Fprintf(&buf, "// #generated_at: %s\n", now)
	fmt.Fprintf(&buf, "// #go_version: %s\n", runtime.Version())
	fmt.Fprintf(&buf, "// #goroot: %s\n", build.Default.GOROOT)
	fmt.Fprintf(&buf, "// #root: %s\n", filepath.ToSlash(rootAbs))
	fmt.Fprintf(&buf, "// #target: %s\n", filepath.ToSlash(targetAbs))
	fmt.Fprintf(&buf, "// #out: %s\n", filepath.ToSlash(outAbs))
	fmt.Fprintf(&buf, "// =================================\n\n")

	for _, it := range items {
		data, err := os.ReadFile(it.abs)
		if err != nil { return "", 0, err }
		content := data
		if !c.Pkg {
			content = StripPackageLine(data)
		}
		fmt.Fprintf(&buf, "// ===== BEGIN FILE =====\n")
		fmt.Fprintf(&buf, "// #rel_path: %s\n", it.rel)
		fmt.Fprintf(&buf, "// #abs_path: %s\n", filepath.ToSlash(it.abs))
		fmt.Fprintf(&buf, "// #size_bytes: %d\n", it.size)
		fmt.Fprintf(&buf, "// #sha256: %s\n", it.sha)
		fmt.Fprintf(&buf, "// ======================\n")
		io.Copy(&buf, bytes.NewReader(content))
		if len(content) > 0 && content[len(content)-1] != '\n' {
			buf.WriteByte('\n')
		}
		fmt.Fprintln(&buf, "// ===== END FILE =====\n")
	}

	if err := os.MkdirAll(filepath.Dir(outAbs), 0o755); err != nil { return "", 0, err }
	if err := os.WriteFile(outAbs, buf.Bytes(), 0o644); err != nil { return "", 0, err }
	return outAbs, len(items), nil
}

// Collect walks the target directory, applying filters, and returns metadata for each file.
func Collect(targetAbs string, c Config) ([]Item, error) {
	excl := SplitClean(c.Exclude)
	wd, _ := os.Getwd()
	var out []Item

	err := filepath.WalkDir(targetAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil { return err }
		if d.IsDir() {
			pp := filepath.ToSlash(path)
			for _, bad := range excl {
				if bad != "" && strings.Contains(pp, bad) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, c.Ext) { return nil }
		if filepath.Base(path) == c.Out { return nil }

		pp := filepath.ToSlash(path)
		if c.Include != "" && !strings.Contains(pp, c.Include) { return nil }
		for _, bad := range excl {
			if bad != "" && strings.Contains(pp, bad) { return nil }
		}

		st, err := os.Stat(path)
		if err != nil { return err }
		data, err := os.ReadFile(path)
		if err != nil { return err }
		sum := sha256.Sum256(data)
		rel, _ := filepath.Rel(wd, path)
		out = append(out, Item{
			rel:  filepath.ToSlash(rel),
			abs:  path,
			sha:  hex.EncodeToString(sum[:]),
			size: st.Size(),
		})
		return nil
	})
	if err != nil { return nil, err }

	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out, nil
}

// SplitClean splits a comma-separated list and trims/normalizes separators.
func SplitClean(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p != "" { out = append(out, p) }
	}
	return out
}

// StripPackageLine removes the first "package" line from a Go source file.
func StripPackageLine(src []byte) []byte {
	lines := bytes.Split(src, []byte("\n"))
	out := make([][]byte, 0, len(lines))
	skipped := false
	for _, ln := range lines {
		trim := bytes.TrimSpace(ln)
		if !skipped && bytes.HasPrefix(trim, []byte("package ")) {
			skipped = true
			continue
		}
		out = append(out, ln)
	}
	return bytes.Join(out, []byte("\n"))
}

// WriteDefaultRC writes a new RC file with defaults to the given path.
func WriteDefaultRC(path string) error {
	content := `# .codedumprc
# Root of the project (where the final TXT will be saved)
root=.

# Target folder to scan
target=./models

# Output file name (relative to root)
out=models_tree.txt

# File extension to include
ext=.go

# Substrings to exclude (comma separated)
exclude=_test.go,/.git/,/vendor/

# Required substring (optional)
include=

# Keep "package" line (true/false)
pkg=false
`
	return os.WriteFile(path, []byte(content), 0o644)
}

// ReadRC populates the given Config from a RC file.
func ReadRC(path string, c *Config) error {
	b, err := os.ReadFile(path)
	if err != nil { return err }
	lines := strings.Split(string(b), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") { continue }
		kv := strings.SplitN(ln, "=", 2)
		if len(kv) != 2 { continue }
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		switch strings.ToLower(k) {
		case "root": c.Root = v
		case "target": c.Target = v
		case "out": c.Out = v
		case "ext": c.Ext = v
		case "exclude": c.Exclude = v
		case "include": c.Include = v
		case "pkg": c.Pkg = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
		}
	}
	return nil
}

// FindRC searches for a .codedumprc starting from the CWD up to root, then $HOME.
func FindRC() string {
	wd, _ := os.Getwd()
	cur := wd
	for {
		rc := filepath.Join(cur, DefaultRCName)
		if _, err := os.Stat(rc); err == nil { return rc }
		parent := filepath.Dir(cur)
		if parent == cur { break }
		cur = parent
	}
	if home, err := os.UserHomeDir(); err == nil {
		rc := filepath.Join(home, DefaultRCName)
		if _, err := os.Stat(rc); err == nil { return rc }
	}
	return ""
}

// AbsFrom resolves a possibly-relative path against a base directory.
func AbsFrom(base, p string) string {
	if filepath.IsAbs(p) { return p }
	ap, _ := filepath.Abs(filepath.Join(base, p))
	return ap
}


