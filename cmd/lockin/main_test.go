package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const testdata = "../../testdata"

// testdataScore is the score of the fixtures. Update it, and the example in README.md, when rules or fixtures change.
const testdataScore = 56

func TestRunText(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{testdata}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "EU-portable: 56%") {
		t.Errorf("unexpected report:\n%s", out.String())
	}
}

func TestRunOutputs(t *testing.T) {
	dir := t.TempDir()
	report, md, badge := filepath.Join(dir, "r.json"), filepath.Join(dir, "r.md"), filepath.Join(dir, "badge.json")
	var out, errOut bytes.Buffer
	// Flags after the path work too.
	code := run([]string{testdata, "--format", "json", "--json", report, "--markdown", md, "--badge", badge}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var r struct {
		Score    *int `json:"score"`
		Findings []struct {
			Rule      string `json:"rule"`
			Locations []any  `json:"locations"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	if r.Score == nil || *r.Score != testdataScore {
		t.Errorf("score = %v, want %d", r.Score, testdataScore)
	}
	for _, f := range r.Findings {
		if f.Rule == "aws.dynamodb" && len(f.Locations) <= 5 {
			t.Errorf("JSON should list every location, got %d for DynamoDB", len(f.Locations))
		}
	}
	file, _ := os.ReadFile(report)
	if !bytes.Equal(file, out.Bytes()) {
		t.Error("--json file differs from --format json output")
	}
	if b, _ := os.ReadFile(md); !strings.HasPrefix(string(b), "## EU-portable: 56%") {
		t.Errorf("markdown file: %s", b)
	}
	if b, _ := os.ReadFile(badge); !strings.Contains(string(b), `"message":"56%"`) {
		t.Errorf("badge file: %s", b)
	}
}

func TestFailUnder(t *testing.T) {
	tests := []struct {
		args []string
		want int
	}{
		{[]string{"--fail-under", "57", testdata}, 1},
		{[]string{"--fail-under", "56", testdata}, 0},
		{[]string{"--fail-under=0", testdata}, 0},
		{[]string{"--fail-under", "101", testdata}, 2},
		{[]string{"--fail-under", "90", t.TempDir()}, 0}, // nothing found: no score, no failure
		{[]string{"--format", "xml", testdata}, 2},
		{[]string{testdata, testdata}, 2},
		{[]string{"does-not-exist"}, 2},
	}
	for _, tt := range tests {
		var out, errOut bytes.Buffer
		if got := run(tt.args, &out, &errOut); got != tt.want {
			t.Errorf("run(%q) = %d, want %d (%s)", tt.args, got, tt.want, errOut.String())
		}
	}
}

func TestRules(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"--rules"}, &out, &out); code != 0 {
		t.Fatal(out.String())
	}
	if !strings.Contains(out.String(), "aws.dynamodb") || !strings.Contains(out.String(), " rules: AWS ") {
		t.Errorf("rules list:\n%s", out.String())
	}
}

func TestPermute(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.String("format", "", "")
	fs.Bool("rules", false, "")
	got := permute(fs, []string{"dir", "--format", "json", "--rules", "--x=1", "--", "-odd"})
	want := []string{"--format", "json", "--rules", "--x=1", "--", "dir", "-odd"}
	if !slices.Equal(got, want) {
		t.Errorf("permute = %q, want %q", got, want)
	}
}
