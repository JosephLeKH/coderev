# coderev

[![Release](https://img.shields.io/github/v/release/JosephLeKH/coderev)](https://github.com/JosephLeKH/coderev/releases/latest)

AI-powered code review via AWS Bedrock. Run it before you push — it reviews your staged git diff and flags bugs, security issues, and performance problems using Claude.

```
$ git add handler.go
$ coderev review

Reviewing 1 file(s) with model us.anthropic.claude-3-5-haiku-20241022-v1:0...

▶ handler.go
[BUG] L42 nil pointer dereference: user.Profile can be nil here
[SECURITY] L87 SQL query built with string concatenation — use parameterized queries

2 issue(s) found across 1 file(s) · 1 bug · 1 security
```

## Installation

### Option 1: `go install` (requires Go 1.21+)

```bash
go install github.com/JosephLeKH/coderev@latest
```

### Option 2: Pre-built binary

Download the latest binary for your platform from the [Releases page](https://github.com/JosephLeKH/coderev/releases/latest), extract it, and move it to your `$PATH`:

```bash
# Example for macOS arm64
curl -L https://github.com/JosephLeKH/coderev/releases/latest/download/coderev_<version>_darwin_arm64.tar.gz | tar xz
sudo mv coderev /usr/local/bin/
```

### Option 3: Build from source

```bash
git clone https://github.com/JosephLeKH/coderev
cd coderev
go build -o coderev .
sudo mv coderev /usr/local/bin/
```

## Prerequisites

- AWS account with Bedrock access (and model access enabled for Claude)
- AWS credentials configured (see below)

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

# Post inline comments to the open GitHub PR for the current branch
coderev review --post

# Test without AWS credentials (mock response)
coderev review --mock
```

### Posting to GitHub PRs (`--post`)

The `--post` flag posts inline review comments directly on the open pull request for your current branch.

Requirements:
- `GITHUB_TOKEN` env var must be set (a personal access token with `repo` scope)
- You must be on a branch with an open PR

```bash
export GITHUB_TOKEN=ghp_...
coderev review --target main --post
```

If no open PR is found for the current branch, `--post` is silently skipped.

### Exit Codes

- **0** — no issues found (clean review)
- **1** — one or more issues found, or a fatal error occurred

This lets you optionally gate on findings in git hooks or scripts:

```bash
# In .git/hooks/pre-push
coderev review --target origin/main || exit 1
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

Severities: `BUG`, `SECURITY`, `PERFORMANCE`

The summary line shows a per-severity breakdown:

```
2 issue(s) found across 1 file(s) · 1 bug · 1 security
```

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
