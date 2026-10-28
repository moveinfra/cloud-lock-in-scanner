package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/moveinfra/cloud-lock-in-scanner/internal/rules"
	"github.com/moveinfra/cloud-lock-in-scanner/internal/scan"
)

func tf(typ, file string, line int) scan.Hit {
	cloud := "aws"
	switch {
	case strings.HasPrefix(typ, "google_"):
		cloud = "gcp"
	case strings.HasPrefix(typ, "azurerm_"):
		cloud = "azure"
	}
	return scan.Hit{Cloud: cloud, Kind: rules.KindTerraform, Name: typ, Text: typ, File: file, Line: line}
}

func sdk(cloud, name, file string, line int) scan.Hit {
	return scan.Hit{Cloud: cloud, Kind: rules.KindSDK, Name: name, Text: "sdk " + name, File: file, Line: line}
}

func build(hits ...scan.Hit) *Report {
	return Build(&scan.Result{Root: "repo", FilesScanned: 3, Hits: hits}, rules.Default(), "test")
}

func TestScore(t *testing.T) {
	tests := []struct {
		name string
		hits []scan.Hit
		want *int
	}{
		{"nothing found", nil, nil},
		{"one portable", []scan.Hit{tf("aws_s3_bucket", "a.tf", 1)}, ptr(100)},
		{"one hard", []scan.Hit{tf("aws_dynamodb_table", "a.tf", 1)}, ptr(0)},
		{"one moderate", []scan.Hit{tf("aws_sqs_queue", "a.tf", 1)}, ptr(50)},
		{"counted per service, not per use", []scan.Hit{
			tf("aws_s3_bucket", "a.tf", 1), tf("aws_s3_bucket_policy", "a.tf", 5), sdk("aws", "s3", "b.py", 2),
			tf("aws_dynamodb_table", "a.tf", 9),
		}, ptr(50)},
		{"portable + moderate + hard", []scan.Hit{
			tf("aws_s3_bucket", "a.tf", 1), tf("aws_sqs_queue", "a.tf", 2), tf("aws_lambda_function", "a.tf", 3),
		}, ptr(50)},
		{"rounds half up: 2.5 of 4", []scan.Hit{
			tf("aws_s3_bucket", "a.tf", 1), tf("aws_instance", "a.tf", 2), tf("aws_sqs_queue", "a.tf", 3),
			tf("aws_lambda_function", "a.tf", 4),
		}, ptr(63)},
		{"unknown counts as moderate", []scan.Hit{tf("aws_gamelift_fleet", "a.tf", 1), tf("aws_s3_bucket", "a.tf", 2)},
			ptr(75)},
		{"glue is one service", []scan.Hit{
			tf("aws_vpc", "a.tf", 1), tf("aws_subnet", "a.tf", 2), tf("aws_security_group", "a.tf", 3),
			tf("aws_iam_role", "a.tf", 4), tf("aws_route_table", "a.tf", 5), tf("aws_dynamodb_table", "a.tf", 6),
		}, ptr(50)},
		{"same service on two clouds counts twice", []scan.Hit{
			tf("aws_s3_bucket", "a.tf", 1), tf("google_bigquery_dataset", "b.tf", 1),
		}, ptr(50)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := build(tt.hits...)
			switch {
			case tt.want == nil && r.Score != nil:
				t.Errorf("score = %d, want none", *r.Score)
			case tt.want != nil && r.Score == nil:
				t.Errorf("score = none, want %d", *tt.want)
			case tt.want != nil && *r.Score != *tt.want:
				t.Errorf("score = %d, want %d", *r.Score, *tt.want)
			}
		})
	}
}

func ptr(n int) *int { return &n }

