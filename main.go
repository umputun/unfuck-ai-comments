package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/fatih/color"
	"github.com/jessevdk/go-flags"
)

//go:generate go run github.com/umputun/unfuck-ai-comments@latest run --title --fmt main.go main_test.go

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

	Title  bool     `long:"title" description:"Convert only the first character to lowercase, keep the rest unchanged"`
	Skip   []string `long:"skip" description:"Skip specified directories or files (can be used multiple times)"`
	Format bool     `long:"fmt" description:"Run gofmt on processed files"`

	DryRun bool `long:"dry" description:"Don't modify files, just show what would be changed"`
}

var osExit = os.Exit // replace os.Exit with a variable for testing

func main() {
	// define options
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

	// create process request with all options
	req := ProcessRequest{
		OutputMode:   mode,
		TitleCase:    opts.Title,
		Format:       opts.Format,
		SkipPatterns: opts.Skip,
	}

	// process each pattern
	for _, pattern := range patterns(args) {
		processPattern(pattern, req)
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

// ProcessRequest contains all processing parameters
type ProcessRequest struct {
	OutputMode   string
	TitleCase    bool
	Format       bool
	SkipPatterns []string
}

// processPattern processes a single pattern
func processPattern(pattern string, req ProcessRequest) {
	// handle special "./..." pattern for recursive search
	if pattern == "./..." {
		walkDir(".", req)
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
		walkDir(dir, req)
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

		// check if file should be skipped
		if shouldSkip(file, req.SkipPatterns) {
			continue
		}

		processFile(file, req.OutputMode, req.TitleCase, req.Format)
	}
}

// walkDir recursively processes all .go files in directory and subdirectories
func walkDir(dir string, req ProcessRequest) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// skip vendor directories
		if info.IsDir() && (info.Name() == "vendor" || strings.Contains(path, "/vendor/")) {
			return filepath.SkipDir
		}

		// check if directory should be skipped
		if info.IsDir() && shouldSkip(path, req.SkipPatterns) {
			return filepath.SkipDir
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			// check if file should be skipped
			if shouldSkip(path, req.SkipPatterns) {
				return nil
			}
			processFile(path, req.OutputMode, req.TitleCase, req.Format)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory %s: %v\n", dir, err)
	}
}

// shouldSkip checks if a path should be skipped based on skip patterns
func shouldSkip(path string, skipPatterns []string) bool {
	if len(skipPatterns) == 0 {
		return false
	}

	// normalize path
	normalizedPath := filepath.Clean(path)

	for _, skipPattern := range skipPatterns {
		// check for exact match
		if skipPattern == normalizedPath {
			return true
		}

		// check if path is within a skipped directory
		if strings.HasPrefix(normalizedPath, skipPattern+string(filepath.Separator)) {
			return true
		}

		// check for glob pattern match
		matched, err := filepath.Match(skipPattern, normalizedPath)
		if err == nil && matched {
			return true
		}

		// also check just the base name for simple pattern matching
		matched, err = filepath.Match(skipPattern, filepath.Base(normalizedPath))
		if err == nil && matched {
			return true
		}
	}

	return false
}

// runGoFmt runs gofmt on the specified file
func runGoFmt(fileName string) {
	// use gofmt with settings that preserve original formatting as much as possible
	cmd := exec.Command("gofmt", "-w", "-s", fileName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running gofmt on %s: %v\n%s", fileName, err, output)
	}
}

// formatWithGofmt formats the given content with gofmt
// returns the original content if formatting fails
func formatWithGofmt(content string) string {
	cmd := exec.Command("gofmt", "-s")
	cmd.Stdin = strings.NewReader(content)

	// capture the stdout output
	formattedBytes, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting with gofmt: %v\n", err)
		return content // return original content on error
	}

	return string(formattedBytes)
}

func processFile(fileName, outputMode string, titleCase, format bool) {
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
				var processed string
				if titleCase {
					processed = convertCommentToTitleCase(orig)
				} else {
					processed = convertCommentToLowercase(orig)
				}
				if orig != processed {
					comment.Text = processed
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

		// run gofmt if requested
		if format {
			runGoFmt(fileName)
		}

	case "print":
		// print modified source to stdout
		var modifiedBytes strings.Builder
		if err := printer.Fprint(&modifiedBytes, fset, node); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to stdout: %v\n", err)
			return
		}

		// if format is requested, use our helper function
		if format {
			formattedContent := formatWithGofmt(modifiedBytes.String())
			fmt.Print(formattedContent)
		} else {
			fmt.Print(modifiedBytes.String())
		}

	case "diff":
		// generate diff output
		origBytes, err := os.ReadFile(fileName) //nolint:gosec
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading original file %s: %v\n", fileName, err)
			return
		}

		// generate modified content
		var modifiedBytes strings.Builder
		if err := printer.Fprint(&modifiedBytes, fset, node); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating diff: %v\n", err)
			return
		}

		// get original and modified content
		originalContent := string(origBytes)
		modifiedContent := modifiedBytes.String()

		// apply formatting if requested
		if format {
			// format both original and modified content for consistency
			originalContent = formatWithGofmt(originalContent)
			modifiedContent = formatWithGofmt(modifiedContent)
		}

		// use cyan for file information
		cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
		fmt.Printf("%s\n", cyan("--- "+fileName+" (original)"))
		fmt.Printf("%s\n", cyan("+++ "+fileName+" (modified)"))

		// print the diff with colors
		fmt.Print(simpleDiff(originalContent, modifiedContent))
	}
}

