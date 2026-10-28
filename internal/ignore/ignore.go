// Package ignore reads .lockinignore files: one gitignore-style pattern per line.
//
// Supported: blank lines and '#' comments, '*' (anything but '/'), '?' (one character), '**' (any number of
// directories), a leading '/' to anchor the pattern to the scan root, a trailing '/' to match directories only,
// and '!' to re-include a path that an earlier pattern excluded. The last matching pattern wins. A pattern without
// a '/' (other than a trailing one) matches a name at any depth. A matched directory excludes everything below it.
package ignore

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

// Matcher decides whether a path relative to the scan root is ignored.
type Matcher struct {
	rules []rule
}

type rule struct {
	re      *regexp.Regexp
	negate  bool
	dirOnly bool
}

// Parse reads patterns from r.
func Parse(r io.Reader) (*Matcher, error) {
	m := &Matcher{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var ru rule
		if strings.HasPrefix(line, "!") {
			ru.negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			ru.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		anchored := strings.Contains(line, "/")
		line = strings.TrimPrefix(line, "/")
		if line == "" {
			continue
		}
		expr := globToRegexp(line)
		if anchored {
			expr = "^" + expr + "$"
		} else {
			expr = "^(?:.*/)?" + expr + "$"
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, err
		}
		ru.re = re
		m.rules = append(m.rules, ru)
	}
	return m, sc.Err()
}

// Match reports whether rel (slash-separated, relative to the scan root) is ignored. isDir says whether rel is a
// directory. A path below an ignored directory is ignored too, and cannot be re-included (as in git).
func (m *Matcher) Match(rel string, isDir bool) bool {
	if m == nil {
		return false
	}
	for i := 0; i < len(rel); i++ {
		if rel[i] == '/' && m.match(rel[:i], true) {
			return true
		}
	}
	return m.match(rel, isDir)
}

func (m *Matcher) match(rel string, isDir bool) bool {
	ignored := false
	for _, r := range m.rules {
		if r.dirOnly && !isDir {
			continue
		}
		if r.re.MatchString(rel) {
			ignored = !r.negate
		}
	}
	return ignored
}

func globToRegexp(g string) string {
	var b strings.Builder
	for i := 0; i < len(g); i++ {
		c := g[i]
		switch {
		case c == '*' && i+1 < len(g) && g[i+1] == '*':
			i++
			if i+1 < len(g) && g[i+1] == '/' {
				i++
				b.WriteString("(?:.*/)?") // "**/" : zero or more directories
			} else {
				b.WriteString(".*")
			}
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return b.String()
}
