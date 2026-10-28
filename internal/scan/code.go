package scan

import (
	"regexp"
	"strings"

	"github.com/moveinfra/cloud-lock-in-scanner/internal/rules"
)

// codePattern finds SDK or CDK references in source code. Group 1 (and group 2 when present) of re form the
// service name, unless names is set, which then splits group 1 into several names (for import lists).
type codePattern struct {
	re     *regexp.Regexp
	cloud  string
	source string
	// name turns the captured text into a service name. Return "" to drop the match.
	name func(captured string) string
	// list, when set, finds several names inside group 1 (e.g. `from google.cloud import storage, pubsub`).
	list *regexp.Regexp
	// requires, when set, must occur somewhere in the file for the pattern to apply.
	requires string
}

func ident(s string) string { return s }

func run(src string, lines lineIndex, pats []codePattern) []Hit {
	var hits []Hit
	for _, p := range pats {
		if p.requires != "" && !strings.Contains(src, p.requires) {
			continue
		}
		name := p.name
		if name == nil {
			name = ident
		}
		for _, m := range p.re.FindAllStringSubmatchIndex(src, -1) {
			type part struct {
				text string
				off  int
			}
			var parts []part
			if p.list != nil {
				group := src[m[2]:m[3]]
				group = pyComment.ReplaceAllStringFunc(group, func(c string) string {
					return strings.Repeat(" ", len(c)) // keep offsets
				})
				for _, lm := range p.list.FindAllStringSubmatchIndex(group, -1) {
					parts = append(parts, part{group[lm[2]:lm[3]], m[2] + lm[2]})
				}
			} else {
				text := src[m[2]:m[3]]
				if len(m) >= 6 && m[4] >= 0 {
					text += "/" + src[m[4]:m[5]]
				}
				parts = append(parts, part{text, m[2]})
			}
			for _, pt := range parts {
				n := name(pt.text)
				if n == "" {
					continue
				}
				hits = append(hits, Hit{
					Cloud: p.cloud, Kind: rules.KindSDK, Name: rules.NormalizeSDK(n), Source: p.source,
					Text: p.source + " " + pt.text, Line: lines.line(pt.off),
				})
			}
		}
	}
	return hits
}

var pyComment = regexp.MustCompile(`#[^\n]*`)

// importName matches one name in a Python import list, skipping `as alias` parts.
var importName = regexp.MustCompile(`(?:^|[\s(,])([A-Za-z_][A-Za-z0-9_]*)(?:\s+as\s+[A-Za-z_][A-Za-z0-9_]*)?`)

// awsCDKName matches aws_xxx in a CDK import list.
var awsCDKName = regexp.MustCompile(`\baws_([a-z0-9_]+)`)

var pythonPatterns = []codePattern{
	{re: regexp.MustCompile(`\.(?:client|resource)\(\s*(?:service_name\s*=\s*)?['"]([A-Za-z0-9-]+)['"]`),
		cloud: "aws", source: "boto3", requires: "boto"},
	{re: regexp.MustCompile(`(?m)^[ \t]*(?:from|import)[ \t]+google\.cloud\.([a-z0-9_]+)`), cloud: "gcp", source: "google-cloud"},
	{re: regexp.MustCompile(`(?m)^[ \t]*from[ \t]+google\.cloud[ \t]+import[ \t]+(\([^)]*\)|[^\n]+)`),
		cloud: "gcp", source: "google-cloud", list: importName},
	{re: regexp.MustCompile(`(?m)^[ \t]*(?:from|import)[ \t]+(vertexai)\b`), cloud: "gcp", source: "google-cloud"},
	{re: regexp.MustCompile(`(?m)^[ \t]*(?:from|import)[ \t]+azure\.([A-Za-z0-9_.]+)`), cloud: "azure", source: "azure-sdk"},
	{re: regexp.MustCompile(`aws_cdk\.aws_([a-z0-9_]+)`), cloud: "aws", source: "aws-cdk"},
	{re: regexp.MustCompile(`(?m)^[ \t]*from[ \t]+aws_cdk[ \t]+import[ \t]+(\([^)]*\)|[^\n]+)`),
		cloud: "aws", source: "aws-cdk", list: awsCDKName},
}

func python(src string, lines lineIndex) []Hit { return run(src, lines, pythonPatterns) }

var pythonManifestPatterns = []codePattern{
	{re: regexp.MustCompile(`(?m)(?:^|[\s"'\[,])google-cloud-([a-z0-9-]+)`), cloud: "gcp", source: "python package"},
	{re: regexp.MustCompile(`(?m)(?:^|[\s"'\[,])azure-([a-z0-9-]+)`), cloud: "azure", source: "python package"},
}

func pythonManifest(src string, lines lineIndex) []Hit {
	return run(src, lines, pythonManifestPatterns)
}

// awsV2NotService filters `new AWS.Xxx` constructors that are SDK plumbing, not services.
var awsV2NotService = regexp.MustCompile(`(Credentials|Endpoint|Config|Request|Response|Service|Signer|Signers|Validator|Executor|Listeners|Error)$`)

func awsV2(s string) string {
	if awsV2NotService.MatchString(s) {
		return ""
	}
	return s
}

