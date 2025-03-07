package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"unicode"

	"github.com/fatih/color"
	"github.com/jessevdk/go-flags"
)

//go:generate go run github.com/umputun/unfuck-ai-comments@latest run --fmt main.go main_test.go

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

	Title   bool     `long:"title" description:"Convert only the first character to lowercase, keep the rest unchanged (deprecated, now default behavior)"`
	Full    bool     `long:"full" description:"Convert entire comment to lowercase, not just the first character"`
	Skip    []string `long:"skip" description:"Skip specified directories or files (can be used multiple times)"`
	Format  bool     `long:"fmt" description:"Run gofmt on processed files"`
	Backup  bool     `long:"backup" description:"Create .bak backups of files that are modified"`
	Version bool     `short:"v" long:"version" description:"Show version information"`

	DryRun bool `long:"dry" description:"Don't modify files, just show what would be changed"`
}

// OutputWriters holds writers for stdout and stderr
type OutputWriters struct {
	Stdout io.Writer
	Stderr io.Writer
}

// DefaultWriters returns the standard output writers (os.Stdout, os.Stderr)
func DefaultWriters() OutputWriters {
	return OutputWriters{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// Define custom errors for special exit cases
var (
	ErrVersionRequested = errors.New("version info requested")
	ErrHelpRequested    = errors.New("help requested")
	ErrParsingFailed    = errors.New("parsing failed")
)

func main() {
	// use default writers (os.Stdout, os.Stderr)
	writers := DefaultWriters()

	// parse command line options
	opts, p, err := parseCommandLineOptions(writers)

	// handle special exit cases
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionRequested) || errors.Is(err, ErrHelpRequested):
			os.Exit(0)
		case errors.Is(err, ErrParsingFailed):
			os.Exit(1)
		default:
			fmt.Fprintf(writers.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
	}

	// determine mode and file patterns to process
	result := determineProcessingMode(opts, p)
	mode := result.Mode
	args := result.Patterns

	// create process request with all options
	req := ProcessRequest{
		OutputMode:   mode,
		TitleCase:    !opts.Full, // title case is default, full resets it
		Format:       opts.Format,
		SkipPatterns: opts.Skip,
		Backup:       opts.Backup,
	}

	// process each pattern
	for _, pattern := range patterns(args) {
		processPattern(pattern, &req, writers)
	}

	// print summary for run and diff modes (not print mode)
	if mode == "inplace" || mode == "diff" {
		fmt.Fprintf(writers.Stdout, "\nSummary: %d files analyzed, %d files updated, %d total changes\n",
			req.FilesAnalyzed, req.FilesUpdated, req.TotalChanges)
	}
}

// parseCommandLineOptions parses command line arguments and returns options
func parseCommandLineOptions(writers OutputWriters) (Options, *flags.Parser, error) {
	var opts Options
	p := flags.NewParser(&opts, flags.Default)
	p.LongDescription = "Convert in-function comments to lowercase while preserving comments outside functions"

	// check for standalone --version/-v flag before regular parsing
	if len(os.Args) == 2 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		showVersionInfo(writers.Stdout)
		return opts, p, ErrVersionRequested
	}

	// handle parsing errors
	if _, err := p.Parse(); err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && errors.Is(flagsErr.Type, flags.ErrHelp) {
			return opts, p, ErrHelpRequested
		}

		fmt.Fprintf(writers.Stderr, "Error: %s\n", err)
		return opts, p, ErrParsingFailed
	}

	// display version information if requested through the regular option
	if opts.Version {
		showVersionInfo(writers.Stdout)
		return opts, p, ErrVersionRequested
	}

	return opts, p, nil
}

// showVersionInfo displays the version information from Go's build info
func showVersionInfo(w io.Writer) {
	if info, ok := debug.ReadBuildInfo(); ok {
		version := info.Main.Version
		if version == "" {
			version = "dev"
		}
		fmt.Fprintf(w, "unfuck-ai-comments %s\n", version)
	} else {
		fmt.Fprintln(w, "unfuck-ai-comments (version unknown)")
	}
}

