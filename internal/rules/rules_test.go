package rules

import (
	"strings"
	"testing"
)

func TestDefaultLoads(t *testing.T) {
	s := Default()
	n := s.CountByCloud()
	for c := range Clouds {
		if n[c] == 0 {
			t.Errorf("no rules for %s", c)
		}
	}
	for _, r := range s.Rules {
		if strings.HasSuffix(r.Replacement, ".") {
			t.Errorf("%s: replacement ends with a full stop; the report adds none", r.ID)
		}
	}
}

func TestMatch(t *testing.T) {
	s := Default()
	tests := []struct {
		kind, cloud, name, body string
		want                    string // rule id, "" for no match
	}{
		{KindTerraform, "aws", "aws_dynamodb_table", "", "aws.dynamodb"},
		{KindTerraform, "aws", "aws_iam_role_policy", "", "aws.network-iam"},
		{KindTerraform, "aws", "aws_route53_zone", "", "aws.route53"},              // not aws_route*
		{KindTerraform, "aws", "aws_route_table", "", "aws.network-iam"},           // exact glue
		{KindTerraform, "aws", "aws_cloudwatch_event_rule", "", "aws.eventbridge"}, // longer prefix wins
		{KindTerraform, "aws", "aws_cloudwatch_log_group", "", "aws.cloudwatch"},
		{KindTerraform, "aws", "aws_ec2_transit_gateway", "", "aws.network-iam"},
		{KindTerraform, "aws", "aws_ec2_host", "", "aws.ec2"},
		{KindTerraform, "aws", "aws_lb_listener", "", "aws.elb"},
		{KindTerraform, "aws", "aws_appautoscaling_target", "", "aws.ecs"},
		{KindTerraform, "aws", "aws_rds_cluster", `engine = "aurora-postgresql"`, "aws.rds"},
		{KindTerraform, "aws", "aws_rds_cluster", "\n  engine_mode = \"serverless\"\n", "aws.aurora-serverless"},
		{KindTerraform, "aws", "aws_rds_cluster", "serverlessv2_scaling_configuration {", "aws.aurora-serverless"},
		{KindTerraform, "aws", "aws_gamelift_fleet", "", ""},
		{KindTerraform, "gcp", "google_compute_instance", "", "gcp.compute"},
		{KindTerraform, "gcp", "google_compute_firewall", "", "gcp.network-iam"},
		{KindTerraform, "gcp", "google_compute_backend_service", "", "gcp.lb"},
		{KindTerraform, "gcp", "google_compute_target_https_proxy", "", "gcp.lb"},
		{KindTerraform, "gcp", "google_project_iam_member", "", "gcp.network-iam"},
		{KindTerraform, "gcp", "google_cloud_run_v2_service", "", "gcp.run"},
		{KindTerraform, "azure", "azurerm_storage_account", "", "azure.storage"},
		{KindTerraform, "azure", "azurerm_storage_queue", "", "azure.servicebus"},
		{KindTerraform, "azure", "azurerm_storage_table", "", "azure.cosmos"},
		{KindTerraform, "azure", "azuread_application", "", "azure.network-iam"},
		{KindTerraform, "digitalocean", "digitalocean_database_cluster", `engine = "redis"`, "digitalocean.cache"},
		{KindTerraform, "digitalocean", "digitalocean_database_cluster", `engine = "pg"`, "digitalocean.database"},
		{KindTerraform, "digitalocean", "digitalocean_database_firewall", "", "digitalocean.database"},
		{KindTerraform, "gcp", "aws_s3_bucket", "", ""}, // wrong cloud
		{KindCloudFormation, "aws", "AWS::DynamoDB::GlobalTable", "", "aws.dynamodb"},
		{KindCloudFormation, "aws", "AWS::Serverless::Function", "", "aws.lambda"},
		{KindCloudFormation, "aws", "AWS::EC2::Instance", "", "aws.ec2"},
		{KindCloudFormation, "aws", "AWS::EC2::SecurityGroupIngress", "", "aws.network-iam"},
		{KindCloudFormation, "aws", "AWS::Events::Rule", "", "aws.eventbridge"},
		{KindSDK, "aws", "cognito-idp", "", "aws.cognito"},
		{KindSDK, "aws", "rds-data", "", "aws.aurora-serverless"}, // exact beats rds*
		{KindSDK, "aws", "rds", "", "aws.rds"},
		{KindSDK, "aws", "elbv2", "", "aws.elb"},
		{KindSDK, "aws", "sfn", "", "aws.stepfunctions"},
		{KindSDK, "aws", "dynamodbstreams", "", "aws.dynamodb"},
		{KindSDK, "aws", "lambdanodejs", "", "aws.lambda"},
		{KindSDK, "aws", "sts", "", "aws.network-iam"},
		{KindSDK, "aws", "logsdestinations", "", "aws.cloudwatch"},
		{KindSDK, "aws", "gamelift", "", ""},
		{KindSDK, "gcp", "bigquery_storage_v1", "", "gcp.bigquery"},
		{KindSDK, "gcp", "computemetadata", "", "gcp.network-iam"},
		{KindSDK, "gcp", "compute_v1", "", "gcp.compute"},
		{KindSDK, "gcp", "functions-framework", "", "gcp.functions"},
		{KindSDK, "azure", "storage-blob", "", "azure.storage"},
		{KindSDK, "azure", "storage-queue", "", "azure.servicebus"},
		{KindSDK, "azure", "service-bus", "", "azure.servicebus"},
		{KindSDK, "azure", "keyvault-secrets", "", "azure.keyvault"},
		{KindSDK, "azure", "core-rest-pipeline", "", "azure.network-iam"},
		{KindSDK, "azure", "functions", "", "azure.functions"},
	}
	for _, tt := range tests {
		got := ""
		if r := s.Match(tt.kind, tt.cloud, tt.name, tt.body); r != nil {
			got = r.ID
		}
		if got != tt.want {
			t.Errorf("Match(%s, %s, %q) = %q, want %q", tt.kind, tt.cloud, tt.name, got, tt.want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	base := `{"id":"x.a","cloud":"aws","service":"A","class":"portable","replacement":"B","terraform":["aws_a"]}`
	tests := map[string]string{
		"bad class":         strings.Replace(base, `"portable"`, `"easy"`, 1),
		"bad cloud":         strings.Replace(base, `"aws"`, `"ibm"`, 1),
		"bad capability":    strings.Replace(base, `"class"`, `"capability":"databases","class"`, 1),
		"no patterns":       strings.Replace(base, `"terraform":["aws_a"]`, `"terraform":[]`, 1),
		"no replacement":    strings.Replace(base, `"replacement":"B"`, `"replacement":""`, 1),
		"duplicate id":      base + "," + base,
		"duplicate pattern": base + "," + strings.Replace(base, `"x.a"`, `"x.b"`, 1),
		"bad body regexp":   strings.Replace(base, `"terraform"`, `"terraform_body":["("],"terraform"`, 1),
		"match-all pattern": strings.Replace(base, `["aws_a"]`, `["*"]`, 1),
	}
	for name, rules := range tests {
		if _, err := Parse([]byte(`{"schema_version":1,"rules":[` + rules + `]}`)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Parse([]byte(`{"schema_version":1,"rules":[` + base + `]}`)); err != nil {
		t.Errorf("valid rule rejected: %v", err)
	}
}

func TestNormalizeSDK(t *testing.T) {
	for in, want := range map[string]string{
		"cognito-idp": "cognitoidp", "DynamoDB": "dynamodb", "pubsub_v1": "pubsubv1", "storage.blob": "storageblob",
		"ses*": "ses*",
	} {
		if got := NormalizeSDK(in); got != want {
			t.Errorf("NormalizeSDK(%q) = %q, want %q", in, got, want)
		}
	}
}
