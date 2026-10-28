# cloud-lock-in-scanner

**Which cloud services in this repository would be hard to move off AWS, Google Cloud, Azure or DigitalOcean?**
Run one command on a repository. It finds the cloud services your infrastructure code and application code use,
says how portable each one is, and names its replacement on EU-owned clouds (UpCloud, Scaleway, Hetzner, OVHcloud).

- **Static.** It reads text files: Terraform, CloudFormation / SAM, Serverless Framework, CDK, and SDK imports in
  Python, JavaScript / TypeScript, Go and Java. No cloud credentials, no cloud API calls, no network access.
- **One score.** Each distinct service is portable, moderate or hard. The score is the share of portable services,
  with moderate ones counting half. Put it in your README as a badge, or fail CI below a threshold.
- **Every finding has a place.** File and line for each use, and the EU replacement.
- **Open source**, Apache-2.0. Binary name `lockin`.

## Documentation

[How it works](docs/architecture.md) · [Rules and how to add one](docs/rules.md) · [Development](docs/development.md) ·
[Releasing](docs/releasing.md) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md)

## Install

With Go (version in `go.mod`):

```sh
go install github.com/moveinfra/cloud-lock-in-scanner/cmd/lockin@latest
```

Or download a binary for Linux, macOS or Windows (amd64, arm64) from the
[releases](https://github.com/moveinfra/cloud-lock-in-scanner/releases), check it against `checksums.txt`, and put
`lockin` on your `PATH`:

```sh
V=0.1.0; OS=linux; ARCH=amd64
curl -fsSLO https://github.com/moveinfra/cloud-lock-in-scanner/releases/download/v$V/lockin_${V}_${OS}_${ARCH}.tar.gz
curl -fsSLO https://github.com/moveinfra/cloud-lock-in-scanner/releases/download/v$V/checksums.txt
grep " lockin_${V}_${OS}_${ARCH}.tar.gz\$" checksums.txt | sha256sum -c -
tar -xzf lockin_${V}_${OS}_${ARCH}.tar.gz lockin
```

Or use the [GitHub Action](#github-action).

## Usage

```sh
lockin                      # scan the current directory, print a table
lockin path/to/repo
lockin --format markdown .  # for a pull request comment
lockin --format json --out report.json .
lockin --badge badge.json --fail-under 60 .
lockin --rules              # list the built-in rules
```

| Flag | Meaning |
|---|---|
| `--format text\|json\|markdown` | Output format. Default `text`. JSON lists every location; text and Markdown list the first 5 per service. |
| `--out FILE` | Write the report to a file instead of standard output. |
| `--json FILE`, `--markdown FILE` | Also write the JSON or Markdown report to a file (one scan, several outputs). |
| `--badge FILE` | Write a [shields.io endpoint](https://shields.io/badges/endpoint-badge) badge JSON. |
| `--fail-under N` | Exit with code 1 when the score is below `N`. No score (nothing found) never fails. |
| `--rules` | Print the rules and exit. |
| `--version` | Print the version and exit. |

Exit codes: `0` done, `1` score below `--fail-under`, `2` usage or file error. Flags may come before or after the path.

To leave paths out, add a `.lockinignore` file to the root of the scanned directory. It uses `.gitignore` syntax
(`*`, `?`, `**`, a leading `/` to anchor, a trailing `/` for directories, `!` to re-include):

```gitignore
examples/
/legacy/old-stack.yaml
**/*.generated.tf
```

`.git`, `vendor`, `node_modules`, `.terraform`, `dist`, `build`, `cdk.out`, `.serverless`, `.aws-sam` and Python
virtual environments are always skipped, as are files over 1 MB and binary files.

## Example

Terminal output for the Serverless Framework sample in this repository (`lockin testdata/serverless`):

```text
EU-portable: 30%   5 services: 0 portable, 3 moderate, 2 hard
Path testdata/serverless, 1 file scanned.

CLASS     CLOUD  SERVICE                  USES
hard      AWS    DynamoDB                    2
          EU: No direct equivalent. Redesign on PostgreSQL (JSONB), or run ScyllaDB, which has a DynamoDB-compatible API
          at: serverless.yml:20, serverless.yml:31
hard      AWS    Lambda                      1
          EU: Containers on Kubernetes, or Scaleway Serverless Functions / Containers; handlers and triggers need repackaging
          at: serverless.yml:8

moderate  AWS    API Gateway                 1
          EU: An ingress controller or API gateway on Kubernetes (Traefik, Kong, NGINX); authorizers and mappings need rewriting
          at: serverless.yml:12
moderate  AWS    EventBridge / Scheduler     1
          EU: NATS or RabbitMQ for events, Kubernetes CronJobs for schedules
          at: serverless.yml:26
moderate  AWS    SQS                         2
          EU: Scaleway Queues and Topics (SQS/SNS-compatible), or NATS / RabbitMQ on servers
          at: serverless.yml:18, serverless.yml:33

Score per distinct service: portable = 1, moderate = 0.5, hard = 0; score = round(100 × sum / services).
Static text matching: no cloud access, dynamic usage can be missed.
Help moving to an EU cloud: https://moveinfra.eu/
```

Markdown output for the Terraform samples (`lockin --format markdown testdata/terraform`), as it shows in a job
summary or pull request comment:

> ## EU-portable: 67%
>
> 23 cloud services in `testdata/terraform` (4 files scanned): **14 portable**, **3 moderate**, **6 hard**.
>
> | Class | Cloud | Service | Uses | EU replacement | Where |
> |---|---|---|---:|---|---|
> | hard | AWS | Aurora Serverless / RDS Data API | 1 | A fixed-size managed PostgreSQL / MySQL (UpCloud, Scaleway, OVHcloud); Data API calls need a normal driver and connection pool | `aws.tf:37` |
> | hard | AWS | DynamoDB | 1 | No direct equivalent. Redesign on PostgreSQL (JSONB), or run ScyllaDB, which has a DynamoDB-compatible API | `aws.tf:47` |
> | hard | AWS | Lambda | 1 | Containers on Kubernetes, or Scaleway Serverless Functions / Containers; handlers and triggers need repackaging | `aws.tf:52` |
> | hard | Google Cloud | BigQuery | 1 | No direct equivalent. A warehouse you run (e.g. ClickHouse); SQL dialect and loaders need porting | `gcp.tf:10` |
> | hard | Azure | Azure Functions | 1 | Containers on Kubernetes, or Scaleway Serverless Functions / Containers; bindings and triggers need repackaging | `azure.tf:15` |
> | hard | Azure | Cosmos DB / Table Storage | 1 | No direct equivalent. PostgreSQL (JSONB), or MongoDB / Cassandra you run if you use those APIs | `azure.tf:11` |
> | moderate | AWS | SQS | 1 | Scaleway Queues and Topics (SQS/SNS-compatible), or NATS / RabbitMQ on servers | `aws.tf:57` |
> | moderate | AWS | Unknown service: aws_gamelift_fleet | 1 | Not in the rules yet. Check this one by hand, and consider adding a rule | `aws.tf:69` |
> | moderate | Google Cloud | Pub/Sub | 1 | NATS or RabbitMQ on servers, or OVHcloud Managed Kafka; subscriptions and push endpoints need porting | `gcp.tf:14` |
> | portable | AWS | EC2 / Lightsail | 1 | UpCloud Cloud Servers, Scaleway Instances, Hetzner Cloud Servers, OVHcloud Public Cloud Instances | `aws.tf:27` |
> | portable | AWS | EKS | 1 | UpCloud Managed Kubernetes, Scaleway Kubernetes Kapsule, OVHcloud Managed Kubernetes Service, or self-managed Kubernetes on Hetzner Cloud Servers | `aws.tf:74` |
> | portable | AWS | Elastic Load Balancing (ALB, NLB) | 1 | UpCloud Managed Load Balancer, Scaleway Load Balancer, Hetzner Load Balancer, OVHcloud Public Cloud Load Balancer | `aws.tf:65` |
> | portable | AWS | RDS / Aurora (provisioned) | 1 | UpCloud Managed PostgreSQL / MySQL, Scaleway Managed Database for PostgreSQL / MySQL, OVHcloud Managed Databases for PostgreSQL / MySQL, or PostgreSQL / MySQL on Hetzner Cloud Servers | `aws.tf:32` |
> | portable | AWS | S3 | 1 | UpCloud Managed Object Storage, Scaleway Object Storage, Hetzner Object Storage, OVHcloud Object Storage (S3 API) | `aws.tf:61` |
> | portable | Google Cloud | Cloud Storage | 1 | UpCloud Managed Object Storage, Scaleway Object Storage, Hetzner Object Storage, OVHcloud Object Storage (S3 API); code using the GCS client needs an S3 client | `gcp.tf:18` |
> | portable | Google Cloud | GKE | 1 | UpCloud Managed Kubernetes, Scaleway Kubernetes Kapsule, OVHcloud Managed Kubernetes Service, or self-managed Kubernetes on Hetzner Cloud Servers | `gcp.tf:5` |
> | portable | Azure | AKS | 1 | UpCloud Managed Kubernetes, Scaleway Kubernetes Kapsule, OVHcloud Managed Kubernetes Service, or self-managed Kubernetes on Hetzner Cloud Servers | `azure.tf:6` |
> | portable | DigitalOcean | Droplets | 1 | UpCloud Cloud Servers, Scaleway Instances, Hetzner Cloud Servers, OVHcloud Public Cloud Instances | `digitalocean.tf:1` |
> | portable | DigitalOcean | Managed Caching | 1 | UpCloud Managed Valkey, Scaleway Managed Redis, OVHcloud Managed Databases for Caching, or Valkey on Hetzner Cloud Servers | `digitalocean.tf:7` |
> | portable | DigitalOcean | Managed Databases | 1 | UpCloud Managed PostgreSQL / MySQL, Scaleway Managed Database for PostgreSQL / MySQL, OVHcloud Managed Databases for PostgreSQL / MySQL, or PostgreSQL / MySQL on Hetzner Cloud Servers | `digitalocean.tf:13` |
> | portable | AWS | Networking & IAM (5 types) | 5 | UpCloud SDN private networks, Scaleway VPC and Private Networks, Hetzner Networks and Firewalls, OVHcloud vRack private networks; IAM becomes API tokens and RBAC | `aws.tf:3`, `aws.tf:7`, `aws.tf:12`, `aws.tf:16`, `aws.tf:21` |
> | portable | Google Cloud | Networking & IAM | 1 | UpCloud SDN private networks, Scaleway VPC and Private Networks, Hetzner Networks and Firewalls, OVHcloud vRack private networks; IAM becomes API tokens and RBAC | `gcp.tf:1` |
> | portable | Azure | Networking & IAM | 1 | UpCloud SDN private networks, Scaleway VPC and Private Networks, Hetzner Networks and Firewalls, OVHcloud vRack private networks; IAM becomes API tokens and RBAC | `azure.tf:1` |
>
> <sub>[cloud-lock-in-scanner](https://github.com/moveinfra/cloud-lock-in-scanner) v0.1.0 · Help moving to an EU cloud: [moveinfra.eu](https://moveinfra.eu/)</sub>

(The real output also has a collapsed "How the score is calculated" section.) Running `lockin testdata` scans all
samples: 35 services, score 56%.

## What it detects

| Source | Files | Recognised |
|---|---|---|
| Terraform / OpenTofu | `*.tf` | `resource` and `data` blocks of the `aws`, `google`, `azurerm`, `azuread`, `azapi` and `digitalocean` providers; common `terraform-aws-modules` and `terraform-google-modules` modules |
| CloudFormation, SAM | `*.yaml`, `*.yml`, `*.json`, `*.template` | `Type: AWS::Service::Resource`, including `AWS::Serverless::*`. Short-form tags (`!Ref`, `!GetAtt`, `!Sub`) are fine: types are matched as text. |
| Serverless Framework | `serverless.yml`, `serverless.yaml` | `functions:` (Lambda, or Google / Azure functions for those providers) and events: `http`, `httpApi`, `websocket`, `sqs`, `sns`, `stream`, `s3`, `schedule`, `eventBridge`, `cognitoUserPool`, `alb`, `iot`, `msk` and more |
| AWS CDK | `*.ts`, `*.js`, `*.py`, `*.go`, `*.java` | `aws-cdk-lib/aws-*`, `@aws-cdk/aws-*`, `aws_cdk.aws_*`, `awscdk/v2/aws*`, `software.amazon.awscdk.services.*` |
| Python | `*.py`, `requirements*.txt`, `pyproject.toml`, `Pipfile`, `setup.cfg`, `setup.py` | `boto3.client('sqs')`, `.resource('dynamodb')`, `from google.cloud import bigquery`, `import vertexai`, `from azure.cosmos import …`, `google-cloud-*` and `azure-*` packages |
| JavaScript / TypeScript | `*.js`, `*.ts`, `*.mjs`, `*.cjs`, `*.jsx`, `*.tsx`, `package.json` | `@aws-sdk/client-*`, `@aws-sdk/lib-dynamodb`, `aws-sdk` v2 (`new AWS.SQS()`, `aws-sdk/clients/s3`), `@google-cloud/*`, `@azure/*` |
| Go | `*.go` | `github.com/aws/aws-sdk-go-v2/service/*`, `github.com/aws/aws-sdk-go/service/*`, `cloud.google.com/go/*`, `github.com/Azure/azure-sdk-for-go/sdk/*` |
| Java, Kotlin, Scala | `*.java`, `*.kt`, `*.kts`, `*.scala` | `software.amazon.awssdk.services.*`, `com.amazonaws.services.*`, `com.google.cloud.*` |

The [rules](internal/rules/rules.json) cover 106 services: AWS 43, Google Cloud 27, Azure 24, DigitalOcean 12.
A Terraform type, CloudFormation type or SDK name of a known cloud that no rule matches is still reported, as
"Unknown service: …" with class moderate, so nothing is dropped silently. Networking and IAM resources (VPCs,
subnets, security groups, routes, IAM roles and policies, resource groups) are portable infrastructure-as-code and
are reported as one "Networking & IAM" finding per cloud, so they do not dominate the report.

EU product names come from the [EU Cloud Map](https://github.com/moveinfra/eu-cloud-map), and each rule carries the
map's capability id (`nosql`, `queues`, …) where one exists. The interactive map is at https://moveinfra.eu/eu-cloud-map.

## Score

Each **distinct service** counts once, however often it is used. Weights:

| Class | Weight | Meaning | Examples |
|---|---:|---|---|
| portable | 1 | Standard technology with a direct EU equivalent | VMs, S3 API, managed PostgreSQL / MySQL, Kubernetes, load balancers, Redis, DNS |
| moderate | 0.5 | Needs a swap, but standard alternatives exist | SQS / SNS → NATS or RabbitMQ, ECS → Kubernetes, CloudFront → a CDN, Secrets Manager → OpenBao / Vault, Cognito → Keycloak |
| hard | 0 | Proprietary data model or runtime | DynamoDB, Lambda, Step Functions, Kinesis, Bedrock, Vertex AI, AppSync, Aurora Serverless, BigQuery, Firestore, Cosmos DB, Azure Functions, Logic Apps |

```
score = round(100 × (portable + 0.5 × moderate) / services)
```

Halves round up. With nothing cloud-specific found there is no score ("n/a"), and `--fail-under` does not fail.

The same service on two clouds (S3 and Cloud Storage) counts twice. All networking and IAM resources of one cloud
count as one portable service. The score says how much of what you use has a standard replacement; it does not say
how much work the move is. One hard service with a lot of data can be most of the work.

## GitHub Action

```yaml
name: Cloud lock-in
on:
  push:
    branches: [main]
  pull_request: {}

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - id: lockin
        uses: moveinfra/cloud-lock-in-scanner@v0.1.0
        with:
          path: .
          fail-under: 50                          # optional: fail below 50%
      - run: echo "EU-portable ${{ steps.lockin.outputs.score }}%"
```

The action downloads the release binary for the runner (Linux, macOS, Windows; x64, ARM64), checks its SHA-256
against the release's `checksums.txt`, runs the scan and writes the Markdown report to the job summary.

| Input | Default | Meaning |
|---|---|---|
| `path` | `.` | Directory to scan |
| `fail-under` | empty | Fail the step when the score is below this number |
| `badge` | empty | Write a shields.io endpoint badge JSON to this path |
| `version` | empty | Release to download. Empty: the tag the action is used at (`@v0.1.0`), or the latest release when used at a branch or commit |

| Output | Meaning |
|---|---|
| `score` | 0–100, empty when nothing was found |
| `report` | Path of the JSON report |
| `markdown` | Path of the Markdown report |

To comment on pull requests, add `permissions: pull-requests: write` and:

```yaml
      - if: github.event_name == 'pull_request'
        env:
          GH_TOKEN: ${{ github.token }}
        run: gh pr comment ${{ github.event.number }} --body-file "${{ steps.lockin.outputs.markdown }}"
```

### Badge

The badge is a [shields.io endpoint](https://shields.io/badges/endpoint-badge) JSON, for example
`{"schemaVersion":1,"label":"EU-portable","message":"82%","color":"green"}`. Colours: 90+ brightgreen, 75+ green,
60+ yellowgreen, 40+ yellow, 20+ orange, below 20 red, no score lightgrey.

Shields needs the JSON at a public URL. One way is to keep it on its own branch, updated from `main`
(the job needs `permissions: contents: write`):

```yaml
      - id: lockin
        uses: moveinfra/cloud-lock-in-scanner@v0.1.0
        with:
          badge: ${{ runner.temp }}/lockin-badge.json
      - name: Publish the badge
        if: github.ref == 'refs/heads/main'
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
          if git fetch origin badges; then git switch badges; else git switch --orphan badges; fi
          cp "$RUNNER_TEMP/lockin-badge.json" lockin.json
          git add lockin.json
          git diff --cached --quiet || { git commit -m "Update lock-in badge"; git push origin badges; }
```

Then in your README:

```markdown
![EU-portable](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/OWNER/REPO/badges/lockin.json)
```

Any other public place works too: GitHub Pages, a gist, or an object storage bucket.

## Limitations

- It is static text matching. It finds what is written in the repository, not what runs in your account. There is
  no cloud access, so resources created by hand, by another repository or by a deployment tool it does not read
  (Pulumi, ARM / Bicep, Deployment Manager, Helm charts with cloud annotations) are not seen.
- Dynamic usage can be missed: service names built at runtime, SDK clients created through a wrapper,
  `boto3.client(name)` with a variable, or Terraform modules other than the common registry ones.
- It can over-report: commented-out code and examples still match, and an imported SDK may be used for one call.
  Use `.lockinignore` for paths you do not want counted.
- Classes are judgement calls for a typical setup. A "portable" service can still be a lot of work (a 50 TB bucket),
  and a "hard" one can be small (one Lambda function).
- Aurora Serverless is recognised from Terraform settings (`engine_mode = "serverless"`,
  `serverlessv2_scaling_configuration`, `db.serverless`) and from the RDS Data API client; in CloudFormation it
  counts as RDS.

## Development

```sh
go test ./...
go vet ./...
go run ./cmd/lockin testdata
```

More in [docs/development.md](docs/development.md). Releases: push a tag `vX.Y.Z`; see [docs/releasing.md](docs/releasing.md).

---

Made by [MoveInfra](https://moveinfra.eu), which moves company infrastructure from AWS, Google Cloud and Azure to
EU-owned clouds at a fixed price. See also the [EU Cloud Map](https://github.com/moveinfra/eu-cloud-map) and the
[cloud exit cost analyzer](https://github.com/moveinfra/cloud-exit-cost-analyzer).