// ProcessingResult holds the result of determining the processing mode
type ProcessingResult struct {
	Mode     string
	Patterns []string
}

// determineProcessingMode figures out the processing mode and file patterns
func determineProcessingMode(opts Options, p *flags.Parser) ProcessingResult {
	// if dry run is enabled, return diff mode with run patterns
	if opts.DryRun {
		return ProcessingResult{
			Mode:     "diff",
			Patterns: opts.Run.Args.Patterns,
		}
	}

	// get processing mode and patterns based on active command
	if p.Command.Active != nil {
		switch p.Command.Active.Name {
		case "run":
			return ProcessingResult{
				Mode:     "inplace",
				Patterns: opts.Run.Args.Patterns,
			}
		case "diff":
			return ProcessingResult{
				Mode:     "diff",
				Patterns: opts.Diff.Args.Patterns,
			}
		case "print":
			return ProcessingResult{
				Mode:     "print",
				Patterns: opts.Print.Args.Patterns,
			}
		}
	}

	// default to inplace mode
	return ProcessingResult{
		Mode:     "inplace",
		Patterns: nil,
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
	Backup       bool

	// statistics for final summary
	FilesAnalyzed int
	FilesUpdated  int
	TotalChanges  int
}

// processPattern processes a single pattern
func processPattern(pattern string, req *ProcessRequest, writers OutputWriters) {
	// handle recursive pattern cases
	if isRecursivePattern(pattern) {
		dir := extractDirectoryFromPattern(pattern)
		walkDir(dir, req, writers)
		return
	}

	// find files to process
	files := findGoFilesFromPattern(pattern)
	if len(files) == 0 {
		fmt.Fprintf(writers.Stdout, "No Go files found matching pattern: %s\n", pattern)
		return
	}

	// process each file
	for _, file := range files {
		if !strings.HasSuffix(file, ".go") || shouldSkip(file, req.SkipPatterns) {
			continue
		}

		req.FilesAnalyzed++
		changes := processFile(file, req.OutputMode, req.TitleCase, req.Format, writers, req.Backup)

		if changes > 0 {
			req.FilesUpdated++
			req.TotalChanges += changes
		}
	}
}

// isRecursivePattern checks if a pattern is recursive (contains "...")
func isRecursivePattern(pattern string) bool {
	return pattern == "./..." || strings.HasSuffix(pattern, "/...") || strings.HasSuffix(pattern, "...")
}

// extractDirectoryFromPattern gets the directory part from a recursive pattern
func extractDirectoryFromPattern(pattern string) string {
	if pattern == "./..." {
		return "."
	}

	dir := strings.TrimSuffix(pattern, "/...")
	dir = strings.TrimSuffix(dir, "...")
	if dir == "" {
		dir = "."
	}
	return dir
}

// findGoFilesFromPattern finds Go files matching a pattern
func findGoFilesFromPattern(pattern string) []string {
	// first check if the pattern is a directory
	fileInfo, err := os.Stat(pattern)
	if err == nil && fileInfo.IsDir() {
		// it's a directory, find go files in it
		globPattern := filepath.Join(pattern, "*.go")
		matches, err := filepath.Glob(globPattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding Go files in %s: %v\n", pattern, err)
		}
		return matches
	}

	// not a directory, try as a glob pattern
	files, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error globbing pattern %s: %v\n", pattern, err)
		return nil
	}

	return files
}

