# Code Formatting and Linting Guide

This document provides guidelines for maintaining consistent code formatting and linting in the Binance Trading Bot project.

## Tools

The project uses the following tools for code quality:

1. **gofmt** - The standard Go code formatter
2. **golangci-lint** - A fast, parallel runner for Go linters

## Setup

To set up the linting tools, run:

```bash
make tools
```

This will install golangci-lint. The gofmt tool is included with the Go installation.

## Configuration

The linting rules are configured in the `.golangci.yml` file in the root of the project. This configuration:

- Enables a set of useful linters
- Sets reasonable thresholds for various linting rules
- Excludes specific files and functions from certain linting rules where appropriate

## Usage

### Formatting Code

To format all Go code in the project:

```bash
make fmt
```

This runs `gofmt -w -s .` which formats all Go files and simplifies code where possible.

### Linting Code

To run all linters:

```bash
make lint
```

This runs golangci-lint with the configuration from `.golangci.yml`.

### Combined Workflow

To format, lint, and build the code:

```bash
make all
```

## Linting in CI/CD

It's recommended to add linting checks to your CI/CD pipeline. Here's an example GitHub Actions workflow step:

```yaml
- name: Lint
  run: make lint
```

## Ignoring Linting Rules

In some cases, it may be necessary to ignore specific linting rules. This can be done in two ways:

1. **File-specific exclusions** in `.golangci.yml`:

```yaml
issues:
  exclude-rules:
    - path: path/to/file.go
      linters:
        - linter1
        - linter2
```

2. **Line-specific exclusions** in code:

```go
 
func someFunction() {
    // Code that would trigger linting errors
}
```

Only use these exclusions when absolutely necessary and always include a comment explaining why the rule is being ignored.

## Best Practices

1. **Run linting before committing** code to ensure consistent quality
2. **Fix linting issues** rather than ignoring them when possible
3. **Keep the configuration up to date** as the project evolves
4. **Document any custom rules** or exceptions in this file

## Additional Resources

- [golangci-lint documentation](https://golangci-lint.run/)
- [Effective Go - Formatting](https://golang.org/doc/effective_go#formatting)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)