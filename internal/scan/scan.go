// Package scan walks a directory and reports every place where a cloud service is referenced: Terraform types,
// CloudFormation / SAM types, Serverless Framework functions and events, CDK constructs and SDK imports. It only
// reads text; it never calls a cloud API.
package scan

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moveinfra/cloud-lock-in-scanner/internal/ignore"
)

// MaxFileSize is the largest file that is read. Bigger files are counted as skipped.
const MaxFileSize = 1 << 20

// IgnoreFile is read from the scan root when present.
const IgnoreFile = ".lockinignore"

// skipDirs are never entered, at any depth.
var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"vendor": true, "node_modules": true, "dist": true, "build": true,
	".terraform": true, ".terragrunt-cache": true,
	"cdk.out": true, ".serverless": true, ".aws-sam": true, // generated templates, would double count
	".venv": true, "venv": true, "__pycache__": true,
}

// Hit is one reference to a cloud service.
type Hit struct {
	Cloud  string // aws, gcp, azure, digitalocean
	Kind   string // rules.KindTerraform, rules.KindCloudFormation or rules.KindSDK
	Name   string // the name rules are matched on: a Terraform or CloudFormation type, or an SDK service name
	Source string // what was recognised, e.g. "terraform resource", "boto3", "@aws-sdk", "serverless event"
	Text   string // the text as written, e.g. aws_dynamodb_table or @aws-sdk/client-sqs
	Body   string // Terraform block body, for rules with a body condition
	File   string // slash-separated, relative to the scan root
	Line   int
}

// Result is everything one scan found.
type Result struct {
	Root         string
	FilesScanned int // files that at least one detector looked at
	FilesSkipped int // files over MaxFileSize
	Hits         []Hit
}

// Path scans a directory (or a single file).
func Path(root string) (*Result, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	res := &Result{Root: root}
	if !info.IsDir() {
		if err := res.file(root, filepath.Base(root), info.Size()); err != nil {
			return nil, err
		}
		return res, nil
	}

	var ign *ignore.Matcher
	if f, err := os.Open(filepath.Join(root, IgnoreFile)); err == nil {
		ign, err = ignore.Parse(f)
		f.Close()
		if err != nil {
			return nil, err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDirs[d.Name()] || ign.Match(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || ign.Match(rel, false) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return res.file(p, rel, info.Size())
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(res.Hits, func(i, j int) bool {
		a, b := res.Hits[i], res.Hits[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return res, nil
}

func (res *Result) file(p, rel string, size int64) error {
	if len(detectorsFor(rel)) == 0 {
		return nil
	}
	if size > MaxFileSize {
		res.FilesSkipped++
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if bytes.IndexByte(b[:min(len(b), 8000)], 0) >= 0 {
		return nil // binary
	}
	res.FilesScanned++
	res.Hits = append(res.Hits, File(rel, b)...)
	return nil
}

// File runs every detector that applies to the file name on its content. rel is used for the file type and is
// copied into the hits.
func File(rel string, content []byte) []Hit {
	src := string(content)
	lines := newLineIndex(src)
	var hits []Hit
	for _, d := range detectorsFor(rel) {
		for _, h := range d(src, lines) {
			h.File = rel
			hits = append(hits, h)
		}
	}
	return hits
}

type detector func(src string, lines lineIndex) []Hit

func detectorsFor(rel string) []detector {
	base := strings.ToLower(filepath.Base(rel))
	ext := filepath.Ext(base)
	var ds []detector
	switch ext {
	case ".tf":
		ds = append(ds, terraform)
	case ".yaml", ".yml", ".json", ".template":
		if base == "package-lock.json" || base == "pnpm-lock.yaml" || base == "composer.lock" {
			return nil
		}
		ds = append(ds, cloudFormation)
		if base == "serverless.yml" || base == "serverless.yaml" {
			ds = append(ds, serverless)
		}
		if base == "package.json" {
			ds = append(ds, javascript)
		}
	case ".py":
		ds = append(ds, python)
		if base == "setup.py" {
			ds = append(ds, pythonManifest)
		}
	case ".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx", ".mts", ".cts":
		ds = append(ds, javascript)
	case ".go":
		ds = append(ds, golang)
	case ".java", ".kt", ".kts", ".scala":
		ds = append(ds, java)
	}
	if (strings.HasPrefix(base, "requirements") && ext == ".txt") || base == "pyproject.toml" ||
		base == "pipfile" || base == "setup.cfg" {
		ds = append(ds, pythonManifest)
	}
	return ds
}

// lineIndex maps byte offsets to 1-based line numbers.
type lineIndex []int // offsets of line starts

func newLineIndex(s string) lineIndex {
	idx := lineIndex{0}
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			idx = append(idx, i+1)
		}
	}
	return idx
}

func (l lineIndex) line(off int) int {
	return sort.Search(len(l), func(i int) bool { return l[i] > off })
}
