// Compare-snapshots CLI ported from tests/compare-snapshots.ts.
//
// Usage:
//
//	go run ./test/verify/compare.go snapshot <dir> --out <manifest.json>
//	go run ./test/verify/compare.go compare <manifest.json> <dir> [--tolerance <regex>] [--max-delta 2]
//	go run ./test/verify/compare.go diff <dirA> <dirB> [--tolerance <regex>] [--max-delta 2]
package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/pqhuy98/wow-converter/test/internal/snapshot"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]
	toleranceRegex, maxDelta := parseFlags(args)

	switch cmd {
	case "snapshot":
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "snapshot requires <dir>")
			os.Exit(1)
		}
		dir := args[0]
		out := flagValue(args, "--out")
		if out == "" {
			out = "snapshot.json"
		}
		manifest, err := snapshot.Create(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := snapshot.WriteManifest(out, manifest); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Snapshot of %d files written to %s\n", len(manifest.Files), out)

	case "compare":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "compare requires <manifest.json> <dir>")
			os.Exit(1)
		}
		manifest, err := snapshot.LoadManifest(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		summary, err := snapshot.CompareManifestToDir(manifest, args[1], snapshot.CompareOptions{
			ToleranceRegex: toleranceRegex,
			MaxDelta:       maxDelta,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if !snapshot.PrintSummary(os.Stdout, summary) {
			os.Exit(1)
		}

	case "diff":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "diff requires <dirA> <dirB>")
			os.Exit(1)
		}
		manifest, err := snapshot.Create(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		summary, err := snapshot.CompareManifestToDir(manifest, args[1], snapshot.CompareOptions{
			ToleranceRegex: toleranceRegex,
			MaxDelta:       maxDelta,
			BaselineDir:    args[0],
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if !snapshot.PrintSummary(os.Stdout, summary) {
			os.Exit(1)
		}

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage:
  go run ./test/verify/compare.go snapshot <dir> --out <manifest.json>
  go run ./test/verify/compare.go compare <manifest.json> <dir> [--tolerance <regex>] [--max-delta 2]
  go run ./test/verify/compare.go diff <dirA> <dirB> [--tolerance <regex>] [--max-delta 2]`)
}

func parseFlags(args []string) (*regexp.Regexp, int) {
	toleranceArg := flagValue(args, "--tolerance")
	var toleranceRegex *regexp.Regexp
	if toleranceArg != "" {
		toleranceRegex = regexp.MustCompile("(?i)" + toleranceArg)
	}
	maxDelta := 2
	if v := flagValue(args, "--max-delta"); v != "" {
		fmt.Sscanf(v, "%d", &maxDelta)
	}
	return toleranceRegex, maxDelta
}

func flagValue(args []string, name string) string {
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
