# Development Guidelines for unfuck-ai-comments

## Build & Test Commands
- Build: `go build ./...`
- Run tests: `go test ./...`
- Run specific test: `go test -run TestName ./path/to/package`
- Run tests with coverage: `go test -cover ./...`
- Run linting: `golangci-lint run ./...`
- Format code: `gofmt -s -w .`

## Code Style Guidelines
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use snake_case for filenames, camelCase for variables, PascalCase for exported names
- Group imports: standard library, then third-party, then local packages
- Error handling: check errors immediately and return them with context
- Use meaningful variable names; avoid single-letter names except in loops
- Validate function parameters at the start before processing
- Return early when possible to avoid deep nesting
- Prefer composition over inheritance
- Keep functions small and focused on a single responsibility
- Comment style: in-function comments should be lowercase sentences