func azureJS(s string) string {
	if strings.HasPrefix(s, "arm-") {
		return "mgmt" + s[len("arm-"):]
	}
	return s
}

var jsIdent = regexp.MustCompile(`(?:^|[\s{,])([A-Z][A-Za-z0-9]*)(?:\s+as\s+\w+|\s*:\s*\w+)?`)

var javascriptPatterns = []codePattern{
	{re: regexp.MustCompile(`@aws-sdk/client-([a-z0-9-]+)`), cloud: "aws", source: "@aws-sdk"},
	{re: regexp.MustCompile(`@aws-sdk/(?:lib|util)-(dynamodb)\b`), cloud: "aws", source: "@aws-sdk"},
	{re: regexp.MustCompile(`@aws-sdk/(lib-storage|s3-request-presigner|s3-presigned-post)\b`), cloud: "aws", source: "@aws-sdk",
		name: func(string) string { return "s3" }},
	{re: regexp.MustCompile(`['"]aws-sdk/clients/([a-z0-9]+)['"]`), cloud: "aws", source: "aws-sdk v2"},
	{re: regexp.MustCompile(`\bnew\s+AWS\.([A-Z][A-Za-z0-9]*)`), cloud: "aws", source: "aws-sdk v2", name: awsV2},
	{re: regexp.MustCompile(`import\s*\{([^}]*)\}\s*from\s*['"]aws-sdk['"]`), cloud: "aws", source: "aws-sdk v2",
		list: jsIdent, name: awsV2},
	{re: regexp.MustCompile(`\{([^}]*)\}\s*=\s*require\(\s*['"]aws-sdk['"]\s*\)`), cloud: "aws", source: "aws-sdk v2",
		list: jsIdent, name: awsV2},
	{re: regexp.MustCompile(`['"](?:aws-cdk-lib|@aws-cdk)/aws-([a-z0-9-]+)`), cloud: "aws", source: "aws-cdk"},
	{re: regexp.MustCompile(`import\s*\{([^}]*)\}\s*from\s*['"]aws-cdk-lib['"]`), cloud: "aws", source: "aws-cdk",
		list: awsCDKName},
	{re: regexp.MustCompile(`['"]@google-cloud/([a-z0-9-]+)`), cloud: "gcp", source: "@google-cloud"},
	{re: regexp.MustCompile(`['"]@azure/([a-z0-9-]+)`), cloud: "azure", source: "@azure", name: azureJS},
}

func javascript(src string, lines lineIndex) []Hit { return run(src, lines, javascriptPatterns) }

// azureGo turns a path below github.com/Azure/azure-sdk-for-go/sdk/ into a service name: the last element
// without its "az" prefix (data/azcosmos -> cosmos), or "mgmt" + service for resourcemanager/<service>/arm<service>.
func azureGo(p string) string {
	parts := strings.Split(strings.Trim(p, "/"), "/")
	for len(parts) > 1 && isVersion(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	if parts[0] == "resourcemanager" && len(parts) > 1 {
		return "mgmt" + parts[1]
	}
	last := parts[len(parts)-1]
	return strings.TrimPrefix(last, "az")
}

func isVersion(s string) bool {
	return len(s) > 1 && s[0] == 'v' && strings.Trim(s[1:], "0123456789") == ""
}

var goPatterns = []codePattern{
	{re: regexp.MustCompile(`"github\.com/aws/aws-sdk-go-v2/service/([a-z0-9]+)`), cloud: "aws", source: "aws-sdk-go-v2"},
	{re: regexp.MustCompile(`"github\.com/aws/aws-sdk-go/service/([a-z0-9]+)`), cloud: "aws", source: "aws-sdk-go"},
	{re: regexp.MustCompile(`"github\.com/aws/aws-cdk-go/awscdk(?:/v2)?/aws([a-z0-9]+)`), cloud: "aws", source: "aws-cdk"},
	{re: regexp.MustCompile(`"cloud\.google\.com/go/([a-z0-9]+)(?:/([a-z0-9]+))?`), cloud: "gcp", source: "cloud.google.com/go"},
	{re: regexp.MustCompile(`"github\.com/Azure/azure-sdk-for-go/sdk/([A-Za-z0-9/_.-]+)"`), cloud: "azure",
		source: "azure-sdk-for-go", name: azureGo},
}

func golang(src string, lines lineIndex) []Hit { return run(src, lines, goPatterns) }

func gcpJava(s string) string {
	if s == "spring" || s == "tools" {
		return ""
	}
	return s
}

var javaPatterns = []codePattern{
	{re: regexp.MustCompile(`software\.amazon\.awssdk\.services\.([a-z0-9]+)`), cloud: "aws", source: "aws-sdk-java-v2"},
	{re: regexp.MustCompile(`com\.amazonaws\.services\.([a-z0-9]+)`), cloud: "aws", source: "aws-sdk-java"},
	{re: regexp.MustCompile(`software\.amazon\.awscdk\.services\.([a-z0-9]+)`), cloud: "aws", source: "aws-cdk"},
	{re: regexp.MustCompile(`com\.google\.cloud\.([a-z][a-z0-9]*)\.`), cloud: "gcp", source: "google-cloud-java", name: gcpJava},
}

func java(src string, lines lineIndex) []Hit { return run(src, lines, javaPatterns) }
