// Package report turns scan hits into findings (one per distinct service), calculates the portability score and
// renders the result as text, JSON, Markdown or a shields.io badge.
package report

import (
	"math"
	"sort"
	"strconv"

	"github.com/moveinfra/cloud-lock-in-scanner/internal/rules"
	"github.com/moveinfra/cloud-lock-in-scanner/internal/scan"
)

// HelpURL is linked once in the report footer.
const HelpURL = "https://moveinfra.eu/"

// RepoURL is the project home.
const RepoURL = "https://github.com/moveinfra/cloud-lock-in-scanner"

// Formula describes the score in one line.
const Formula = "per distinct service: portable = 1, moderate = 0.5, hard = 0; score = round(100 × sum / services)"

// MaxLocations is how many locations the text and Markdown outputs list per finding.
const MaxLocations = 5

// Location is one place a service is referenced.
type Location struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"match"`
}

// Finding is one distinct service.
type Finding struct {
	Rule        string     `json:"rule"`
	Cloud       string     `json:"cloud"`
	Service     string     `json:"service"`
	Class       string     `json:"class"`
	Weight      float64    `json:"weight"`
	Capability  string     `json:"capability,omitempty"`
	Replacement string     `json:"replacement"`
	Unknown     bool       `json:"unknown,omitempty"`
	Group       bool       `json:"group,omitempty"`
	Matched     []string   `json:"matched"`
	Occurrences int        `json:"occurrences"`
	Locations   []Location `json:"locations"`
}

// Summary counts services by class.
type Summary struct {
	Services int `json:"services"`
	Portable int `json:"portable"`
	Moderate int `json:"moderate"`
	Hard     int `json:"hard"`
}

// Report is the full result of a scan.
type Report struct {
	Tool         string    `json:"tool"`
	Version      string    `json:"version"`
	Path         string    `json:"path"`
	FilesScanned int       `json:"files_scanned"`
	FilesSkipped int       `json:"files_skipped"`
	Score        *int      `json:"score"` // nil when no cloud service was found
	Summary      Summary   `json:"summary"`
	Clouds       []string  `json:"clouds"`
	Findings     []Finding `json:"findings"`
	Formula      string    `json:"formula"`
	Help         string    `json:"help"`
}

// Build groups hits by service and scores them.
func Build(res *scan.Result, set *rules.Set, version string) *Report {
	r := &Report{
		Tool: "cloud-lock-in-scanner", Version: version, Path: res.Root,
		FilesScanned: res.FilesScanned, FilesSkipped: res.FilesSkipped,
		Clouds: []string{}, Findings: []Finding{}, Formula: Formula, Help: HelpURL,
	}
	byID := map[string]*Finding{}
	var order []string
	seenLoc := map[string]bool{}
	seenText := map[string]bool{}
	for _, h := range res.Hits {
		rule := set.Match(h.Kind, h.Cloud, h.Name, h.Body)
		var f *Finding
		if rule != nil {
			f = byID[rule.ID]
			if f == nil {
				f = &Finding{
					Rule: rule.ID, Cloud: rule.Cloud, Service: rule.Service, Class: rule.Class,
					Capability: rule.Capability, Replacement: rule.Replacement, Group: rule.Group,
				}
			}
		} else {
			id := h.Cloud + ".unknown:" + h.Name
			f = byID[id]
			if f == nil {
				f = &Finding{
					Rule: id, Cloud: h.Cloud, Service: "Unknown service: " + h.Text, Class: rules.Moderate,
					Replacement: "Not in the rules yet. Check this one by hand, and consider adding a rule",
					Unknown:     true,
				}
			}
		}
		if byID[f.Rule] == nil {
			f.Weight = rules.Weight[f.Class]
			byID[f.Rule] = f
			order = append(order, f.Rule)
		}
		if k := f.Rule + "\x00" + h.Text; !seenText[k] {
			seenText[k] = true
			f.Matched = append(f.Matched, h.Text)
		}
		if k := f.Rule + "\x00" + h.File + "\x00" + strconv.Itoa(h.Line); !seenLoc[k] {
			seenLoc[k] = true
			f.Locations = append(f.Locations, Location{File: h.File, Line: h.Line, Text: h.Text})
		}
	}

	clouds := map[string]bool{}
	sum := 0.0
	for _, id := range order {
		f := byID[id]
		f.Occurrences = len(f.Locations)
		sort.Strings(f.Matched)
		sort.SliceStable(f.Locations, func(i, j int) bool {
			a, b := f.Locations[i], f.Locations[j]
			if a.File != b.File {
				return a.File < b.File
			}
			return a.Line < b.Line
		})
		r.Findings = append(r.Findings, *f)
		clouds[f.Cloud] = true
		sum += f.Weight
		switch f.Class {
		case rules.Portable:
			r.Summary.Portable++
		case rules.Moderate:
			r.Summary.Moderate++
		case rules.Hard:
			r.Summary.Hard++
		}
	}
	r.Summary.Services = len(r.Findings)
	if n := r.Summary.Services; n > 0 {
		s := Score(sum, n)
		r.Score = &s
	}
	for c := range clouds {
		r.Clouds = append(r.Clouds, c)
	}
	sort.Slice(r.Clouds, func(i, j int) bool { return rules.CloudOrder(r.Clouds[i]) < rules.CloudOrder(r.Clouds[j]) })
	sort.SliceStable(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		if a.Class != b.Class {
			return rules.ClassOrder(a.Class) < rules.ClassOrder(b.Class)
		}
		if a.Group != b.Group {
			return b.Group // glue last within its class
		}
		if a.Cloud != b.Cloud {
			return rules.CloudOrder(a.Cloud) < rules.CloudOrder(b.Cloud)
		}
		return a.Service < b.Service
	})
	return r
}

// Score is round(100 * sum / services), with halves rounded up.
func Score(sum float64, services int) int {
	return int(math.Floor(100*sum/float64(services) + 0.5))
}
