package main

import (
	"flag"
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
)

// replace os.Exit with a variable for testing
var osExit = os.Exit

func main() {
	// define command line flags
	outputMode := flag.String("output", "inplace", "Output mode: inplace, print, diff")
	dryRun := flag.Bool("dry-run", false, "Don't modify files, just show what would be changed")
	showHelp := flag.Bool("help", false, "Show usage information")
	noColor := flag.Bool("no-color", false, "Disable colorized output")
	flag.Parse()

	// enable colors by default
	color.NoColor = *noColor

	// if dry-run is set, override output mode to diff
	if *dryRun {
		*outputMode = "diff"
	}

	// show help if requested
	if *showHelp {
		fmt.Println("unfuck-ai-comments - Convert in-function comments to lowercase")
		fmt.Println("\nUsage:")
		fmt.Println("  unfuck-ai-comments [options] [file/pattern...]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  unfuck-ai-comments                       # Process all .go files in current directory")
		fmt.Println("  unfuck-ai-comments file.go               # Process specific file")
		fmt.Println("  unfuck-ai-comments ./...                 # Process all .go files recursively")
		fmt.Println("  unfuck-ai-comments -output=print file.go # Print modified file to stdout")
		fmt.Println("  unfuck-ai-comments -output=diff *.go     # Show diff for all .go files")
		osExit(0)
	}

	// get files to process
	patterns := []string{"."}
	if flag.NArg() > 0 {
		patterns = flag.Args()
	}

	// process each pattern
	for _, pattern := range patterns {
		processPattern(pattern, *outputMode)
	}
}

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
		file, err := os.Create(fileName)
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
		origBytes, err := os.ReadFile(fileName)
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
				return false	// stop traversal
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