func TestFindings(t *testing.T) {
	r := build(
		tf("aws_s3_bucket", "a.tf", 1), sdk("aws", "s3", "b.py", 2), sdk("aws", "s3", "b.py", 2), // same place twice
		tf("aws_vpc", "a.tf", 3), tf("aws_subnet", "a.tf", 4),
		tf("aws_dynamodb_table", "a.tf", 5), tf("aws_gamelift_fleet", "a.tf", 6),
	)
	if r.Summary != (Summary{Services: 4, Portable: 2, Moderate: 1, Hard: 1}) {
		t.Errorf("summary = %+v", r.Summary)
	}
	var order []string
	for _, f := range r.Findings {
		order = append(order, f.Rule)
	}
	want := "aws.dynamodb aws.unknown:aws_gamelift_fleet aws.s3 aws.network-iam"
	if got := strings.Join(order, " "); got != want {
		t.Errorf("order = %s, want %s (hard first, glue last)", got, want)
	}
	s3 := r.Findings[2]
	if s3.Occurrences != 2 || len(s3.Matched) != 2 || s3.Capability != "object-storage" {
		t.Errorf("s3 finding = %+v", s3)
	}
	unknown := r.Findings[1]
	if !unknown.Unknown || unknown.Class != rules.Moderate || unknown.Weight != 0.5 {
		t.Errorf("unknown finding = %+v", unknown)
	}
	if !r.Findings[3].Group || len(r.Findings[3].Matched) != 2 {
		t.Errorf("glue finding = %+v", r.Findings[3])
	}
}

func TestTextAndMarkdown(t *testing.T) {
	var hits []scan.Hit
	for i := 1; i <= 8; i++ {
		hits = append(hits, tf("aws_dynamodb_table", "db.tf", i))
	}
	hits = append(hits, tf("aws_vpc", "net.tf", 1), tf("aws_subnet", "net.tf", 2))
	r := build(hits...)

	var text bytes.Buffer
	if err := Text(&text, r); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"EU-portable: 50%", "2 services: 1 portable, 0 moderate, 1 hard", "DynamoDB",
		"db.tf:1, db.tf:2, db.tf:3, db.tf:4, db.tf:5, and 3 more", "Networking & IAM (2 types)", HelpURL,
	} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("text report lacks %q:\n%s", want, text.String())
		}
	}

	var md bytes.Buffer
	if err := Markdown(&md, r); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## EU-portable: 50%", "| hard | AWS | DynamoDB | 8 |", "`db.tf:5`, and 3 more |", "[moveinfra.eu](" + HelpURL + ")",
	} {
		if !strings.Contains(md.String(), want) {
			t.Errorf("markdown report lacks %q:\n%s", want, md.String())
		}
	}
	if n := strings.Count(md.String(), HelpURL); n != 1 {
		t.Errorf("markdown links %s %d times, want once", HelpURL, n)
	}
}

func TestNothingFound(t *testing.T) {
	r := build()
	var text, md, js bytes.Buffer
	_ = Text(&text, r)
	_ = Markdown(&md, r)
	_ = JSON(&js, r)
	if !strings.Contains(text.String(), "Nothing cloud-specific found in repo") {
		t.Errorf("text: %s", text.String())
	}
	if !strings.Contains(md.String(), "EU-portable: n/a") {
		t.Errorf("markdown: %s", md.String())
	}
	var parsed map[string]any
	if err := json.Unmarshal(js.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["score"] != nil {
		t.Errorf("json score = %v, want null", parsed["score"])
	}
	if f, ok := parsed["findings"].([]any); !ok || len(f) != 0 {
		t.Errorf("json findings = %v, want []", parsed["findings"])
	}
	if b := NewBadge(r); b.Message != "n/a" || b.Color != "lightgrey" {
		t.Errorf("badge = %+v", b)
	}
}

func TestBadge(t *testing.T) {
	for score, want := range map[int]string{
		100: "brightgreen", 90: "brightgreen", 89: "green", 75: "green", 74: "yellowgreen", 60: "yellowgreen",
		59: "yellow", 40: "yellow", 39: "orange", 20: "orange", 19: "red", 0: "red",
	} {
		if got := BadgeColor(score); got != want {
			t.Errorf("BadgeColor(%d) = %s, want %s", score, got, want)
		}
	}
	var buf bytes.Buffer
	if err := WriteBadge(&buf, build(tf("aws_s3_bucket", "a.tf", 1))); err != nil {
		t.Fatal(err)
	}
	want := `{"schemaVersion":1,"label":"EU-portable","message":"100%","color":"brightgreen"}` + "\n"
	if buf.String() != want {
		t.Errorf("badge = %s, want %s", buf.String(), want)
	}
}