// walkDir recursively processes all .go files in directory and subdirectories
func walkDir(dir string, req *ProcessRequest, writers OutputWriters) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// skip vendor and testdata directories
		if info.IsDir() && (info.Name() == "vendor" || strings.Contains(path, "/vendor/") ||
			info.Name() == "testdata" || strings.Contains(path, "/testdata/")) {
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

			req.FilesAnalyzed++
			changes := processFile(path, req.OutputMode, req.TitleCase, req.Format, writers, req.Backup)

			if changes > 0 {
				req.FilesUpdated++
				req.TotalChanges += changes
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(writers.Stderr, "Error walking directory %s: %v\n", dir, err)
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

// processFile processes a file using custom writers
func processFile(fileName, outputMode string, titleCase, format bool, writers OutputWriters, backup ...bool) int {
	// parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, fileName, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(writers.Stderr, "Error parsing %s: %v\n", fileName, err)
		return 0
	}

	// process comments
	numChanges, modified := processComments(node, titleCase)

	// if no comments were modified, no need to proceed
	if !modified {
		return 0
	}

	// handle output based on specified mode
	switch outputMode {
	case "inplace":
		backupEnabled := false
		if len(backup) > 0 {
			backupEnabled = backup[0]
		}
		handleInplaceMode(fileName, fset, node, format, backupEnabled, writers)
	case "print":
		handlePrintMode(fset, node, format, writers)
	case "diff":
		handleDiffMode(fileName, fset, node, format, writers)
	}

	return numChanges
}

// processComments processes all comments in the file
// returns the number of changes made and whether any modifications were made
func processComments(node *ast.File, titleCase bool) (int, bool) {
	modified := false
	changeCount := 0

	for _, commentGroup := range node.Comments {
		for _, comment := range commentGroup.List {
			// check if comment is inside a function
			if isCommentInsideFunctionOrStruct(node, comment) {
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
					changeCount++
				}
			}
		}
	}
	return changeCount, modified
}

// getModifiedContent generates the modified content as a string
func getModifiedContent(fset *token.FileSet, node *ast.File) (string, error) {
	var modifiedBuf strings.Builder
	if err := printer.Fprint(&modifiedBuf, fset, node); err != nil {
		return "", fmt.Errorf("save modified buffer: %w", err)
	}
	return modifiedBuf.String(), nil
}

// handleInplaceMode writes modified content back to the file with custom writers
func handleInplaceMode(fileName string, fset *token.FileSet, node *ast.File, format, backup bool, writers OutputWriters) {
	// create backup if requested
	if backup {
		createBackupIfNeeded(fileName, fset, node)
	}

	// write the modified content to file
	file, err := os.Create(fileName) //nolint:gosec
	if err != nil {
		fmt.Fprintf(writers.Stderr, "Error opening %s for writing: %v\n", fileName, err)
		return
	}
	defer func() { _ = file.Close() }()

	if err := printer.Fprint(file, fset, node); err != nil {
		fmt.Fprintf(writers.Stderr, "Error writing to file %s: %v\n", fileName, err)
		return
	}

	fmt.Fprintf(writers.Stdout, "Updated: %s\n", fileName)

	// run gofmt if requested
	if format {
		runGoFmt(fileName)
	}
}

// createBackupIfNeeded creates a backup of the file if content will change
func createBackupIfNeeded(fileName string, fset *token.FileSet, node *ast.File) {
	// read the original content
	origContent, err := os.ReadFile(fileName) //nolint:gosec
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file for backup %s: %v\n", fileName, err)
		return
	}

	// get the modified content
	modifiedContent, err := getModifiedContent(fset, node)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating modified content for %s: %v\n", fileName, err)
		return
	}

	// only create a backup if the file is actually going to change
	if string(origContent) != modifiedContent {
		backupFile := fileName + ".bak"
		if err := os.WriteFile(backupFile, origContent, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating backup file %s: %v\n", backupFile, err)
		}
	}
}

