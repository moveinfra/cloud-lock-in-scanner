package scan

import (
	"regexp"
	"strings"

	"github.com/moveinfra/cloud-lock-in-scanner/internal/rules"
)

var (
	tfBlock    = regexp.MustCompile(`(?m)^[ \t]*(resource|data)[ \t]+"([A-Za-z0-9_-]+)"[ \t]+"[^"]*"`)
	tfTopLevel = regexp.MustCompile(`(?m)^(resource|data|module|variable|output|locals|provider|terraform|moved|import|check|removed)\b`)
	tfModule   = regexp.MustCompile(`(?m)^[ \t]*source[ \t]*=[ \t]*"(?:registry\.terraform\.io/|git::https://github\.com/|github\.com/)?(terraform-(?:aws|google)-modules)/(?:terraform-(?:aws|google)-)?([a-z0-9-]+)(?:/(aws|google))?[^"]*"`)
)

// tfPrefixes maps a Terraform type prefix to its cloud. Unknown types with these prefixes still count, as an
// unknown service of that cloud.
var tfPrefixes = []struct{ prefix, cloud string }{
	{"aws_", "aws"},
	{"google_", "gcp"},
	{"azurerm_", "azure"},
	{"azuread_", "azure"},
	{"azapi_", "azure"},
	{"digitalocean_", "digitalocean"},
}

// tfModules maps popular registry modules to the main resource type they create.
var tfModules = map[string]string{
	"terraform-aws-modules/vpc":                  "aws_vpc",
	"terraform-aws-modules/security-group":       "aws_security_group",
	"terraform-aws-modules/iam":                  "aws_iam_role",
	"terraform-aws-modules/ec2-instance":         "aws_instance",
	"terraform-aws-modules/autoscaling":          "aws_autoscaling_group",
	"terraform-aws-modules/eks":                  "aws_eks_cluster",
	"terraform-aws-modules/ecs":                  "aws_ecs_cluster",
	"terraform-aws-modules/rds":                  "aws_db_instance",
	"terraform-aws-modules/rds-aurora":           "aws_rds_cluster",
	"terraform-aws-modules/s3-bucket":            "aws_s3_bucket",
	"terraform-aws-modules/alb":                  "aws_lb",
	"terraform-aws-modules/elb":                  "aws_elb",
	"terraform-aws-modules/route53":              "aws_route53_zone",
	"terraform-aws-modules/acm":                  "aws_acm_certificate",
	"terraform-aws-modules/cloudfront":           "aws_cloudfront_distribution",
	"terraform-aws-modules/elasticache":          "aws_elasticache_cluster",
	"terraform-aws-modules/kms":                  "aws_kms_key",
	"terraform-aws-modules/secrets-manager":      "aws_secretsmanager_secret",
	"terraform-aws-modules/ecr":                  "aws_ecr_repository",
	"terraform-aws-modules/efs":                  "aws_efs_file_system",
	"terraform-aws-modules/msk-kafka-cluster":    "aws_msk_cluster",
	"terraform-aws-modules/lambda":               "aws_lambda_function",
	"terraform-aws-modules/step-functions":       "aws_sfn_state_machine",
	"terraform-aws-modules/dynamodb-table":       "aws_dynamodb_table",
	"terraform-aws-modules/sqs":                  "aws_sqs_queue",
	"terraform-aws-modules/sns":                  "aws_sns_topic",
	"terraform-aws-modules/eventbridge":          "aws_cloudwatch_event_bus",
	"terraform-aws-modules/apigateway-v2":        "aws_apigatewayv2_api",
	"terraform-aws-modules/appsync":              "aws_appsync_graphql_api",
	"terraform-aws-modules/cloudwatch":           "aws_cloudwatch_metric_alarm",
	"terraform-google-modules/network":           "google_compute_network",
	"terraform-google-modules/project-factory":   "google_project",
	"terraform-google-modules/kubernetes-engine": "google_container_cluster",
	"terraform-google-modules/sql-db":            "google_sql_database_instance",
	"terraform-google-modules/cloud-storage":     "google_storage_bucket",
	"terraform-google-modules/pubsub":            "google_pubsub_topic",
	"terraform-google-modules/bigquery":          "google_bigquery_dataset",
	"terraform-google-modules/memorystore":       "google_redis_instance",
	"terraform-google-modules/lb-http":           "google_compute_backend_service",
	"terraform-google-modules/cloud-dns":         "google_dns_managed_zone",
	"terraform-google-modules/event-function":    "google_cloudfunctions_function",
}

func tfCloud(typ string) string {
	for _, p := range tfPrefixes {
		if strings.HasPrefix(typ, p.prefix) {
			return p.cloud
		}
	}
	return ""
}

func terraform(src string, lines lineIndex) []Hit {
	var hits []Hit
	tops := tfTopLevel.FindAllStringIndex(src, -1)
	for _, m := range tfBlock.FindAllStringSubmatchIndex(src, -1) {
		typ := src[m[4]:m[5]]
		cloud := tfCloud(typ)
		if cloud == "" {
			continue
		}
		// The body runs to the next top-level block.
		end := len(src)
		for _, t := range tops {
			if t[0] > m[1] {
				end = t[0]
				break
			}
		}
		source := "terraform resource"
		if src[m[2]:m[3]] == "data" {
			source = "terraform data source"
		}
		hits = append(hits, Hit{
			Cloud: cloud, Kind: rules.KindTerraform, Name: typ, Source: source, Text: typ,
			Body: src[m[1]:end], Line: lines.line(m[4]),
		})
	}
	for _, m := range tfModule.FindAllStringSubmatchIndex(src, -1) {
		mod := src[m[2]:m[3]] + "/" + src[m[4]:m[5]]
		typ, ok := tfModules[mod]
		if !ok {
			continue
		}
		hits = append(hits, Hit{
			Cloud: tfCloud(typ), Kind: rules.KindTerraform, Name: typ, Source: "terraform module",
			Text: mod, Line: lines.line(m[2]),
		})
	}
	return hits
}
