# Contributing to JazzPetri Engine

Thank you for your interest in contributing. This document explains how to report bugs, propose changes, and submit code.

## Reporting Bugs

Open a [GitHub issue](https://github.com/jazzpetri/engine/issues) and include:

- Go version (`go version`)
- A minimal, self-contained reproduction case
- Expected behavior and actual behavior
- Any relevant error output or stack traces

Search existing issues before opening a new one — the bug may already be tracked.

## Proposing Changes

For non-trivial changes, open an issue first to discuss the design before writing code. This avoids situations where significant work is invested in a direction that doesn't align with the project's goals.

For small fixes (typos, obvious bugs, documentation), a pull request without a prior issue is fine.

## Submitting Pull Requests

1. Fork the repository and create a branch from `main`:
   ```bash
   git checkout -b fix/describe-your-change
   ```

2. Make your changes following the code style guidelines below.

3. Add or update tests. All new behavior requires test coverage.

4. Ensure the full test suite passes:
   ```bash
   go test ./... -race
   ```

5. Ensure there are no linting issues:
   ```bash
   go vet ./...
   golangci-lint run
   ```

6. Push your branch and open a pull request against `main`. Fill in the PR template — describe what changed and why.

## Code Style

- Format with `gofmt`. No exceptions. CI will fail on unformatted code.
- Run `go vet ./...` before committing. Fix all warnings.
- Run `golangci-lint run` and resolve reported issues.
- Follow standard Go naming conventions: `MixedCaps` for exported names, short names for local variables.
- All exported types, functions, and methods require godoc comments.
- Keep functions small and focused. Prefer many small functions over one large one.
- The module has zero external dependencies. Do not add any — not even `golang.org/x/...`. If you believe a dependency is genuinely necessary, open an issue to discuss it first.

## Testing Requirements

- Target 80% or higher code coverage per package.
- Use `VirtualClock` for all time-dependent tests — never `time.Sleep` in tests.
- Include concurrency tests using the `-race` flag.
- Add `Example*` test functions for public APIs where they aid understanding.
- Test files live alongside source files (`foo_test.go` next to `foo.go`), except for integration tests which may use a separate `_test` package.

Run tests with coverage:

```bash
go test ./... -race -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Commit Message Format

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short description>

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `chore`

Examples:

```
feat(engine): add WaitWithTimeout to Engine
fix(petri): prevent panic when Resolve is called twice
docs(readme): add conditional routing example
test(verification): add mutual exclusion property test
```

Keep the subject line under 72 characters. Use the body to explain _why_, not _what_.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). By participating you agree to abide by its terms. Report unacceptable behavior to conduct@jazzpetri.dev.
