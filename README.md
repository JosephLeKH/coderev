# coderev

AI-powered code review via AWS Bedrock. Run it before you push — it reviews your staged git diff and flags bugs, security issues, and style problems using Claude.

```
$ git add handler.go
$ coderev review

Reviewing 1 file(s) with model us.anthropic.claude-3-5-haiku-20241022-v1:0...

▶ handler.go
[BUG] L42 nil pointer dereference: user.Profile can be nil here
[SECURITY] L87 SQL query built with string concatenation — use parameterized queries

2 issue(s) found across 1 file(s)
```

## Prerequisites

- Go 1.21+
- AWS account with Bedrock access
- AWS credentials configured (see below)

## Installation

```bash
git clone https://github.com/JosephLeKH/coderev
cd coderev
go build -o coderev .
sudo mv coderev /usr/local/bin/
```

## AWS Setup

coderev calls AWS Bedrock. You need:

1. **IAM user** with the `AmazonBedrockFullAccess` policy
2. **Access key** for that user
3. **`aws configure`** to store the credentials locally

```bash
aws configure
# AWS Access Key ID: AKIA...
# AWS Secret Access Key: ...
# Default region name: us-east-1
# Default output format: json
```

No credentials are ever stored in the code or config file.

## Usage

```bash
# Review staged changes
coderev review

# Review changes since a specific commit or branch
coderev review --target HEAD~1
coderev review --target main

# Output as JSON (for CI or scripting)
coderev review --json

# Override AWS region
coderev review --region us-west-2

# Test without AWS credentials (mock response)
coderev review --mock
```

## Configuration

Create `.coderev.yaml` in your repo root (optional):

```yaml
model: us.anthropic.claude-3-5-haiku-20241022-v1:0

focus:
  - bugs
  - security
  - performance

ignore:
  - "*.lock"
  - "vendor/**"
  - "*_test.go"

language_hints:
  .go: "Follow standard Go error handling patterns."
  .ts: "Prefer strict TypeScript types. Avoid 'any'."
```

See `.coderev.yaml.example` for the full reference.

## Output Format

Each finding is printed as:

```
[SEVERITY] L<line> <description>
```

Severities: `BUG`, `SECURITY`, `PERFORMANCE`, `STYLE`, `NITPICK`

With `--json`, output is a JSON array:

```json
[
  {
    "file": "handler.go",
    "comments": [
      { "file": "handler.go", "severity": "BUG", "line": 42, "message": "nil pointer dereference" }
    ]
  }
]
```
