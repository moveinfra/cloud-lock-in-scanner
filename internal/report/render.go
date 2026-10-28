package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/moveinfra/cloud-lock-in-scanner/internal/rules"
)

// JSON writes the full report, with every location.
func JSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(r)
}

func cloudName(c string) string {
	if n := rules.Clouds[c]; n != "" {
		return n
	}
	return c
}

func serviceName(f Finding) string {
	if f.Group && len(f.Matched) > 1 {
		return fmt.Sprintf("%s (%d types)", f.Service, len(f.Matched))
	}
	return f.Service
}

func headline(r *Report) string {
	if r.Score == nil {
		return "EU-portable: n/a"
	}
	return fmt.Sprintf("EU-portable: %d%%", *r.Score)
}

func summaryLine(r *Report) string {
	s := r.Summary
	noun := "services"
	if s.Services == 1 {
		noun = "service"
	}
	return fmt.Sprintf("%d %s: %d portable, %d moderate, %d hard", s.Services, noun, s.Portable, s.Moderate, s.Hard)
}

func filesLine(r *Report) string {
	s := fmt.Sprintf("%d files scanned", r.FilesScanned)
	if r.FilesScanned == 1 {
		s = "1 file scanned"
	}
	if r.FilesSkipped > 0 {
		s += fmt.Sprintf(", %d skipped (over 1 MB)", r.FilesSkipped)
	}
	return s
}

func locations(f Finding, format func(Location) string) string {
	var parts []string
	for i, l := range f.Locations {
		if i == MaxLocations {
			parts = append(parts, fmt.Sprintf("and %d more", len(f.Locations)-MaxLocations))
			break
		}
		parts = append(parts, format(l))
	}
	return strings.Join(parts, ", ")
}

// Text writes the terminal report.
func Text(w io.Writer, r *Report) error {
	var b strings.Builder
	if r.Score == nil {
		fmt.Fprintf(&b, "Nothing cloud-specific found in %s (%s).\n", r.Path, filesLine(r))
	} else {
		fmt.Fprintf(&b, "%s   %s\n", headline(r), summaryLine(r))
		fmt.Fprintf(&b, "Path %s, %s.\n\n", r.Path, filesLine(r))

		wClass, wCloud, wService := len("CLASS"), len("CLOUD"), len("SERVICE")
		for _, f := range r.Findings {
			wClass = max(wClass, len(f.Class))
			wCloud = max(wCloud, len(cloudName(f.Cloud)))
			wService = max(wService, len(serviceName(f)))
		}
		indent := strings.Repeat(" ", wClass+2)
		fmt.Fprintf(&b, "%-*s  %-*s  %-*s  %s\n", wClass, "CLASS", wCloud, "CLOUD", wService, "SERVICE", "USES")
		prev := ""
		for _, f := range r.Findings {
			if prev != "" && f.Class != prev {
				b.WriteString("\n")
			}
			prev = f.Class
			fmt.Fprintf(&b, "%-*s  %-*s  %-*s  %4d\n", wClass, f.Class, wCloud, cloudName(f.Cloud), wService, serviceName(f), f.Occurrences)
			fmt.Fprintf(&b, "%sEU: %s\n", indent, f.Replacement)
			fmt.Fprintf(&b, "%sat: %s\n", indent, locations(f, func(l Location) string {
				return fmt.Sprintf("%s:%d", l.File, l.Line)
			}))
		}
		fmt.Fprintf(&b, "\nScore %s.\n", Formula)
		b.WriteString("Static text matching: no cloud access, dynamic usage can be missed.\n")
	}
	fmt.Fprintf(&b, "Help moving to an EU cloud: %s\n", HelpURL)
	_, err := io.WriteString(w, b.String())
	return err
}

func mdCell(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ").Replace(s)
}

// Markdown writes the report for pull request comments and $GITHUB_STEP_SUMMARY.
func Markdown(w io.Writer, r *Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", headline(r))
	if r.Score == nil {
		fmt.Fprintf(&b, "Nothing cloud-specific found in `%s` (%s).\n\n", r.Path, filesLine(r))
	} else {
		s := r.Summary
		fmt.Fprintf(&b, "%d cloud services in `%s` (%s): **%d portable**, **%d moderate**, **%d hard**.\n\n",
			s.Services, r.Path, filesLine(r), s.Portable, s.Moderate, s.Hard)
		b.WriteString("| Class | Cloud | Service | Uses | EU replacement | Where |\n")
		b.WriteString("|---|---|---|---:|---|---|\n")
		for _, f := range r.Findings {
			fmt.Fprintf(&b, "| %s | %s | %s | %d | %s | %s |\n", f.Class, cloudName(f.Cloud), mdCell(serviceName(f)),
				f.Occurrences, mdCell(f.Replacement), locations(f, func(l Location) string {
					return fmt.Sprintf("`%s:%d`", mdCell(l.File), l.Line)
				}))
		}
		b.WriteString("\n<details><summary>How the score is calculated</summary>\n\n")
		b.WriteString("Each distinct service counts once: portable = 1, moderate = 0.5, hard = 0. ")
		b.WriteString("Score = round(100 × sum / number of services). ")
		b.WriteString("**Portable**: standard technology with a direct EU equivalent. ")
		b.WriteString("**Moderate**: needs a swap, but standard alternatives exist. ")
		b.WriteString("**Hard**: proprietary data model or runtime. ")
		b.WriteString("The scan matches text in Terraform, CloudFormation, Serverless, CDK and SDK imports; it has no cloud access.\n\n")
		b.WriteString("</details>\n\n")
	}
	fmt.Fprintf(&b, "<sub>[cloud-lock-in-scanner](%s) %s · Help moving to an EU cloud: [moveinfra.eu](%s)</sub>\n",
		RepoURL, r.Version, HelpURL)
	_, err := io.WriteString(w, b.String())
	return err
}

// Badge is a shields.io endpoint response (https://shields.io/badges/endpoint-badge).
type Badge struct {
	SchemaVersion int    `json:"schemaVersion"`
	Label         string `json:"label"`
	Message       string `json:"message"`
	Color         string `json:"color"`
}

// NewBadge returns the badge for a report.
func NewBadge(r *Report) Badge {
	b := Badge{SchemaVersion: 1, Label: "EU-portable", Message: "n/a", Color: "lightgrey"}
	if r.Score != nil {
		b.Message = fmt.Sprintf("%d%%", *r.Score)
		b.Color = BadgeColor(*r.Score)
	}
	return b
}

// BadgeColor picks the shields.io colour for a score.
func BadgeColor(score int) string {
	switch {
	case score >= 90:
		return "brightgreen"
	case score >= 75:
		return "green"
	case score >= 60:
		return "yellowgreen"
	case score >= 40:
		return "yellow"
	case score >= 20:
		return "orange"
	}
	return "red"
}

// WriteBadge writes the badge JSON.
func WriteBadge(w io.Writer, r *Report) error {
	b, err := json.Marshal(NewBadge(r))
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}
