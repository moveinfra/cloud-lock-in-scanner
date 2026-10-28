package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// short renders hits as "cloud name line", sorted, for easy comparison.
func short(hits []Hit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, fmt.Sprintf("%s %s %d", h.Cloud, h.Name, h.Line))
	}
	slices.Sort(out)
	return out
}

func sorted(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)
	return s
}

func TestFixtures(t *testing.T) {
	tests := []struct {
		file string
		want []string
	}{
		{"terraform/aws.tf", []string{
			"aws aws_vpc 3", "aws aws_subnet 7", "aws aws_security_group 12", "aws aws_iam_role 16",
			"aws aws_iam_policy_document 21", "aws aws_instance 27", "aws aws_db_instance 32", "aws aws_rds_cluster 37",
			"aws aws_dynamodb_table 47", "aws aws_lambda_function 52", "aws aws_sqs_queue 57", "aws aws_s3_bucket 61",
			"aws aws_lb 65", "aws aws_gamelift_fleet 69", "aws aws_eks_cluster 74", // last one from the eks module
		}},
		{"terraform/gcp.tf", []string{
			"gcp google_compute_network 1", "gcp google_container_cluster 5", "gcp google_bigquery_dataset 10",
			"gcp google_pubsub_topic 14", "gcp google_storage_bucket 18",
		}},
		{"terraform/azure.tf", []string{
			"azure azurerm_resource_group 1", "azure azurerm_kubernetes_cluster 6", "azure azurerm_cosmosdb_account 11",
			"azure azurerm_linux_function_app 15",
		}},
		{"terraform/digitalocean.tf", []string{
			"digitalocean digitalocean_droplet 1", "digitalocean digitalocean_database_cluster 7",
			"digitalocean digitalocean_database_cluster 13",
		}},
		{"cloudformation/template.yaml", []string{
			"aws AWS::DynamoDB::Table 6", "aws AWS::SQS::Queue 12", "aws AWS::IAM::Role 17",
			"aws AWS::StepFunctions::StateMachine 22", "aws AWS::Kinesis::Stream 27",
		}},
		{"cloudformation/template.json", []string{"aws AWS::S3::Bucket 4", "aws AWS::SNS::Topic 6"}},
		{"sam/template.yaml", []string{"aws AWS::Serverless::Api 6", "aws AWS::Serverless::Function 11"}},
		{"serverless/serverless.yml", []string{
			"aws lambda 8", "aws apigateway 12", "aws sqs 18", "aws dynamodbstreams 20", "aws eventbridge 26",
			"aws AWS::DynamoDB::Table 31", "aws AWS::SQS::Queue 33",
		}},
		{"python/app.py", []string{
			"gcp bigquery 2", "gcp storage 2", "azure cosmos 3", "aws s3 5", "aws dynamodb 6", "aws kinesis 7", "aws sesv2 8",
		}},
		{"python/requirements.txt", []string{"gcp pubsub 2", "azure servicebus 3"}},
		{"js/handler.ts", []string{"aws sqs 1", "aws dynamodb 2", "gcp firestore 3", "azure storageblob 4"}},
		{"js/legacy.js", []string{"aws kinesis 2", "aws sns 5", "aws dynamodb 6"}}, // not SharedIniFileCredentials
		{"js/package.json", []string{"aws sqs 4", "aws dynamodb 5", "gcp firestore 6", "azure storageblob 7"}},
		{"cdk/stack.ts", []string{"aws sqs 1", "aws dynamodb 2", "aws lambda 3", "aws ec2 4"}},
		{"cdk/app.py", []string{"aws s3 3", "aws appsync 4", "aws cognito 6"}},
		{"go/main.go", []string{
			"gcp bigquery 4", "gcp computemetadata 5", "azure cosmos 6", "aws dynamodb 7", "aws s3 8", "aws sqs 9",
		}},
		{"java/App.java", []string{"aws dynamodb 3", "aws s3 4", "aws bedrockruntime 5"}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("../../testdata", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			got := short(File(tt.file, b))
			if want := sorted(tt.want); !slices.Equal(got, want) {
				t.Errorf("hits\n got %q\nwant %q", got, want)
			}
		})
	}
}

