# unfuck-ai-comments [![build](https://github.com/umputun/unfuck-ai-comments/actions/workflows/ci.yml/badge.svg)](https://github.com/umputun/unfuck-ai-comments/actions/workflows/ci.yml)&nbsp;[![Coverage Status](https://coveralls.io/repos/github/umputun/unfuck-ai-comments/badge.svg?branch=master)](https://coveralls.io/github/umputun/unfuck-ai-comments?branch=master)

A simple CLI tool that converts all comments inside Go functions and structs to lowercase while preserving comments for packages and function definitions, as well as special indicator comments like TODO and FIXME. This makes comments in code consistent and easier to read.

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

<details markdown>
  <summary>Other install methods</summary>


**Install from homebrew (macOS)**

```bash
brew tap umputun/apps
brew install umputun/apps/unfuck-ai-comments
```

**Install from deb package (Ubuntu/Debian)**

1. Download the latest version of the package by running: `wget https://github.com/umputun/unfuck-ai-comments/releases/download/<versiom>/unfuck-ai-comments_<version>_linux_<arch>.deb` (replace `<version>` and `<arch>` with the actual values).
2. Install the package by running: `sudo dpkg -i unfuck-ai-comments_<version>_linux_<arch>.deb`

Example for the version 0.1.1 and amd64 architecture:

```bash
wget https://github.com/umputun/unfuck-ai-comments/releases/download/v0.1.1/unfuck-ai-comments_v0.1.1_linux_<arch>.deb
sudo dpkg -i unfuck-ai-comments_v0.1.1_linux_<arch>.deb
```

**Install from rpm package (CentOS/RHEL/Fedora/AWS Linux)**

```bash
wget https://github.com/umputun/unfuck-ai-comments/releases/download/v<version>/unfuck-ai-comments_v<version>_linux_<arch>.rpm
sudo rpm -i unfuck-ai-comments_v<version>_linux_<arch>.rpm
```

**Install from apk package (Alpine)**

```bash
wget https://github.com/umputun/unfuck-ai-comments/releases/download/<versiom>/unfuck-ai-comments_<version>_linux_<arch>.apk
sudo apk add unfuck-ai-comments_<version>_linux_<arch>.apk
```

</details>

## Usage

The tool uses subcommands for different operations:

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

- `--dry`:     Don't modify files, just show what would be changed (shortcut for diff command)
- `--title`:   Convert only the first character to lowercase, keep the rest unchanged
- `--fmt`:     Format the output using "go fmt"
- `--skip`:    Skip specified files or directories (can be used multiple times)
- `--backup`:  Create .bak backup files for any files that are modified

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

Skip specific files or directories:
```
unfuck-ai-comments run ./... --skip vendor --skip "*_test.go"
```

Create backup (.bak) files when modifying:
```
unfuck-ai-comments run --backup ./...
```

## How it works

The tool uses Go's AST parser to identify comments that are inside functions or structs, while leaving package comments, function documentation, and other structural comments untouched.

Comments inside function bodies and struct definitions are modified to be lowercase (or title case if the `--title` option is used).

### Special Indicator Comments

Comments that begin with special indicators are preserved completely unchanged:

- `TODO`
- `FIXME`
- `HACK`
- `XXX`
- `NOTE`
- `BUG`
- `IDEA`
- `OPTIMIZE`
- `REVIEW`
- `TEMP`
- `DEBUG`
- `NB`
- `WARNING`
- `DEPRECATED`
- `NOTICE`

For example:
```go
func Example() {
    // TODO This comment will remain COMPLETELY unchanged
    // this regular comment will be converted to lowercase
    // FIXME: This will also remain untouched
}
```