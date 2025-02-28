# Development Guidelines for unfuck-ai-comments

## Build & Test Commands
- Build: `go build ./...`
- Run tests: `go test ./...`
- Run specific test: `go test -run TestName ./path/to/package`
- Run tests with coverage: `go test -cover ./...`
- Run linting: `golangci-lint run ./...`
- Format code: `gofmt -s -w .`
- Process comments in title case mode: `unfuck-ai-comments run --title main.go main_test.go`
- On completion, run: formating, tests amd comments processing

## Code Style Guidelines
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use snake_case for filenames, camelCase for variables, PascalCase for exported names
- Group imports: standard library, then third-party, then local packages
- Error handling: check errors immediately and return them with context
- Use meaningful variable names; avoid single-letter names except in loops
- Validate function parameters at the start before processing
- Return early when possible to avoid deep nesting
- Prefer composition over inheritance
- Function size preferences:
  - Aim for functions around 50-60 lines when possible
  - Don't break down functions too small as it can reduce readability
  - Maintain focus on a single responsibility per function
- Comment style: in-function comments should be lowercase sentences
- Code width: keep lines under 130 characters when possible