// handlePrintMode prints the modified content to stdout with custom writers
func handlePrintMode(fset *token.FileSet, node *ast.File, format bool, writers OutputWriters) {
	var modifiedBytes strings.Builder
	if err := printer.Fprint(&modifiedBytes, fset, node); err != nil {
		fmt.Fprintf(writers.Stderr, "Error writing to stdout: %v\n", err)
		return
	}

	content := modifiedBytes.String()
	if format {
		content = formatWithGofmt(content)
	}
	fmt.Fprint(writers.Stdout, content)
}

// handleDiffMode shows a diff between original and modified content with custom writers
func handleDiffMode(fileName string, fset *token.FileSet, node *ast.File, format bool, writers OutputWriters) {
	// read original content
	origBytes, err := os.ReadFile(fileName) //nolint:gosec
	if err != nil {
		fmt.Fprintf(writers.Stderr, "Error reading original file %s: %v\n", fileName, err)
		return
	}

	// generate modified content
	var modifiedBytes strings.Builder
	if err := printer.Fprint(&modifiedBytes, fset, node); err != nil {
		fmt.Fprintf(writers.Stderr, "Error creating diff: %v\n", err)
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

	// display diff with colors
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	fmt.Fprintf(writers.Stdout, "%s\n", cyan("--- "+fileName+" (original)"))
	fmt.Fprintf(writers.Stdout, "%s\n", cyan("+++ "+fileName+" (modified)"))
	fmt.Fprint(writers.Stdout, simpleDiff(originalContent, modifiedContent))
}

// isCommentInsideFunctionOrStruct checks if a comment is inside a function declaration, struct declaration,
// var block, or const block
func isCommentInsideFunctionOrStruct(file *ast.File, comment *ast.Comment) bool {
	commentPos := comment.Pos()

	// find if comment is inside a function, struct, var block, or const block
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
		case *ast.GenDecl:
			// handle variable and constant declarations in blocks
			if node.Tok == token.VAR || node.Tok == token.CONST {
				// check if it's a block declaration (with braces)
				if node.Lparen != token.NoPos && node.Rparen != token.NoPos {
					// check if comment is inside the block (between braces)
					if node.Lparen <= commentPos && commentPos <= node.Rparen {
						insideNode = true
						return false // stop traversal
					}
				}
			}
		}
		return true
	})

	return insideNode
}

// specialIndicators that should be preserved in comments
var specialIndicators = []string{
	"TODO", "FIXME", "HACK", "XXX", "NOTE", "BUG", "IDEA", "OPTIMIZE",
	"REVIEW", "TEMP", "DEBUG", "NB", "WARNING", "DEPRECATED", "NOTICE",
}

// hasSpecialIndicator checks if a comment starts with a special indicator
func hasSpecialIndicator(content string) bool {
	trimmedContent := strings.TrimSpace(content)
	for _, indicator := range specialIndicators {
		if strings.HasPrefix(trimmedContent, indicator) {
			return true
		}
	}
	return false
}

// processLineComment handles single line comments (// style)
// it gets the content after "//" and processes it
func processLineComment(content string, fullLowercase bool) string {
	// check if this comment starts with a special indicator
	if hasSpecialIndicator(content) {
		// if comment starts with a special indicator, leave it unchanged
		return "//" + content
	}

	// Handle double comment format like "nolint:gosec // using math/rand is acceptable for tests"
	// by finding the second "//" and processing each part appropriately

	// try to find different formats of technical comments
	for _, sep := range []string{" // ", "//", " //"} {
		if idx := strings.Index(content, sep); idx >= 0 {
			// for the first part (typically a directive like "nolint:gosec"), leave it unchanged
			firstPart := content[:idx]
			// process the second part (actual comment) according to the rules
			secondPart := processCommentPart(content[idx+len(sep):], fullLowercase, getCommentIdentifiers(content[idx+len(sep):]))
			return "//" + firstPart + sep + secondPart
		}
	}

	// for normal comments, process the entire content
	return "//" + processCommentPart(content, fullLowercase, getCommentIdentifiers(content))
}

