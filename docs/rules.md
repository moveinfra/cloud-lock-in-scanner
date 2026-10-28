# Rules

All rules are in one file, [`internal/rules/rules.json`](../internal/rules/rules.json), embedded into the binary at
build time. `lockin --rules` lists them.

## A rule

```json
{
  "id": "aws.dynamodb",
  "cloud": "aws",
  "service": "DynamoDB",
  "class": "hard",
  "capability": "nosql",
  "replacement": "No direct equivalent. Redesign on PostgreSQL (JSONB), or run ScyllaDB, which has a DynamoDB-compatible API",
  "terraform": ["aws_dynamodb_*"],
  "cloudformation": ["AWS::DynamoDB::*", "AWS::Serverless::SimpleTable"],
  "sdk": ["dynamodb*", "dax"]
}
```

| Field | Required | Meaning |
|---|---|---|
| `id` | yes | `<cloud>.<short-name>`, unique. Shown in JSON output. |
| `cloud` | yes | `aws`, `gcp`, `azure` or `digitalocean`. |
| `service` | yes | Display name, as the provider calls it. Several related products can share a rule (`Kinesis / Firehose`). |
| `class` | yes | `portable`, `moderate` or `hard` (see below). |
| `capability` | no | Capability id from the [EU Cloud Map](https://github.com/moveinfra/eu-cloud-map/blob/main/data/eu-cloud-map.json), when the map has one: `compute`, `managed-postgres`, `object-storage`, `load-balancer`, `networking`, `kubernetes`, `redis`, `queues`, `functions`, `dns`, `nosql`, `warehouse`. |
| `replacement` | yes | What replaces it on EU-owned clouds, in one line, without a final full stop. Use the product names from the EU Cloud Map (for example "UpCloud Managed Valkey", "Scaleway Kubernetes Kapsule", "OVHcloud Object Storage (S3 API)"); do not invent product names. Where the map has nothing, name open-source software (Keycloak, OpenBao, NATS) or describe the approach. |
| `group` | no | `true` for glue resources (networking, IAM). Shown as one finding with a count of types. |
| `terraform` | * | Terraform resource and data source types. |
| `terraform_body` | no | Regular expressions. When set, the rule's Terraform patterns only apply to blocks whose body matches one of them, and the rule wins over rules without a body condition. Used for Aurora Serverless (`aws_rds_cluster` with `engine_mode = "serverless"`) and DigitalOcean Managed Caching (`digitalocean_database_cluster` with `engine = "valkey"`). |
| `cloudformation` | * | CloudFormation and SAM resource types. |
| `sdk` | * | SDK, CDK and Serverless Framework service names, after normalisation (below). |

\* At least one of `terraform`, `cloudformation` and `sdk`.

## Classes

Pick the class for a typical setup, not the worst case:

- **portable** (weight 1): standard technology with a direct EU equivalent. Moving it is mostly data transfer and
  Terraform. VMs, S3-compatible object storage, managed PostgreSQL / MySQL, Kubernetes, load balancers,
  Redis-compatible caches, DNS, OCI registries, Kafka, RabbitMQ, OpenSearch.
- **moderate** (weight 0.5): needs a swap, but standard alternatives exist and the application change is bounded.
  SQS / SNS → NATS or RabbitMQ, ECS → Kubernetes, CloudFront → a CDN, Secrets Manager → OpenBao or Vault,
  Cognito → Keycloak, Cloud Run → Knative-style containers, observability → Prometheus and OpenTelemetry.
- **hard** (weight 0): proprietary data model or runtime. The application or its data has to be redesigned.
  DynamoDB, Lambda, Step Functions, Kinesis, Bedrock, Vertex AI, AppSync, Aurora Serverless, BigQuery, Firestore,
  Cosmos DB, Azure Functions, Logic Apps.

If you are unsure, say why in the pull request.

## Patterns

- A pattern is an exact name or a glob with `*` (`aws_dynamodb_*`, `AWS::DynamoDB::*`, `cognito*`).
  A pattern that is only `*` is rejected.
- Patterns apply only to names of the rule's cloud.
- When several patterns match one name, the winner is, in order: a rule with a matching `terraform_body`; an exact
  pattern; the pattern with the most characters other than `*`. So `aws_route53_*` beats `aws_route*`, and
  `rdsdata` (Aurora Serverless) beats `rds*` (RDS). Order in the file does not matter.
- The same pattern in two rules of the same cloud is an error.

### SDK names

SDK names are normalised to lower-case letters and digits, and so are `sdk` patterns. What a detector reports:

| Source | Example | Name |
|---|---|---|
| boto3 | `boto3.client("cognito-idp")` | `cognitoidp` |
| JavaScript v3 | `@aws-sdk/client-cognito-identity-provider` | `cognitoidentityprovider` |
| JavaScript v2 | `new AWS.DynamoDB.DocumentClient()`, `aws-sdk/clients/sqs` | `dynamodb`, `sqs` |
| Go | `github.com/aws/aws-sdk-go-v2/service/sfn` | `sfn` |
| Java | `software.amazon.awssdk.services.dynamodb` | `dynamodb` |
| CDK | `aws-cdk-lib/aws-lambda-nodejs`, `aws_cdk.aws_s3` | `lambdanodejs`, `s3` |
| Serverless events | `- sqs:`, `- http:`, `- stream:` | `sqs`, `apigateway`, `dynamodbstreams` or `kinesis` |
| Google, Python | `from google.cloud import pubsub_v1` | `pubsubv1` |
| Google, JavaScript | `@google-cloud/secret-manager` | `secretmanager` |
| Google, Go | `cloud.google.com/go/bigquery/storage/apiv1` | `bigquerystorage` (first two path elements) |
| Azure, Python | `from azure.storage.blob import …`, `azure-mgmt-compute` | `storageblob`, `mgmtcompute` |
| Azure, JavaScript | `@azure/service-bus`, `@azure/arm-cosmosdb` | `servicebus`, `mgmtcosmosdb` |
| Azure, Go | `sdk/data/azcosmos`, `sdk/resourcemanager/compute/armcompute` | `cosmos`, `mgmtcompute` |

Because the same service is spelled differently across SDKs, most `sdk` patterns end with `*` and some rules list
aliases (`["sfn", "stepfunctions*"]`).

## Adding or changing a rule

1. Find the names: the Terraform registry page of the resource, the CloudFormation resource reference, and the SDK
   package names in each language.
2. Add or edit the rule in `internal/rules/rules.json`. Keep rules of one cloud together, glue rule first.
3. Add a line to the table in `TestMatch` in `internal/rules/rules_test.go` for each new name, especially where a
   longer pattern has to win over a shorter one.
4. If the rule needs a new kind of reference in code (a new SDK, a new file type), add the detector to
   `internal/scan`, a fixture under `testdata/` or a case in `TestSnippets`, and describe it in
   [architecture.md](architecture.md).
5. Run `go test ./...`. If the fixture score changes, update `testdataScore` in `cmd/lockin/main_test.go` and the
   examples in the README (`go run ./cmd/lockin testdata/serverless`, `--format markdown testdata/terraform`).
6. In the pull request, link the provider documentation and say why you chose the class.

`rules.Parse` rejects unknown clouds, classes and capability ids, missing fields, duplicate ids and duplicate
patterns, so most mistakes fail the tests.
