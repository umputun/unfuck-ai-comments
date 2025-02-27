# unfuck-ai-comments [![build](https://github.com/umputun/unfuck-ai-comments/actions/workflows/ci.yml/badge.svg)](https://github.com/umputun/unfuck-ai-comments/actions/workflows/ci.yml)&nbsp;[![Coverage Status](https://coveralls.io/repos/github/umputun/unfuck-ai-comments/badge.svg?branch=master)](https://coveralls.io/github/umputun/unfuck-ai-comments?branch=master)

A simple CLI tool that converts all comments inside Go functions to lowercase while preserving comments for packages and function definitions. This makes comments in code consistent and easier to read.

## Motivation

Modern AI coding assistants like GitHub Copilot, Claude, and ChatGPT have become invaluable tools for many developers. However, they often generate code with inconsistent comment styling, including:

- ALL UPPERCASE comments for emphasis
- Initial Uppercase Words for pseudo-titles
- Mixed Case comments that Don't Follow conventions

For example:

```go
func ProcessData(data []byte) error {
    // CHECK FOR EMPTY INPUT
    if len(data) == 0 {
        return errors.New("empty data")
    }
    
    // Important: Validate Data Before Processing
    if !isValid(data) {
        return errors.New("invalid data format")
    }
    
    // This Function handles Multiple Formats
    switch determineFormat(data) {
        // Process Each Format differently
        // ...
    }
}
```

This inconsistent capitalization clashes with Go's conventional style where in-function comments typically use lowercase sentences. While these AI-generated comments are helpful in the moment, they create visual noise and inconsistency in codebases.

This tool automatically normalizes comments to match standard Go code conventions, ensuring your codebase maintains a consistent style regardless of whether the code was written by a human or generated by AI.

The tool is smart enough to preserve proper documentation comments (package comments and function documentation) while only modifying comments inside function bodies.

## Installation

```
go install github.com/umputun/unfuck-ai-comments@latest
```

## Usage

The tool now uses subcommands for different operations:

```
unfuck-ai-comments [options] <command> [file-patterns...]
```

Available commands:
- `run`: Process files in place (default)
- `diff`: Show diff without modifying files
- `print`: Print processed content to stdout

Process all .go files in the current directory:
```
unfuck-ai-comments run
```

Process a specific file:
```
unfuck-ai-comments run file.go
```

Process all .go files recursively:
```
unfuck-ai-comments run ./...
```

Show what would be changed without modifying files:
```
unfuck-ai-comments diff ./...
```

## Options

- `--dry`: Don't modify files, just show what would be changed (shortcut for diff command)
- `--title`: Convert only the first character to lowercase, keep the rest unchanged
- `--help` or `-h`: Show usage information

## Examples

Show diff for all Go files in the current directory:
```
unfuck-ai-comments diff .
```

Print the modified content to stdout:
```
unfuck-ai-comments print file.go
```

Process all Go files recursively and modify them:
```
unfuck-ai-comments run ./...
```

Use title case (only lowercase first character) on all Go files:
```
unfuck-ai-comments --title run ./...
```

## How it works

The tool uses Go's AST parser to identify comments that are inside functions, while leaving package comments, function documentation, and other structural comments untouched.

Only comments inside function bodies are modified to be lowercase.