func TestSnippets(t *testing.T) {
	tests := []struct {
		name, file, src string
		want            []string
	}{
		{"terraform comment and other provider ignored", "main.tf",
			"# resource \"aws_s3_bucket\" \"x\" {}\nresource \"hcloud_server\" \"x\" {}\nresource \"random_id\" \"x\" {}\n", nil},
		{"terraform module from git", "main.tf",
			"module \"vpc\" {\n  source = \"git::https://github.com/terraform-aws-modules/terraform-aws-vpc.git?ref=v5.0.0\"\n}\n",
			[]string{"aws aws_vpc 2"}},
		{"terraform unknown module ignored", "main.tf",
			"module \"x\" {\n  source = \"terraform-aws-modules/something-new/aws\"\n}\n", nil},
		{"terraform body is kept for rules", "main.tf",
			"resource \"aws_rds_cluster\" \"a\" {\n  engine_mode = \"serverless\"\n}\n", []string{"aws aws_rds_cluster 1"}},
		{"cloudformation custom resources ignored", "t.yaml",
			"Resources:\n  X:\n    Type: Custom::Thing\n  Y:\n    Type: 'AWS::Lambda::Function'\n", []string{"aws AWS::Lambda::Function 5"}},
		{"cloudformation minified json", "t.json",
			`{"Resources":{"A":{"Type":"AWS::SQS::Queue"},"B":{"Type":"AWS::SNS::Topic"}}}`,
			[]string{"aws AWS::SNS::Topic 1", "aws AWS::SQS::Queue 1"}},
		{"cloudformation ResourceType is not a type", "t.json", `{"ResourceType": "AWS::S3::Bucket"}`, nil},
		{"serverless kinesis stream", "serverless.yml",
			"provider:\n  name: aws\nfunctions:\n  f:\n    events:\n      - stream:\n          type: kinesis\n          arn: x\n",
			[]string{"aws kinesis 6", "aws lambda 3"}},
		{"serverless short provider, google", "serverless.yaml",
			"provider: google\nfunctions:\n  f:\n    events:\n      - http: true\n", []string{"gcp functions 2"}},
		{"serverless unknown provider", "serverless.yml", "provider:\n  name: openwhisk\nfunctions:\n  f: {}\n", nil},
		{"python import list with comment", "a.py",
			"from google.cloud import (\n    storage,  # files\n    pubsub_v1 as ps,\n)\n",
			[]string{"gcp pubsubv1 3", "gcp storage 2"}},
		{"python boto3 needs boto in the file", "a.py", "s3 = session.client('s3')\n", nil},
		{"python vertexai and azure mgmt", "a.py", "import vertexai\nfrom azure.mgmt.compute import ComputeManagementClient\n",
			[]string{"azure mgmtcompute 2", "gcp vertexai 1"}},
		{"pyproject dependencies", "pyproject.toml",
			"dependencies = [\"google-cloud-bigquery[pandas]>=3\", \"azure-functions\"]\n",
			[]string{"azure functions 1", "gcp bigquery 1"}},
		{"aws-sdk v2 import list and clients path", "a.ts",
			"import { DynamoDB, S3 as S3v2, Credentials } from 'aws-sdk';\nimport SQS from 'aws-sdk/clients/sqs';\n",
			[]string{"aws dynamodb 1", "aws s3 1", "aws sqs 2"}},
		{"js @aws-sdk helpers", "a.js",
			"require('@aws-sdk/lib-storage'); require('@aws-sdk/credential-providers'); require('@aws-sdk/client-cognito-identity-provider')\n",
			[]string{"aws cognitoidentityprovider 1", "aws s3 1"}},
		{"js azure arm package", "a.js", "import { CosmosDBManagementClient } from \"@azure/arm-cosmosdb\";\n",
			[]string{"azure mgmtcosmosdb 1"}},
		{"go azure resourcemanager and versions", "a.go",
			"import (\n\t\"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos/v3\"\n\t\"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus\"\n)\n",
			[]string{"azure mgmtcosmos 2", "azure servicebus 3"}},
		{"go cdk and pubsub v2", "a.go",
			"import (\n\t\"github.com/aws/aws-cdk-go/awscdk/v2/awssqs\"\n\t\"cloud.google.com/go/pubsub/v2\"\n)\n",
			[]string{"aws sqs 2", "gcp pubsubv2 3"}},
		{"java v1 and cdk", "A.kt",
			"import com.amazonaws.services.dynamodbv2.AmazonDynamoDB\nimport software.amazon.awscdk.services.lambda.Function\nimport com.google.cloud.spanner.Spanner\n",
			[]string{"aws dynamodbv2 1", "aws lambda 2", "gcp spanner 3"}},
		{"files of other types are ignored", "README.md", "boto3.client('s3') resource \"aws_s3_bucket\" \"x\"", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := short(File(tt.file, []byte(tt.src)))
			if want := sorted(tt.want); !slices.Equal(got, want) {
				t.Errorf("hits\n got %q\nwant %q", got, want)
			}
		})
	}
}

func TestPathSkipsAndIgnores(t *testing.T) {
	res, err := Path("../../testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range res.Hits {
		if strings.HasPrefix(h.File, "ignored/") || strings.Contains(h.File, "node_modules") {
			t.Errorf("hit in a skipped path: %s:%d", h.File, h.Line)
		}
	}
	if res.FilesScanned != 17 {
		t.Errorf("files scanned = %d, want 17", res.FilesScanned)
	}
}

func TestPathLargeAndBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, b []byte) {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tf := []byte("resource \"aws_s3_bucket\" \"x\" {}\n")
	write("small.tf", tf)
	write("big.tf", append(tf, make([]byte, MaxFileSize)...))
	write("bin.tf", append([]byte{0, 1, 2}, tf...))
	write("build/out.tf", tf)
	write("sub/.terraform/modules/m.tf", tf)
	write("vendor/x.go", []byte(`import "github.com/aws/aws-sdk-go-v2/service/sqs"`))
	write("web/node_modules/aws-sdk/index.js", []byte(`require("@aws-sdk/client-appsync")`))

	res, err := Path(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := short(res.Hits); !slices.Equal(got, []string{"aws aws_s3_bucket 1"}) {
		t.Errorf("hits = %q, want only small.tf", got)
	}
	if res.FilesSkipped != 1 {
		t.Errorf("skipped = %d, want 1", res.FilesSkipped)
	}

	// A single file can be scanned too.
	one, err := Path(filepath.Join(dir, "small.tf"))
	if err != nil || len(one.Hits) != 1 || one.Hits[0].File != "small.tf" {
		t.Errorf("single file: %+v, %v", one, err)
	}
}

func TestLineIndex(t *testing.T) {
	l := newLineIndex("a\nbc\n\nd")
	for off, want := range map[int]int{0: 1, 1: 1, 2: 2, 4: 2, 5: 3, 6: 4} {
		if got := l.line(off); got != want {
			t.Errorf("line(%d) = %d, want %d", off, got, want)
		}
	}
}
