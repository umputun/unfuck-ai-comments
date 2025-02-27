package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/jessevdk/go-flags"
)

// Options holds command line options
type Options struct {
	Run struct {
		Args struct {
			Patterns []string `positional-arg-name:"FILE/PATTERN" description:"Files or patterns to process (default: current directory)"`
		} `positional-args:"yes"`
	} `command:"run" description:"Process files in-place (default)"`

	Diff struct {
		Args struct {
			Patterns []string `positional-arg-name:"FILE/PATTERN" description:"Files or patterns to process (default: current directory)"`
		} `positional-args:"yes"`
	} `command:"diff" description:"Show diff without modifying files"`

	Print struct {
		Args struct {
			Patterns []string `positional-arg-name:"FILE/PATTERN" description:"Files or patterns to process (default: current directory)"`
		} `positional-args:"yes"`
	} `command:"print" description:"Print processed content to stdout"`

	DryRun bool `long:"dry" description:"Don't modify files, just show what would be changed"`
}

var osExit = os.Exit // replace os.Exit with a variable for testing

func main() {
	// sefine options
	var opts Options
	p := flags.NewParser(&opts, flags.Default)

	p.LongDescription = "Convert in-function comments to lowercase while preserving comments outside functions"

	// handle parsing errors
	if _, err := p.Parse(); err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && errors.Is(flagsErr.Type, flags.ErrHelp) {
			osExit(0)
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		osExit(1)
	}

	// determine the mode based on command or flags
	mode := "inplace" // default

	var args []string
	// process according to command or flags
	if p.Command.Active != nil {
		// command was specified
		switch p.Command.Active.Name {
		case "run":
			mode = "inplace"
			args = opts.Run.Args.Patterns
		case "diff":
			mode = "diff"
			args = opts.Diff.Args.Patterns
		case "print":
			mode = "print"
			args = opts.Print.Args.Patterns
		}
	}

	if opts.DryRun {
		mode = "diff"
		args = opts.Run.Args.Patterns
	}

	// process each pattern
	for _, pattern := range patterns(args) {
		processPattern(pattern, mode)
	}
}

// patterns to process, defaulting to current directory
func patterns(p []string) []string {
	res := p
	if len(res) == 0 {
		res = []string{"."}
	}
	return res
}

// processPattern processes a single pattern
func processPattern(pattern, outputMode string) {
	// handle special "./..." pattern for recursive search
	if pattern == "./..." {
		walkDir(".", outputMode)
		return
	}

	// if it's a recursive pattern, handle it
	if strings.HasSuffix(pattern, "/...") || strings.HasSuffix(pattern, "...") {
		// extract the directory part
		dir := strings.TrimSuffix(pattern, "/...")
		dir = strings.TrimSuffix(dir, "...")
		if dir == "" {
			dir = "."
		}
		walkDir(dir, outputMode)
		return
	}

	// initialize files slice
	var files []string

	// first check if the pattern is a directory
	fileInfo, err := os.Stat(pattern)
	if err == nil && fileInfo.IsDir() {
		// it's a directory, find go files in it
		globPattern := filepath.Join(pattern, "*.go")
		matches, err := filepath.Glob(globPattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding Go files in %s: %v\n", pattern, err)
		}
		if len(matches) > 0 {
			files = matches
		}
	} else {
		// not a directory, try as a glob pattern
		files, err = filepath.Glob(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error globbing pattern %s: %v\n", pattern, err)
			return
		}
	}

	if len(files) == 0 {
		fmt.Printf("No Go files found matching pattern: %s\n", pattern)
		return
	}

	// process each file
	for _, file := range files {
		if !strings.HasSuffix(file, ".go") {
			continue
		}
		processFile(file, outputMode)
	}
}

// walkDir recursively processes all .go files in directory and subdirectories
func walkDir(dir, outputMode string) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// skip vendor directories
		if info.IsDir() && (info.Name() == "vendor" || strings.Contains(path, "/vendor/")) {
			return filepath.SkipDir
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			processFile(path, outputMode)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory %s: %v\n", dir, err)
	}
}

