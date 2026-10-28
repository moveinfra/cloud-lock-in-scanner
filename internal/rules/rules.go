// Package rules holds the service catalogue: which Terraform types, CloudFormation types and SDK names belong to
// which cloud service, how portable that service is, and what replaces it on an EU-owned cloud. The data lives in
// rules.json and is embedded at build time.
package rules

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

//go:embed rules.json
var data []byte

// Portability classes.
const (
	Portable = "portable" // standard technology with a direct EU equivalent
	Moderate = "moderate" // needs a swap, but standard alternatives exist
	Hard     = "hard"     // proprietary data model or runtime
)

// Kinds of names a detector can report. Each kind has its own pattern list in a rule.
const (
	KindTerraform      = "terraform"      // Terraform resource or data source type, e.g. aws_dynamodb_table
	KindCloudFormation = "cloudformation" // CloudFormation / SAM resource type, e.g. AWS::DynamoDB::Table
	KindSDK            = "sdk"            // normalised SDK or CDK service name, e.g. dynamodb
)

// Clouds the scanner knows, with their display names.
var Clouds = map[string]string{
	"aws":          "AWS",
	"gcp":          "Google Cloud",
	"azure":        "Azure",
	"digitalocean": "DigitalOcean",
}

// Capabilities are the ids used in the EU Cloud Map (https://github.com/moveinfra/eu-cloud-map,
// data/eu-cloud-map.json). A rule's capability must be one of these, or empty.
var Capabilities = []string{
	"compute", "managed-postgres", "object-storage", "load-balancer", "networking", "kubernetes",
	"redis", "queues", "functions", "dns", "nosql", "warehouse",
}

// Weight is the score contribution of each class.
var Weight = map[string]float64{Portable: 1, Moderate: 0.5, Hard: 0}

// Rule describes one cloud service.
type Rule struct {
	ID          string `json:"id"`
	Cloud       string `json:"cloud"`
	Service     string `json:"service"`
	Class       string `json:"class"`
	Capability  string `json:"capability,omitempty"`
	Replacement string `json:"replacement"`
	// Group marks glue resources (networking, IAM) that are reported as one finding with a count of types.
	Group bool `json:"group,omitempty"`

	Terraform []string `json:"terraform,omitempty"`
	// TerraformBody, when set, makes the Terraform patterns apply only to blocks whose body matches one of these
	// regular expressions. Such a rule wins over a rule without a body condition.
	TerraformBody  []string `json:"terraform_body,omitempty"`
	CloudFormation []string `json:"cloudformation,omitempty"`
	SDK            []string `json:"sdk,omitempty"`

	body []*regexp.Regexp
}

// Set is a loaded, validated rule catalogue.
type Set struct {
	Rules    []*Rule
	patterns map[string][]pattern // by kind
}

type pattern struct {
	glob     string
	literal  int // characters that are not '*'
	exact    bool
	rule     *Rule
	withBody bool
}

type file struct {
	SchemaVersion int     `json:"schema_version"`
	Rules         []*Rule `json:"rules"`
}

// Default returns the embedded catalogue. It panics if the embedded data is invalid, which the tests prevent.
func Default() *Set {
	s, err := Parse(data)
	if err != nil {
		panic(err)
	}
	return s
}

