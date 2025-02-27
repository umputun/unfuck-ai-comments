# unfuck-ai-comments

A simple CLI tool that converts all comments inside Go functions to lowercase while preserving comments for packages and function definitions. This makes comments in code consistent and easier to read.

## Installation

```
go install github.com/umputun/unai-comments@latest
```

## Usage

Process all .go files in the current directory:
```
unfuck-ai-comments
```

Process a specific file:
```
unfuck-ai-comments file.go
```

Process all .go files recursively:
```
unfuck-ai-comments ./...
```

Show what would be changed without modifying files:
```
unfuck-ai-comments -dry-run ./...
```

## Options

- `-output=inplace`: Modify files in place (default)
- `-output=print`: Print modified file to stdout
- `-output=diff`: Show diff output
- `-dry-run`: Same as `-output=diff`
- `-help`: Show usage information

## Examples

Show diff for all Go files in the current directory:
```
unfuck-ai-comments -output=diff
```

Process all Go files recursively and modify them:
```
unfuck-ai-comments ./...
```

## How it works

The tool uses Go's AST parser to identify comments that are inside functions, while leaving package comments, function documentation, and other structural comments untouched.

Only comments inside function bodies are modified to be lowercase.