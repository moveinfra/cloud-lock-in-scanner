package ignore

import (
	"strings"
	"testing"
)

func TestMatch(t *testing.T) {
	m, err := Parse(strings.NewReader(`
# comment
examples/
/infra/legacy
*.generated.tf
docs/**/*.yaml
!docs/keep/**
secret?.py
`))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		path  string
		dir   bool
		want  bool
		about string
	}{
		{"examples", true, true, "directory pattern matches a directory"},
		{"a/b/examples", true, true, "unanchored directory pattern matches at any depth"},
		{"examples", false, false, "directory pattern does not match a file"},
		{"infra/legacy", true, true, "anchored pattern"},
		{"x/infra/legacy", true, false, "anchored pattern only at the root"},
		{"infra/legacy/main.tf", false, true, "anything below an anchored match"},
		{"modules/net/vpc.generated.tf", false, true, "glob on the base name at any depth"},
		{"modules/net/vpc.tf", false, false, "no match"},
		{"docs/a/b/c.yaml", false, true, "** spans directories"},
		{"docs/c.yaml", false, true, "**/ also matches zero directories"},
		{"docs/keep/c.yaml", false, false, "negation re-includes"},
		{"secret1.py", false, true, "? is one character"},
		{"secret12.py", false, false, "? is only one character"},
	}
	for _, tt := range tests {
		if got := m.Match(tt.path, tt.dir); got != tt.want {
			t.Errorf("%s: Match(%q) = %v, want %v", tt.about, tt.path, got, tt.want)
		}
	}
}

func TestNilMatcher(t *testing.T) {
	var m *Matcher
	if m.Match("anything", false) {
		t.Error("nil matcher ignored a path")
	}
}