// Parse reads and validates a catalogue in the rules.json format.
func Parse(b []byte) (*Set, error) {
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("rules: %w", err)
	}
	s := &Set{Rules: f.Rules, patterns: map[string][]pattern{}}
	ids := map[string]bool{}
	caps := map[string]bool{}
	for _, c := range Capabilities {
		caps[c] = true
	}
	seen := map[string]string{} // kind/cloud/pattern/withBody -> rule id
	for _, r := range f.Rules {
		switch {
		case r.ID == "" || ids[r.ID]:
			return nil, fmt.Errorf("rules: missing or duplicate id %q", r.ID)
		case Clouds[r.Cloud] == "":
			return nil, fmt.Errorf("rules: %s: unknown cloud %q", r.ID, r.Cloud)
		case r.Class != Portable && r.Class != Moderate && r.Class != Hard:
			return nil, fmt.Errorf("rules: %s: class must be portable, moderate or hard, not %q", r.ID, r.Class)
		case r.Capability != "" && !caps[r.Capability]:
			return nil, fmt.Errorf("rules: %s: capability %q is not in the EU Cloud Map", r.ID, r.Capability)
		case r.Service == "" || r.Replacement == "":
			return nil, fmt.Errorf("rules: %s: service and replacement are required", r.ID)
		case len(r.Terraform)+len(r.CloudFormation)+len(r.SDK) == 0:
			return nil, fmt.Errorf("rules: %s: no patterns", r.ID)
		}
		ids[r.ID] = true
		for _, expr := range r.TerraformBody {
			re, err := regexp.Compile(expr)
			if err != nil {
				return nil, fmt.Errorf("rules: %s: terraform_body: %w", r.ID, err)
			}
			r.body = append(r.body, re)
		}
		add := func(kind string, globs []string, withBody bool) error {
			for _, g := range globs {
				if kind == KindSDK {
					g = NormalizeSDK(g)
				}
				if _, err := path.Match(g, ""); err != nil || g == "" || g == "*" {
					return fmt.Errorf("rules: %s: bad %s pattern %q", r.ID, kind, g)
				}
				key := fmt.Sprintf("%s/%s/%s/%v", kind, r.Cloud, g, withBody)
				if other, ok := seen[key]; ok {
					return fmt.Errorf("rules: %s: %s pattern %q is also in %s", r.ID, kind, g, other)
				}
				seen[key] = r.ID
				s.patterns[kind] = append(s.patterns[kind], pattern{
					glob:     g,
					literal:  len(strings.ReplaceAll(g, "*", "")),
					exact:    !strings.Contains(g, "*"),
					rule:     r,
					withBody: withBody,
				})
			}
			return nil
		}
		if err := add(KindTerraform, r.Terraform, len(r.body) > 0); err != nil {
			return nil, err
		}
		if err := add(KindCloudFormation, r.CloudFormation, false); err != nil {
			return nil, err
		}
		if err := add(KindSDK, r.SDK, false); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Match returns the rule for a name, or nil when no rule of that cloud matches. body is the Terraform block body
// (empty for other kinds). When several patterns match, a rule with a matching body condition wins, then an exact
// pattern, then the pattern with the most literal characters.
func (s *Set) Match(kind, cloud, name, body string) *Rule {
	if kind == KindSDK {
		name = NormalizeSDK(name)
	}
	var best *pattern
	better := func(a, b *pattern) bool { // is a better than b
		if b == nil {
			return true
		}
		if a.withBody != b.withBody {
			return a.withBody
		}
		if a.exact != b.exact {
			return a.exact
		}
		return a.literal > b.literal
	}
	for i := range s.patterns[kind] {
		p := &s.patterns[kind][i]
		if p.rule.Cloud != cloud {
			continue
		}
		if ok, _ := path.Match(p.glob, name); !ok {
			continue
		}
		if p.withBody && !p.rule.bodyMatches(body) {
			continue
		}
		if better(p, best) {
			best = p
		}
	}
	if best == nil {
		return nil
	}
	return best.rule
}

func (r *Rule) bodyMatches(body string) bool {
	for _, re := range r.body {
		if re.MatchString(body) {
			return true
		}
	}
	return false
}

// NormalizeSDK lower-cases a service name and drops everything that is not a letter or digit, so that boto3
// "cognito-idp", JavaScript "@aws-sdk/client-cognito-identity-provider" and Go "cognitoidentityprovider" all compare
// on the same alphabet. A trailing '*' (glob) is kept.
func NormalizeSDK(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '*' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// CountByCloud returns the number of rules per cloud id.
func (s *Set) CountByCloud() map[string]int {
	n := map[string]int{}
	for _, r := range s.Rules {
		n[r.Cloud]++
	}
	return n
}

// Sorted returns the rules ordered by cloud, class (hard first) and service.
func (s *Set) Sorted() []*Rule {
	out := append([]*Rule(nil), s.Rules...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Cloud != b.Cloud {
			return CloudOrder(a.Cloud) < CloudOrder(b.Cloud)
		}
		if a.Class != b.Class {
			return ClassOrder(a.Class) < ClassOrder(b.Class)
		}
		return a.Service < b.Service
	})
	return out
}

// ClassOrder sorts hard before moderate before portable.
func ClassOrder(c string) int {
	switch c {
	case Hard:
		return 0
	case Moderate:
		return 1
	}
	return 2
}

// CloudOrder gives a stable display order for clouds.
func CloudOrder(c string) int {
	switch c {
	case "aws":
		return 0
	case "gcp":
		return 1
	case "azure":
		return 2
	case "digitalocean":
		return 3
	}
	return 4
}