// isCommentInsideFunction checks if a comment is inside a function declaration or a struct declaration
func isCommentInsideFunction(_ *token.FileSet, file *ast.File, comment *ast.Comment) bool {
	commentPos := comment.Pos()

	// find if comment is inside a function or struct
	var insideNode bool
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		switch node := n.(type) {
		case *ast.FuncDecl:
			// check if comment is inside function body
			if node.Body != nil && node.Body.Lbrace <= commentPos && commentPos <= node.Body.Rbrace {
				insideNode = true
				return false // stop traversal
			}
		case *ast.StructType:
			// check if comment is inside struct definition (between braces)
			if node.Fields != nil && node.Fields.Opening <= commentPos && commentPos <= node.Fields.Closing {
				insideNode = true
				return false // stop traversal
			}
		}
		return true
	})

	return insideNode
}

// convertCommentToLowercase converts a comment to lowercase, preserving the comment markers
// If comment starts with a special indicator like TODO, FIXME, etc. it remains unchanged
func convertCommentToLowercase(comment string) string {
	// list of special indicators that should be preserved
	specialIndicators := []string{
		"TODO", "FIXME", "HACK", "XXX", "NOTE", "BUG", "IDEA", "OPTIMIZE",
		"REVIEW", "TEMP", "DEBUG", "NB", "WARNING", "DEPRECATED", "NOTICE",
	}

	if strings.HasPrefix(comment, "//") {
		// single line comment
		content := strings.TrimPrefix(comment, "//")

		// check if this comment starts with a special indicator
		trimmedContent := strings.TrimSpace(content)
		for _, indicator := range specialIndicators {
			if strings.HasPrefix(trimmedContent, indicator) {
				// if comment starts with a special indicator, leave it unchanged
				return comment
			}
		}

		// otherwise convert to lowercase
		return "//" + strings.ToLower(content)
	}

	if strings.HasPrefix(comment, "/*") && strings.HasSuffix(comment, "*/") {
		// multi-line comment
		content := strings.TrimSuffix(strings.TrimPrefix(comment, "/*"), "*/")

		// check first line for special indicators
		lines := strings.Split(content, "\n")
		if len(lines) > 0 {
			trimmedFirstLine := strings.TrimSpace(lines[0])
			for _, indicator := range specialIndicators {
				if strings.HasPrefix(trimmedFirstLine, indicator) {
					// if first line starts with a special indicator, leave the comment unchanged
					return comment
				}
			}
		}

		// otherwise convert to lowercase
		return "/*" + strings.ToLower(content) + "*/"
	}

	return comment
}

// convertCommentToTitleCase converts only the first character of a comment to lowercase,
// If comment starts with a special indicator like TODO, FIXME, etc. it remains unchanged
func convertCommentToTitleCase(comment string) string {
	// list of special indicators that should be preserved
	specialIndicators := []string{
		"TODO", "FIXME", "HACK", "XXX", "NOTE", "BUG", "IDEA", "OPTIMIZE",
		"REVIEW", "TEMP", "DEBUG", "NB", "WARNING", "DEPRECATED", "NOTICE",
	}

	if strings.HasPrefix(comment, "//") {
		// single line comment
		content := strings.TrimPrefix(comment, "//")

		// check if this comment starts with a special indicator
		trimmedContent := strings.TrimSpace(content)
		for _, indicator := range specialIndicators {
			if strings.HasPrefix(trimmedContent, indicator) {
				// if comment starts with a special indicator, leave it unchanged
				return comment
			}
		}

		// otherwise convert only the first character to lowercase
		leadingWhitespace := ""
		remainingContent := content
		for i, r := range content {
			if !unicode.IsSpace(r) {
				leadingWhitespace = content[:i]
				remainingContent = content[i:]
				break
			}
		}

		if remainingContent != "" {
			firstChar := strings.ToLower(string(remainingContent[0]))
			if len(remainingContent) > 1 {
				// keep the rest of the text as is
				return "//" + leadingWhitespace + firstChar + remainingContent[1:]
			}
			return "//" + leadingWhitespace + firstChar
		}
		return "//" + content
	}

	if strings.HasPrefix(comment, "/*") && strings.HasSuffix(comment, "*/") {
		// multi-line comment
		content := strings.TrimSuffix(strings.TrimPrefix(comment, "/*"), "*/")

		// check first line for special indicators
		lines := strings.Split(content, "\n")
		if len(lines) > 0 {
			trimmedFirstLine := strings.TrimSpace(lines[0])
			for _, indicator := range specialIndicators {
				if strings.HasPrefix(trimmedFirstLine, indicator) {
					// if first line starts with a special indicator, leave the comment unchanged
					return comment
				}
			}

			// process the first line - lowercase only the first character
			if lines[0] != "" {
				// find the first non-whitespace character
				leadingWhitespace := ""
				remainingText := lines[0]
				for i, r := range lines[0] {
					if !unicode.IsSpace(r) {
						leadingWhitespace = lines[0][:i]
						remainingText = lines[0][i:]
						break
					}
				}

				if remainingText != "" {
					// convert only the first non-whitespace character to lowercase
					firstChar := strings.ToLower(string(remainingText[0]))
					if len(remainingText) > 1 {
						lines[0] = leadingWhitespace + firstChar + remainingText[1:]
					} else {
						lines[0] = leadingWhitespace + firstChar
					}
				}
			}
		}

		return "/*" + strings.Join(lines, "\n") + "*/"
	}

	return comment
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