func processFile(fileName, outputMode string) {
	// parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, fileName, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", fileName, err)
		return
	}

	// process comments
	modified := false
	for _, commentGroup := range node.Comments {
		for _, comment := range commentGroup.List {
			// check if comment is inside a function
			if isCommentInsideFunction(fset, node, comment) {
				// process the comment text
				orig := comment.Text
				lower := convertCommentToLowercase(orig)
				if orig != lower {
					comment.Text = lower
					modified = true
				}
			}
		}
	}

	// if no comments were modified, no need to proceed
	if !modified {
		return
	}

	// handle output based on specified mode
	switch outputMode {
	case "inplace":
		// write modified source back to file
		file, err := os.Create(fileName) //nolint:gosec
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s for writing: %v\n", fileName, err)
			return
		}
		defer file.Close()
		if err := printer.Fprint(file, fset, node); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to file %s: %v\n", fileName, err)
			return
		}
		fmt.Printf("Updated: %s\n", fileName)

	case "print":
		// print modified source to stdout
		if err := printer.Fprint(os.Stdout, fset, node); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to stdout: %v\n", err)
			return
		}

	case "diff":
		// generate diff output
		origBytes, err := os.ReadFile(fileName) //nolint:gosec
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading original file %s: %v\n", fileName, err)
			return
		}

		// format the original content with gofmt
		formattedOrig, err := formatGoCode(string(origBytes))
		if err != nil {
			// if formatting fails, fall back to original
			formattedOrig = string(origBytes)
		}

		// generate modified content
		var modifiedBytes strings.Builder
		if err := printer.Fprint(&modifiedBytes, fset, node); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating diff: %v\n", err)
			return
		}

		// format the modified content with gofmt
		formattedMod, err := formatGoCode(modifiedBytes.String())
		if err != nil {
			// if formatting fails, fall back to unformatted
			formattedMod = modifiedBytes.String()
		}

		// use cyan for file information
		cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
		fmt.Printf("%s\n", cyan("--- "+fileName+" (original)"))
		fmt.Printf("%s\n", cyan("+++ "+fileName+" (modified)"))

		// print the diff with colors
		fmt.Print(simpleDiff(formattedOrig, formattedMod))
	}
}

// isCommentInsideFunction checks if a comment is inside a function declaration
func isCommentInsideFunction(_ *token.FileSet, file *ast.File, comment *ast.Comment) bool {
	commentPos := comment.Pos()

	// find function containing the comment
	var insideFunc bool
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		// check if this is a function declaration
		fn, isFunc := n.(*ast.FuncDecl)
		if isFunc {
			// check if comment is inside function body
			if fn.Body != nil && fn.Body.Lbrace <= commentPos && commentPos <= fn.Body.Rbrace {
				insideFunc = true
				return false // stop traversal
			}
		}
		return true
	})

	return insideFunc
}

// convertCommentToLowercase converts a comment to lowercase, preserving the comment markers
func convertCommentToLowercase(comment string) string {
	if strings.HasPrefix(comment, "//") {
		// single line comment
		content := strings.TrimPrefix(comment, "//")
		return "//" + strings.ToLower(content)
	}
	if strings.HasPrefix(comment, "/*") && strings.HasSuffix(comment, "*/") {
		// multi-line comment
		content := strings.TrimSuffix(strings.TrimPrefix(comment, "/*"), "*/")
		return "/*" + strings.ToLower(content) + "*/"
	}
	return comment
}

// formatGoCode formats Go code using go/format.
func formatGoCode(src string) (string, error) {
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return "", err
	}
	return string(formatted), nil
}

// simpleDiff creates a colorized diff output
func simpleDiff(original, modified string) string {
	origLines := strings.Split(original, "\n")
	modLines := strings.Split(modified, "\n")

	// set up colors - use bright versions for better visibility
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()

	var diff strings.Builder

	for i := 0; i < len(origLines) || i < len(modLines); i++ {
		switch {
		case i >= len(origLines):
			diff.WriteString(green("+ "+modLines[i]) + "\n")
		case i >= len(modLines):
			diff.WriteString(red("- "+origLines[i]) + "\n")
		case origLines[i] != modLines[i]:
			diff.WriteString(red("- "+origLines[i]) + "\n")
			diff.WriteString(green("+ "+modLines[i]) + "\n")
		}
	}

	return diff.String()
}
