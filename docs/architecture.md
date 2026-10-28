# How it works

The scanner is one Go binary with no dependencies outside the standard library. It walks a directory, matches text
in each file it understands, maps every match to a rule, and prints a report. It never calls a cloud API and never
opens a network connection.

```
   path ──▶ scan.Path ──────────────▶ []scan.Hit ──▶ report.Build ──▶ report.Report ──▶ Text / JSON / Markdown / Badge
             │  walk, skip dirs,          cloud, kind,       │  rules.Set.Match
             │  .lockinignore, 1 MB       name, file:line    │  (embedded rules.json)
             ▼                                               ▼
     detectors by file type                         one finding per distinct service,
     terraform · cloudformation · serverless        score = round(100 × sum / services)
     python · javascript · go · java
```

## Packages

| Package | Job |
|---|---|
| `cmd/lockin` | Flags, output files, exit codes. Flags may come after the path (`permute`). |
| `internal/scan` | Walks the tree, picks detectors by file name, and returns hits: cloud, kind, name, file, line. |
| `internal/ignore` | Parses `.lockinignore` (gitignore-style globs, `!` negation). |
| `internal/rules` | The service catalogue (`rules.json`, embedded at build time), validation and matching. |
| `internal/report` | Groups hits into findings, calculates the score, renders text, JSON, Markdown and the badge. |

## Walking

- Starts at the given path. A single file can be scanned too.
- Never enters `.git`, `.hg`, `.svn`, `vendor`, `node_modules`, `dist`, `build`, `.terraform`, `.terragrunt-cache`,
  `cdk.out`, `.serverless`, `.aws-sam`, `.venv`, `venv` or `__pycache__`, at any depth. `cdk.out`, `.serverless` and
  `.aws-sam` hold generated CloudFormation, which would count everything twice.
- Reads `.lockinignore` from the scan root only. A path below an ignored directory stays ignored.
- Skips files over 1 MB (counted as skipped in the report), binary files (a NUL byte in the first 8 KB), and lock
  files (`package-lock.json`, `pnpm-lock.yaml`).
- Only files a detector understands are read.

## Detectors

Every detector is a set of regular expressions over the file text. No file is parsed as YAML, HCL or source code,
so templates with CloudFormation short-form tags, partial files and syntax errors are all fine.

| Detector | Files | What becomes a hit |
|---|---|---|
| `terraform` | `*.tf` | `resource "TYPE"` and `data "TYPE"` where TYPE starts with `aws_`, `google_`, `azurerm_`, `azuread_`, `azapi_` or `digitalocean_`. The block body (up to the next top-level block) is kept for rules with a body condition. `module` sources from `terraform-aws-modules` and `terraform-google-modules` map to their main resource type. |
| `cloudFormation` | `*.yaml`, `*.yml`, `*.json`, `*.template` that contain `AWS::` | `Type: AWS::X::Y` in YAML or JSON, quoted or not. `Custom::` types are ignored. |
| `serverless` | `serverless.yml`, `serverless.yaml` | `functions:` (as Lambda, Google Cloud Functions or Azure Functions, from `provider`) and, for AWS, each `- event:` under functions. A `stream` event is DynamoDB unless the next lines mention Kinesis. The `resources:` section is handled by the CloudFormation detector. |
| `python` | `*.py` | boto3 `client()` / `resource()` (only in files that mention `boto`), `google.cloud` imports, `vertexai`, `azure.*` imports, CDK `aws_cdk.aws_*`. |
| `pythonManifest` | `requirements*.txt`, `pyproject.toml`, `Pipfile`, `setup.cfg`, `setup.py` | `google-cloud-*` and `azure-*` package names. |
| `javascript` | `*.js`, `*.ts` and variants, `package.json` | `@aws-sdk/client-*` and helpers, `aws-sdk` v2 constructors and imports, CDK, `@google-cloud/*`, `@azure/*` (`arm-*` is the management SDK). |
| `golang` | `*.go` | AWS SDK v1 and v2 service packages, `cloud.google.com/go/*`, `azure-sdk-for-go/sdk/*`, CDK for Go. `go.mod` is not read: it lists indirect dependencies. |
| `java` | `*.java`, `*.kt`, `*.kts`, `*.scala` | AWS SDK v1 and v2, CDK for Java, `com.google.cloud.*`. |

SDK and CDK names are normalised before matching: lower case, letters and digits only. So boto3 `cognito-idp`,
`@aws-sdk/client-cognito-identity-provider` and Go `cognitoidentityprovider` all become simple strings a rule can
match with `cognito*`. For Azure Go packages the `az` prefix is dropped (`azcosmos` → `cosmos`), and management SDKs
become `mgmt<service>` in every language (`azure-mgmt-compute`, `@azure/arm-compute`, `resourcemanager/compute`).

## From hits to findings

1. Each hit is matched against the rules of its cloud and kind (see [rules.md](rules.md) for precedence).
2. Hits with the same rule form one finding. Locations are de-duplicated by file and line.
3. A hit no rule matches becomes its own finding, "Unknown service: NAME", class moderate.
4. Findings are sorted hard, moderate, portable; then by cloud and service. Networking & IAM comes last.
5. Score: `round(100 × (portable + 0.5 × moderate) / findings)`, halves up. No findings means no score.

## Outputs

| Output | Contents |
|---|---|
| text | Headline, a table with the EU replacement and up to 5 locations per finding, the formula, one help link. |
| JSON | Everything, including all locations, the matched names, the rule id and the EU Cloud Map capability id. |
| Markdown | The same table as text, for `$GITHUB_STEP_SUMMARY` or a pull request comment. |
| badge | `{"schemaVersion":1,"label":"EU-portable","message":"82%","color":"green"}` for shields.io. |

## GitHub Action

`action.yml` is a composite action with two bash steps. The first picks a version (the `version` input, else
`github.action_ref` when it is a `vX.Y.Z` tag, else the latest release), downloads
`lockin_<version>_<os>_<arch>.tar.gz` and `checksums.txt` from the GitHub release, checks the SHA-256 and unpacks the
binary into `$RUNNER_TEMP`. The second runs the scan once with `--json`, `--markdown` and the optional `--badge` and
`--fail-under`, appends the Markdown to the job summary, sets the outputs, and exits with the scanner's exit code.
All inputs reach the scripts through environment variables, never by template expansion inside the script.
