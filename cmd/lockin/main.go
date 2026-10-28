// Command lockin scans a repository for cloud services that are hard to move off AWS, Google Cloud, Azure and
// DigitalOcean, and reports an EU-portability score with an EU replacement for each service it finds.
//
//	lockin [flags] [path]
//
// Exit codes: 0 done, 1 score below --fail-under, 2 usage or I/O error.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/moveinfra/cloud-lock-in-scanner/internal/report"
	"github.com/moveinfra/cloud-lock-in-scanner/internal/rules"
	"github.com/moveinfra/cloud-lock-in-scanner/internal/scan"
)

var version = "dev" // set at build time with -ldflags "-X main.version=..."

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lockin", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format: text, json or markdown")
	out := fs.String("out", "", "write the report to this file instead of standard output")
	jsonOut := fs.String("json", "", "also write the JSON report to this file")
	mdOut := fs.String("markdown", "", "also write the Markdown report to this file")
	badge := fs.String("badge", "", "write a shields.io endpoint badge JSON to this file")
	failUnder := fs.Int("fail-under", -1, "exit with code 1 when the score is below this number (0-100)")
	listRules := fs.Bool("rules", false, "list the built-in rules and exit")
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: lockin [flags] [path]\n\n")
		fmt.Fprintf(stderr, "Scans path (default .) for cloud services and reports how portable they are to EU clouds.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(permute(fs, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, "lockin", version)
		return 0
	}
	set := rules.Default()
	if *listRules {
		printRules(stdout, set)
		return 0
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "lockin: only one path can be scanned at a time")
		return 2
	}
	if *failUnder > 100 {
		fmt.Fprintln(stderr, "lockin: --fail-under must be between 0 and 100")
		return 2
	}
	root := "."
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}

	res, err := scan.Path(root)
	if err != nil {
		fmt.Fprintln(stderr, "lockin:", err)
		return 2
	}
	rep := report.Build(res, set, version)

	render := map[string]func(io.Writer, *report.Report) error{
		"text": report.Text, "json": report.JSON, "markdown": report.Markdown, "md": report.Markdown,
	}
	primary, ok := render[*format]
	if !ok {
		fmt.Fprintf(stderr, "lockin: unknown --format %q (text, json, markdown)\n", *format)
		return 2
	}
	writes := []struct {
		path string
		fn   func(io.Writer, *report.Report) error
	}{
		{*jsonOut, report.JSON}, {*mdOut, report.Markdown}, {*badge, report.WriteBadge},
	}
	for _, w := range writes {
		if w.path == "" {
			continue
		}
		if err := writeFile(w.path, rep, w.fn); err != nil {
			fmt.Fprintln(stderr, "lockin:", err)
			return 2
		}
	}
	if *out != "" {
		err = writeFile(*out, rep, primary)
	} else {
		err = primary(stdout, rep)
	}
	if err != nil {
		fmt.Fprintln(stderr, "lockin:", err)
		return 2
	}

	if *failUnder >= 0 && rep.Score != nil && *rep.Score < *failUnder {
		fmt.Fprintf(stderr, "lockin: score %d%% is below --fail-under %d\n", *rep.Score, *failUnder)
		return 1
	}
	return 0
}

func writeFile(path string, r *report.Report, fn func(io.Writer, *report.Report) error) error {
	var buf bytes.Buffer
	if err := fn(&buf, r); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// permute moves flags in front of positional arguments, so `lockin . --format json` works like
// `lockin --format json .`. Arguments after "--" stay positional.
func permute(fs *flag.FlagSet, args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			pos = append(pos, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue
		}
		f := fs.Lookup(name)
		if f == nil {
			continue
		}
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(append(flags, "--"), pos...)
}

func printRules(w io.Writer, set *rules.Set) {
	counts := set.CountByCloud()
	for _, r := range set.Sorted() {
		capability := r.Capability
		if capability == "" {
			capability = "-"
		}
		fmt.Fprintf(w, "%-26s %-9s %-17s %s\n", r.ID, r.Class, capability, r.Service)
	}
	fmt.Fprintf(w, "\n%d rules: AWS %d, Google Cloud %d, Azure %d, DigitalOcean %d\n", len(set.Rules),
		counts["aws"], counts["gcp"], counts["azure"], counts["digitalocean"])
}
