package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
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

const defaultRCName = ".codedumprc"

type cfg struct {
	Root    string // where the final TXT will be saved
	Target  string // folder to scan
	Out     string // output file name (relative to Root)
	Ext     string // file extension to include
	Include string // optional substring filter
	Exclude string // comma-separated substrings to skip
	Pkg     bool   // keep "package" line if true
}

func defaultCfg() cfg {
	return cfg{
		Root:    ".",
		Target:  "./models",
		Out:     "models_tree.txt",
		Ext:     ".go",
		Exclude: "_test.go,/.git/,/vendor/",
		Pkg:     false,
	}
}

func main() {
	var (
		flInit                    bool
		flRoot, flTarget, flOut   string
		flExt, flInclude, flExclude string
		flPkg                     bool
		flRCPath                  string
	)
	flag.BoolVar(&flInit, "init", false, fmt.Sprintf("Create a %s in the current directory", defaultRCName))
	flag.StringVar(&flRCPath, "rc", "", "Path to RC file (optional). If empty, will search locally and in $HOME")
	flag.StringVar(&flRoot, "root", "", "Root dir (overrides RC)")
	flag.StringVar(&flTarget, "target", "", "Target dir to scan (overrides RC)")
	flag.StringVar(&flOut, "out", "", "Output file name (overrides RC)")
	flag.StringVar(&flExt, "ext", "", "Target file extension (overrides RC)")
	flag.StringVar(&flInclude, "include", "", "Required substring in path (overrides RC)")
	flag.StringVar(&flExclude, "exclude", "", "Comma-separated substrings to skip (overrides RC)")
	flag.BoolVar(&flPkg, "pkg", false, "Preserve package line (overrides RC -> true)")
	flag.Parse()

	if flInit {
		if err := writeDefaultRC(defaultRCName); err != nil {
			fatal(err)
		}
		fmt.Printf("Created %s with defaults. Adjust root/target/out according to your project.\n", defaultRCName)
		return
	}

	c := defaultCfg()
	rcPath := flRCPath
	if rcPath == "" {
		rcPath = findRC()
	}
	if rcPath != "" {
		if err := readRC(rcPath, &c); err != nil {
			fatal(fmt.Errorf("error reading RC %s: %w", rcPath, err))
		}
	}

	// Apply CLI overrides
	if flRoot != "" { c.Root = flRoot }
	if flTarget != "" { c.Target = flTarget }
	if flOut != "" { c.Out = flOut }
	if flExt != "" { c.Ext = flExt }
	if flInclude != "" { c.Include = flInclude }
	if flExclude != "" { c.Exclude = flExclude }
	if flPkg { c.Pkg = true }

	// Normalize paths
	wd, _ := os.Getwd()
	rootAbs := absFrom(wd, c.Root)
	targetAbs := absFrom(wd, c.Target)
	outAbs := absFrom(rootAbs, c.Out)

	// Collect files
	items, err := collect(targetAbs, c)
	if err != nil { fatal(err) }

	// Build output
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
		if err != nil { fatal(err) }
		content := data
		if !c.Pkg {
			content = stripPackageLine(data)
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

	if err := os.MkdirAll(filepath.Dir(outAbs), 0o755); err != nil { fatal(err) }
	if err := os.WriteFile(outAbs, buf.Bytes(), 0o644); err != nil { fatal(err) }

	fmt.Printf("✅ codeDump complete! Generated %q with %d files.\n", outAbs, len(items))
}

type item struct {
	rel  string
	abs  string
	sha  string
	size int64
}

func collect(targetAbs string, c cfg) ([]item, error) {
	excl := splitClean(c.Exclude)
	wd, _ := os.Getwd()
	var out []item

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
			if bad != "" && strings.Contains(pp, bad) {
				return nil
			}
		}

		st, err := os.Stat(path)
		if err != nil { return err }
		data, err := os.ReadFile(path)
		if err != nil { return err }
		sum := sha256.Sum256(data)
		rel, _ := filepath.Rel(wd, path)
		out = append(out, item{
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

func splitClean(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p != "" { out = append(out, p) }
	}
	return out
}

func stripPackageLine(src []byte) []byte {
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

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "❌ error: %v\n", err)
	os.Exit(1)
}

// ---------- RC helpers ----------

func writeDefaultRC(path string) error {
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

func readRC(path string, c *cfg) error {
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

func findRC() string {
	// Look for .codedumprc from cwd upwards
	wd, _ := os.Getwd()
	cur := wd
	for {
		rc := filepath.Join(cur, defaultRCName)
		if _, err := os.Stat(rc); err == nil {
			return rc
		}
		parent := filepath.Dir(cur)
		if parent == cur { break }
		cur = parent
	}
	// Fallback to $HOME/.codedumprc
	if home, err := os.UserHomeDir(); err == nil {
		rc := filepath.Join(home, defaultRCName)
		if _, err := os.Stat(rc); err == nil {
			return rc
		}
	}
	return ""
}

func absFrom(base, p string) string {
	if filepath.IsAbs(p) { return p }
	ap, _ := filepath.Abs(filepath.Join(base, p))
	return ap
}