# Dependency Matrix CLI

Generate interactive dependency matrices for GitLab repositories with multi-language support.

[![codecov](https://codecov.io/gh/smirnoffmg/di-matrix-cli/graph/badge.svg?token=QODUOL3T3V)](https://codecov.io/gh/smirnoffmg/di-matrix-cli)

## Overview

A CLI tool that analyzes dependencies across GitLab repositories and generates interactive HTML reports. Supports monorepos and multiple programming languages with concurrent processing and comprehensive error handling.

## Supported Languages

| Language | Supported Files                                                     | Parser Source                            |
| -------- | ------------------------------------------------------------------- | ---------------------------------------- |
| Go       | `go.mod`, `go.sum`                                                  | `trivy/pkg/dependency/parser/golang/mod` |
| Java     | `pom.xml`, `build.gradle`, `gradle.lockfile`                        | `trivy/pkg/dependency/parser/java`       |
| Node.js  | `package.json`, `package-lock.json`, `yarn.lock`                    | `trivy/pkg/dependency/parser/nodejs`     |
| Python   | `requirements.txt`, `Pipfile`, `poetry.lock`, `uv.lock`, `setup.py` | `trivy/pkg/dependency/parser/python`     |

## Features

- GitLab API integration for repository access
- Multi-language dependency parsing with recursive monorepo discovery
- Interactive HTML matrix with frozen headers and repository links
- Internal vs external dependency classification
- Concurrent processing with worker pools
- Runtime configuration via Docker volumes and environment variables
- Debug logging with API call tracking and performance metrics

## Recent Changes

### Interface Improvements
- Frozen table headers for large matrices
- Latest version display in column headers
- Clickable repository links to GitLab
- Project path display and intelligent sorting

### Docker Runtime Configuration
- Runtime config mounting without rebuilding
- Environment variable overrides for all settings
- Report persistence via volume mounting
- Non-root user execution

### Performance & Reliability
- Concurrent processing for repository operations
- Eliminated race conditions in dependency classification
- Comprehensive error handling and debug logging
- Go 1.25.1 compatibility and golangci-lint integration

## Docker Usage

### Quick Start

```bash
# Build image
docker build -t di-matrix-cli:latest .

# Run with mounted config (recommended)
docker run --rm \
  -v $(pwd)/config.yaml:/app/config/config.yaml \
  di-matrix-cli:latest -l nodejs

# Legacy config path
docker run --rm \
  -v $(pwd)/config.yaml:/app/config.yaml \
  di-matrix-cli:latest analyze --config /app/config.yaml -l nodejs
```

### Make Commands

```bash
make all             # Run all checks (tidy, fmt, lint, coverage, integration-test)
make fmt             # Format code
make lint            # Run linter
make test            # Run tests with race detection
make coverage        # Generate coverage report
make integration-test # Run integration tests
make run             # Run application locally
```

### Environment Variables

```bash
docker run --rm \
  -e GITLAB_TOKEN=your_token \
  -e GITLAB_BASE_URL=https://gitlab.example.com \
  -e OUTPUT_TITLE="Custom Report" \
  -e ANALYSIS_TIMEOUT_MINUTES=15 \
  -v $(pwd)/config.yaml:/app/config/config.yaml \
  di-matrix-cli:latest -l nodejs
```

**Supported Variables:**
- `GITLAB_BASE_URL` - GitLab instance URL (default: https://gitlab.com)
- `GITLAB_TOKEN` - GitLab access token
- `OUTPUT_HTML_FILE` - Output HTML file path (default: dependency-matrix.html)
- `OUTPUT_TITLE` - Report title (default: Dependency Matrix Report)
- `ANALYSIS_TIMEOUT_MINUTES` - Analysis timeout in minutes (default: 10)

## Usage

### Basic Analysis

```bash
# Analyze by language
docker run --rm -v $(pwd)/config.yaml:/app/config/config.yaml di-matrix-cli:latest -l nodejs
docker run --rm -v $(pwd)/config.yaml:/app/config/config.yaml di-matrix-cli:latest -l go
docker run --rm -v $(pwd)/config.yaml:/app/config/config.yaml di-matrix-cli:latest -l python
```

### Environment Configuration

```bash
# Override settings with environment variables
docker run --rm \
  -v $(pwd)/config.yaml:/app/config/config.yaml \
  --env-file .env \
  -e OUTPUT_TITLE="Organization Dependencies" \
  -e ANALYSIS_TIMEOUT_MINUTES=15 \
  di-matrix-cli:latest -l nodejs
```

### Output Persistence

```bash
# Save report to host filesystem
docker run --rm \
  -v $(pwd)/config.yaml:/app/config/config.yaml \
  -v $(pwd)/reports:/app/output \
  -e OUTPUT_HTML_FILE="/app/output/dependency-report.html" \
  di-matrix-cli:latest -l nodejs
```

### Complete Example

```bash
docker run --rm \
  -v $(pwd)/config.yaml:/app/config/config.yaml \
  -v $(pwd)/reports:/app/output \
  --env-file .env \
  -e OUTPUT_HTML_FILE="/app/output/$(date +%Y%m%d)-dependencies.html" \
  -e OUTPUT_TITLE="Daily Dependency Analysis" \
  -e ANALYSIS_TIMEOUT_MINUTES=20 \
  di-matrix-cli:latest \
  -l nodejs \
  --debug
```

## Configuration

### Configuration File

Create `config.yaml`:

```yaml
gitlab:
  base_url: "https://gitlab.com"
  token: "your-gitlab-token-here"

repositories:
  - url: "https://gitlab.com/your-group/your-repo"
    branch: "main"

internal:
  domains:
    - "gitlab.company.com/group"
    - "github.com/company"
  patterns:
    - "@company/"
    - "com.company."
    - "company-"

output:
  html_file: "dependency-matrix.html"
  title: "Organization Dependency Matrix"

timeout:
  analysis_timeout_minutes: 10
```

### Environment Variables File

Create `.env`:

```bash
GITLAB_TOKEN=glpat-your-token-here
GITLAB_BASE_URL=https://gitlab.com
OUTPUT_TITLE="Custom Report Title"
ANALYSIS_TIMEOUT_MINUTES=15
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: Run dependency analysis
  run: |
    docker run --rm \
      -e GITLAB_TOKEN=${{ secrets.GITLAB_TOKEN }} \
      -v $(pwd)/config.yaml:/app/config/config.yaml \
      -v $(pwd)/reports:/app/output \
      -e OUTPUT_HTML_FILE="/app/output/dependency-report.html" \
      di-matrix-cli:latest -l nodejs
```
