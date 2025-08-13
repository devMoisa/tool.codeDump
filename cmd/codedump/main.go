package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/devMoisa/tool.codeDump/pkg/codedump"
)

func main() {
	var (
		flInit                      bool
		flRoot, flTarget, flOut     string
		flExt, flInclude, flExclude string
		flPkg                       bool
		flRCPath                    string
	)

	flag.BoolVar(&flInit, "init", false, fmt.Sprintf("Create a %s in the current directory", codedump.DefaultRCName))
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
		if err := codedump.WriteDefaultRC(codedump.DefaultRCName); err != nil {
			fatal(err)
		}
		fmt.Printf("Created %s with defaults. Adjust root/target/out according to your project.\n", codedump.DefaultRCName)
		return
	}

	c := codedump.DefaultConfig()
	rcPath := flRCPath
	if rcPath == "" {
		rcPath = codedump.FindRC()
	}
	if rcPath != "" {
		if err := codedump.ReadRC(rcPath, &c); err != nil {
			fatal(fmt.Errorf("error reading RC %s: %w", rcPath, err))
		}
	}

	if flRoot != "" { c.Root = flRoot }
	if flTarget != "" { c.Target = flTarget }
	if flOut != "" { c.Out = flOut }
	if flExt != "" { c.Ext = flExt }
	if flInclude != "" { c.Include = flInclude }
	if flExclude != "" { c.Exclude = flExclude }
	if flPkg { c.Pkg = true }

	outAbs, n, err := codedump.Dump(c)
	if err != nil { fatal(err) }
	fmt.Printf("✅ codeDump complete! Generated %q with %d files.\n", outAbs, n)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "❌ error: %v\n", err)
	os.Exit(1)
}