// processCommentPart handles the processing of a single comment part
func processCommentPart(content string, fullLowercase bool, identifiers []string) string {
	if fullLowercase {
		// convert entire comment to lowercase
		res := strings.ToLower(content)
		for _, id := range identifiers {
			res = strings.ReplaceAll(res, strings.ToLower(id), id)
		}
		return res
	}

	// for title case, convert only the first non-whitespace character
	leadingWhitespace := ""
	remainingContent := content
	for i, r := range content {
		if !unicode.IsSpace(r) {
			leadingWhitespace = content[:i]
			remainingContent = content[i:]
			break
		}
	}

	if remainingContent == "" {
		return content
	}

	// check if the first word is all uppercase (for abbreviations like AI, CPU)
	firstWordEnd := 0
	isAllUppercase := true
	for i, r := range remainingContent {
		if unicode.IsSpace(r) || !unicode.IsLetter(r) {
			firstWordEnd = i
			break
		}
		if !unicode.IsUpper(r) {
			isAllUppercase = false
		}
		if i == len(remainingContent)-1 {
			firstWordEnd = i + 1 // handle case where comment is a single word
		}
	}

	// if first word is all uppercase and at least 2 characters, preserve it
	if isAllUppercase && firstWordEnd >= 2 {
		return content
	}

	// check if the first word is in identifiers and preserve it
	for _, id := range identifiers {
		if strings.EqualFold(id, remainingContent[:firstWordEnd]) {
			return content
		}
	}

	// otherwise convert first character to lowercase
	firstChar := strings.ToLower(string(remainingContent[0]))
	if len(remainingContent) > 1 {
		return leadingWhitespace + firstChar + remainingContent[1:]
	}
	return leadingWhitespace + firstChar
}

// getCommentIdentifiers extracts identifiers from a comment
// identifiers are words with either pascal case or camel case
func getCommentIdentifiers(content string) []string {
	isPascalCase := func(s string) bool {
		// pascal case requires uppercase first letter, at least one more uppercase letter
		// followed by lowercase, and no consecutive uppercase letters
		if len(s) < 2 || !unicode.IsUpper(rune(s[0])) {
			return false
		}

		foundSecondUpper := false
		for i := 1; i < len(s); i++ {
			// check for consecutive uppercase letters (which invalidates pascal case)
			if unicode.IsUpper(rune(s[i])) && i > 0 && unicode.IsUpper(rune(s[i-1])) {
				return false
			}

			// valid pattern: uppercase followed by lowercase
			if unicode.IsUpper(rune(s[i])) && i+1 < len(s) && unicode.IsLower(rune(s[i+1])) {
				foundSecondUpper = true
			}
		}

		return foundSecondUpper
	}

	isCamelCase := func(s string) bool {
		// if at least one uppercase letter is found, and it's not the first character, it's camel case
		for i, r := range s {
			if i == 0 && unicode.IsUpper(r) {
				return false
			}
			if unicode.IsUpper(r) && i > 0 {
				return true
			}
		}
		return false
	}

	words := strings.Fields(content)
	var identifiers []string
	for _, word := range words {
		if isPascalCase(word) || isCamelCase(word) {
			identifiers = append(identifiers, word)
		}
	}
	return identifiers
}

// convertCommentToLowercase converts a comment to lowercase, preserving the comment markers
// If comment starts with a special indicator like TODO, FIXME, etc. it remains unchanged
func convertCommentToLowercase(comment string) string {
	if strings.HasPrefix(comment, "//") {
		content := strings.TrimPrefix(comment, "//")
		return processLineComment(content, true)
	}
	return comment
}

// convertCommentToTitleCase converts only the first character of a comment to lowercase,
// If comment starts with a special indicator like TODO, FIXME, etc. it remains unchanged
func convertCommentToTitleCase(comment string) string {
	if strings.HasPrefix(comment, "//") {
		content := strings.TrimPrefix(comment, "//")
		return processLineComment(content, false)
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
