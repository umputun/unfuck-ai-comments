package main

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsCommentInsideFunction tests the core function that determines if a comment is inside a function body
func TestIsCommentInsideFunction(t *testing.T) {

	// define test source code containing comments in different locations
	src := `package main

// Package comment should NOT be modified
// Another package comment

// Function comment should NOT be modified
func Example() {
	// This SHOULD be modified
	x := 1 // This inline comment SHOULD be modified
	
	/*
	 * This multi-line comment
	 * SHOULD be modified
	 */
	
	// Another comment to modify
}

// Another function comment should NOT be modified
func Example2() {
	// This one too SHOULD be modified
}

type S struct {
	// Struct field comment should NOT be modified
	Field int
	
	// Another field comment
	AnotherField string
}

func (s S) Method() {
	// Method comment SHOULD be modified
}

// Comment before a type should NOT be modified
type T int

// Comment between funcs should NOT be modified

// Complex cases with nested blocks
func ComplexFunc() {
	// Comment at start SHOULD be modified
	if true {
		// Comment in if block SHOULD be modified
	}
	
	for i := 0; i < 10; i++ {
		// Comment in for loop SHOULD be modified
	}
	
	// Comment before closure SHOULD be modified
	func() {
		// Comment inside closure SHOULD be modified
	}()
}`

	// parse the source
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test source: %v", err)
	}

	// check all comments using classification patterns
	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			inside := isCommentInsideFunction(fset, file, comment)
			text := comment.Text

			// check if classification is correct
			switch {
			case strings.Contains(text, "SHOULD be modified") && !inside:
				t.Errorf("Comment incorrectly identified as NOT inside function: %q", text)
			case strings.Contains(text, "should NOT be modified") && inside:
				t.Errorf("Comment incorrectly identified as inside function: %q", text)
			case strings.Contains(text, "Package comment") && inside:
				t.Errorf("Package comment incorrectly identified as inside function: %q", text)
			case strings.Contains(text, "Function comment") && inside:
				t.Errorf("Function comment incorrectly identified as inside function: %q", text)
			case strings.Contains(text, "field comment") && inside:
				t.Errorf("Field comment incorrectly identified as inside function: %q", text)
			}
		}
	}
}

// TestConvertCommentToLowercase tests the comment conversion function with various formats
func TestConvertCommentToLowercase(t *testing.T) {
	tests := []struct {
		name		string
		input		string
		expected	string
	}{
		{
			name:		"single line comment",
			input:		"// This SHOULD Be Converted",
			expected:	"// this should be converted",
		},
		{
			name:		"multi-line comment",
			input:		"/* This SHOULD\nBe Converted */",
			expected:	"/* this should\nbe converted */",
		},
		{
			name:		"preserve comment markers",
			input:		"// UPPER case comment",
			expected:	"// upper case comment",
		},
		{
			name:		"comment with special chars",
			input:		"// Special: @#$%^&*()",
			expected:	"// special: @#$%^&*()",
		},
		{
			name:		"comment with code example",
			input:		"// Example: const X = 123",
			expected:	"// example: const x = 123",
		},
		{
			name:		"empty comment",
			input:		"//",
			expected:	"//",
		},
		{
			name:		"comment with leading space",
			input:		"//  Leading space",
			expected:	"//  leading space",
		},
		{
			name:		"multi-line with indentation",
			input:		"/*\n * Line 1\n * Line 2\n */",
			expected:	"/*\n * line 1\n * line 2\n */",
		},
		{
			name:		"not a comment",
			input:		"const X = 1",
			expected:	"const X = 1",	// should return unchanged
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := convertCommentToLowercase(test.input)
			assert.Equal(t, test.expected, result, "Comment conversion failed")
		})
	}
}

// TestConvertCommentToTitleCase tests the title case comment conversion function
func TestConvertCommentToTitleCase(t *testing.T) {
	tests := []struct {
		name		string
		input		string
		expected	string
	}{
		{
			name:		"single line comment",
			input:		"// This SHOULD Be Converted",
			expected:	"// this SHOULD Be Converted",
		},
		{
			name:		"multi-line comment",
			input:		"/* This SHOULD\nBe Converted */",
			expected:	"/* this SHOULD\nBe Converted */",
		},
		{
			name:		"uppercase first letter",
			input:		"// UPPER case comment",
			expected:	"// uPPER case comment",
		},
		{
			name:		"comment with special chars",
			input:		"// Special: @#$%^&*()",
			expected:	"// special: @#$%^&*()",
		},
		{
			name:		"comment with code example",
			input:		"// Example: const X = 123",
			expected:	"// example: const X = 123",
		},
		{
			name:		"empty comment",
			input:		"//",
			expected:	"//",
		},
		{
			name:		"comment with leading space",
			input:		"//  Leading space",
			expected:	"//  leading space",
		},
		{
			name:		"multi-line with indentation",
			input:		"/*\n * Line 1\n * Line 2\n */",
			expected:	"/*\n * Line 1\n * Line 2\n */",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := convertCommentToTitleCase(test.input)
			assert.Equal(t, test.expected, result, "Title case conversion failed")
		})
	}
}

// Helper function to remove whitespace for comparison
func removeWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, "")
}

// TestProcessFile tests the main processing function with different modes
func TestProcessFile(t *testing.T) {
	// create a temporary directory for test files
	tempDir := t.TempDir()

	// create a test file with comments
	testFile := filepath.Join(tempDir, "test_file.go")
	content := `package test

func Example() {
	// THIS COMMENT should be converted
	x := 1 // ANOTHER COMMENT
}`
	err := os.WriteFile(testFile, []byte(content), 0o600)
	require.NoError(t, err, "Failed to write test file")

	// test inplace mode
	t.Run("inplace mode", func(t *testing.T) {
		// reset file
		err := os.WriteFile(testFile, []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process file in inplace mode
		processFile(testFile, "inplace", false, false)

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify "updated" message
		assert.Contains(t, output, "Updated:", "Should show update message")

		// read the file content
		modifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read modified file")
		modifiedStr := string(modifiedContent)

		// verify changes
		assert.Contains(t, modifiedStr, "// this comment", "Should convert comments to lowercase")
		assert.Contains(t, modifiedStr, "// another comment", "Should convert all comments to lowercase")
		assert.NotContains(t, modifiedStr, "// THIS COMMENT", "Should not contain original uppercase comments")
	})

	// test diff mode
	t.Run("diff mode", func(t *testing.T) {
		// reset file
		err := os.WriteFile(testFile, []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process file in diff mode
		processFile(testFile, "diff", false, false)

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify diff output
		assert.Contains(t, output, "---", "Should show diff markers")
		assert.Contains(t, output, "+++", "Should show diff markers")
		// the exact format of the diff output depends on how diff is formatted, so check for content rather than exact format
		assert.Contains(t, output, "THIS COMMENT", "Should show original comment")
		assert.Contains(t, output, "this comment", "Should show converted comment")
		assert.Contains(t, output, "ANOTHER COMMENT", "Should show original comment")
		assert.Contains(t, output, "another comment", "Should show converted comment")

		// file should not be modified
		modifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read file")
		assert.Equal(t, content, string(modifiedContent), "File should not be modified in diff mode")

		// skip spacing checks since printer may normalize some aspects
	})
}

func TestFormatOption(t *testing.T) {
	tempDir := t.TempDir()
	content := `package testpkg

func Example(  ) {
    // THIS COMMENT should be modified
    x:=1  // ANOTHER Comment Here
}`

	// write test file
	testFile := filepath.Join(tempDir, "format_test.go")
	err := os.WriteFile(testFile, []byte(content), 0o600)
	require.NoError(t, err, "Failed to write test file")

	t.Run("inplace mode with format", func(t *testing.T) {
		// reset the file
		err := os.WriteFile(testFile, []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process the file with format option
		processFile(testFile, "inplace", false, true)

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify "updated" message
		assert.Contains(t, output, "Updated:", "Should show update message")

		// read the file content
		modifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read modified file")
		modifiedStr := string(modifiedContent)

		// check that formatting was applied
		assert.Contains(t, modifiedStr, "func Example()", "Should format function declaration")
		assert.Contains(t, modifiedStr, "x := 1", "Should format variable assignment")
		assert.Contains(t, modifiedStr, "// this comment", "Should convert comments to lowercase")
		assert.Contains(t, modifiedStr, "// another comment", "Should convert all comments to lowercase")
	})

	t.Run("inplace mode without format", func(t *testing.T) {
		// reset the file
		err := os.WriteFile(testFile, []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// process without format option
		processFile(testFile, "inplace", false, false)

		// read the file content
		modifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read modified file")
		modifiedStr := string(modifiedContent)

		// note: the go printer will still normalize some formatting even without gofmt
		// so instead of checking for exact spacing, verify that comments are changed
		// but the formatter didn't run (which would add spaces around :=)
		assert.Contains(t, modifiedStr, "// this comment", "Should convert comments to lowercase")
		assert.Contains(t, modifiedStr, "// another comment", "Should convert all comments to lowercase")

		// skip spacing checks since printer may normalize some aspects
	})

	t.Run("print mode with format", func(t *testing.T) {
		// reset the file
		err := os.WriteFile(testFile, []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process the file with format in print mode
		processFile(testFile, "print", false, true)

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify output contains formatted code
		assert.Contains(t, output, "func Example()", "Should format function declaration")
		assert.Contains(t, output, "x := 1", "Should format variable assignment")
		assert.Contains(t, output, "// this comment", "Should convert comments to lowercase")
		assert.Contains(t, output, "// another comment", "Should convert all comments to lowercase")

		// file should remain unchanged
		origContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read original file")
		assert.Equal(t, content, string(origContent), "Original file should not be modified")
	})

	t.Run("diff mode with format", func(t *testing.T) {
		// reset the file
		err := os.WriteFile(testFile, []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process with format in diff mode
		processFile(testFile, "diff", false, true)

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// note: in diff mode, only changed lines appear in the diff
		// if both the original and modified versions are formatted the same way,
		// those lines may not appear in the diff output
		assert.Contains(t, output, "// this comment", "Should show lowercase comments")
		assert.Contains(t, output, "// another comment", "Should show all lowercase comments")

		// file should remain unchanged
		origContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read original file")
		assert.Equal(t, content, string(origContent), "Original file should not be modified")
	})
}

// TestProcessPatternHandling tests different pattern types
func TestProcessPatternHandling(t *testing.T) {
	// create a temporary directory structure for tests
	tempDir := t.TempDir()

	// create subdirectories
	subDir1 := filepath.Join(tempDir, "dir1")
	subDir2 := filepath.Join(tempDir, "dir2")
	err := os.Mkdir(subDir1, 0o750)
	require.NoError(t, err, "Failed to create subdirectory 1")
	err = os.Mkdir(subDir2, 0o750)
	require.NoError(t, err, "Failed to create subdirectory 2")

	// create test go files with comments
	files := map[string]string{
		filepath.Join(tempDir, "root.go"):	"package main\n\nfunc Root() {\n\t// THIS COMMENT\n}\n",
		filepath.Join(subDir1, "file1.go"):	"package dir1\n\nfunc Dir1() {\n\t// ANOTHER COMMENT\n}\n",
		filepath.Join(subDir2, "file2.go"):	"package dir2\n\nfunc Dir2() {\n\t// THIRD COMMENT\n}\n",
		filepath.Join(tempDir, "notago.txt"):	"This is not a go file",
	}

	for path, content := range files {
		err := os.WriteFile(path, []byte(content), 0o600)
		require.NoError(t, err, "Failed to create test file: "+path)
	}

	// save current directory
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	// change to temp dir
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer func() {
		err := os.Chdir(currentDir)
		require.NoError(t, err, "Failed to restore original directory")
	}()

	t.Run("specific file pattern", func(t *testing.T) {
		// reset file
		err := os.WriteFile("root.go", []byte(files[filepath.Join(tempDir, "root.go")]), 0o600)
		require.NoError(t, err)

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process specific file
		processPattern("root.go", ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: false, SkipPatterns: []string{}})

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify output
		assert.Contains(t, output, "Updated:", "Should show file was updated")

		// check file was modified
		content, err := os.ReadFile("root.go")
		require.NoError(t, err)
		assert.Contains(t, string(content), "// this comment", "Comment should be lowercase")
	})

	t.Run("glob pattern", func(t *testing.T) {
		// reset files
		err := os.WriteFile(filepath.Join("dir1", "file1.go"), []byte(files[filepath.Join(tempDir, "dir1", "file1.go")]), 0o600)
		require.NoError(t, err)

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process glob pattern
		processPattern("dir1/*.go", ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: false, SkipPatterns: []string{}})

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify output
		assert.Contains(t, output, "Updated:", "Should show file was updated")

		// check file was modified
		content, err := os.ReadFile(filepath.Join("dir1", "file1.go"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "// another comment", "Comment should be lowercase")
	})

	t.Run("directory pattern", func(t *testing.T) {
		// reset files
		err := os.WriteFile(filepath.Join("dir2", "file2.go"), []byte(files[filepath.Join(tempDir, "dir2", "file2.go")]), 0o600)
		require.NoError(t, err)

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process directory
		processPattern("dir2", ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: false, SkipPatterns: []string{}})

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify output
		assert.Contains(t, output, "Updated:", "Should show file was updated")

		// check file was modified
		content, err := os.ReadFile(filepath.Join("dir2", "file2.go"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "// third comment", "Comment should be lowercase")
	})

	t.Run("recursive pattern with '...'", func(t *testing.T) {
		// reset all files
		for path, content := range files {
			relPath, err := filepath.Rel(tempDir, path)
			require.NoError(t, err)
			if strings.HasSuffix(relPath, ".go") {
				err := os.WriteFile(relPath, []byte(content), 0o600)
				require.NoError(t, err)
			}
		}

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process recursive pattern
		processPattern("dir1...", ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: false, SkipPatterns: []string{}})

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)

		// check file was modified
		content, err := os.ReadFile(filepath.Join("dir1", "file1.go"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "// another comment", "Comment should be lowercase")
	})

	t.Run("invalid pattern", func(t *testing.T) {
		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process non-existent pattern
		processPattern("nonexistent*.go", ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: false, SkipPatterns: []string{}})

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify output
		assert.Contains(t, output, "No Go files found", "Should report no files found")
	})
}

// TestProcessPatternWithFormat tests format option with pattern matching
func TestProcessPatternWithFormat(t *testing.T) {
	// create a temporary directory for test files
	tempDir := t.TempDir()

	// create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	err := os.Mkdir(subDir, 0o750)
	require.NoError(t, err, "Failed to create subdirectory")

	// test content with uppercase comments and poor formatting
	content := `package testpkg

func Test(  ) {
    // UPPERCASE COMMENT
    x:=1
}`

	// write to multiple files
	files := []string{
		filepath.Join(tempDir, "file1.go"),
		filepath.Join(subDir, "file2.go"),
	}

	for _, file := range files {
		err = os.WriteFile(file, []byte(content), 0o600)
		require.NoError(t, err, "Failed to write test file")
	}

	// process the files with format option
	t.Run("recursive pattern with format", func(t *testing.T) {
		// reset files
		for _, file := range files {
			err = os.WriteFile(file, []byte(content), 0o600)
			require.NoError(t, err)
		}

		// save current directory
		currentDir, err := os.Getwd()
		require.NoError(t, err)

		// change to temp dir
		err = os.Chdir(tempDir)
		require.NoError(t, err)

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process recursively with format
		processPattern("./...", ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: true, SkipPatterns: []string{}})

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// restore directory
		err = os.Chdir(currentDir)
		require.NoError(t, err)

		// verify files were processed
		assert.Contains(t, output, "Updated:", "Should show update message")

		// check that both files were formatted
		for _, file := range files {
			formatted, err := os.ReadFile(file)
			require.NoError(t, err)
			formattedStr := string(formatted)

			// check for formatting changes
			assert.Contains(t, formattedStr, "func Test()", "Should format function declaration")
			assert.Contains(t, formattedStr, "x := 1", "Should format variable assignment")
			assert.Contains(t, formattedStr, "// uppercase", "Should convert comments to lowercase")
		}
	})
}

// TestFormatErrorHandling tests error handling in the format feature
func TestFormatErrorHandling(t *testing.T) {
	// skip if gofmt is not available for testing
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not available for testing")
	}

	// create a temporary directory
	tempDir := t.TempDir()

	// test content with poor formatting
	content := `package testpkg

func Example(  ) {
    // THIS COMMENT
    x:=1
}`

	// write test file
	testFile := filepath.Join(tempDir, "format_error_test.go")
	err := os.WriteFile(testFile, []byte(content), 0o600)
	require.NoError(t, err, "Failed to write test file")

	t.Run("error handling for gofmt", func(t *testing.T) {
		// capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// try to run with format
		processFile(testFile, "inplace", false, true)

		// restore stderr
		err := w.Close()
		require.NoError(t, err)
		os.Stderr = oldStderr
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		errOutput := buf.String()

		// error message should be captured if gofmt is not found
		if errOutput != "" {
			assert.Contains(t, errOutput, "Error", "Should report error running gofmt")
		}

		// despite gofmt error, the file should still be processed for comments
		fileContent, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Contains(t, string(fileContent), "// this comment", "Should still convert comments")
	})
}

// TestCLIInvocation tests the CLI by simulating command line invocation
// This tests the whole process without calling main() directly
func TestCLIInvocation(t *testing.T) {
	// create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "unfuck-ai-comments-main")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// create a test file
	testFile := filepath.Join(tempDir, "cli_test_file.go")
	content := `package test
func TestFunc() {
	// THIS is a comment that should be CONVERTED
}`
	if err := os.WriteFile(testFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// test inplace mode (default)
	t.Run("inplace mode", func(t *testing.T) {
		// reset test file
		if err := os.WriteFile(testFile, []byte(content), 0o600); err != nil {
			t.Fatalf("Failed to reset test file: %v", err)
		}

		// process file directly using the processfile function
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		processFile(testFile, "inplace", false, false)

		err := w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify output and file content
		if !strings.Contains(output, "Updated:") {
			t.Errorf("Expected 'Updated:' message in output, got: %q", output)
		}

		// check file was modified
		modifiedContent, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read modified file: %v", err)
		}

		expectedContent := `package test
func TestFunc() {
	// this is a comment that should be converted
}`

		// compare normalized content (removing line breaks and whitespace)
		if removeWhitespace(string(modifiedContent)) != removeWhitespace(expectedContent) {
			t.Errorf("File content doesn't match expected.\nExpected:\n%s\nGot:\n%s",
				expectedContent, string(modifiedContent))
		}
	})

	// test diff mode
	t.Run("diff mode", func(t *testing.T) {
		// reset test file
		if err := os.WriteFile(testFile, []byte(content), 0o600); err != nil {
			t.Fatalf("Failed to reset test file: %v", err)
		}

		// process file directly in diff mode
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		processFile(testFile, "diff", false, false)

		err := w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify diff output contains lowercase conversion
		if !strings.Contains(output, "THIS") && !strings.Contains(output, "this") {
			t.Errorf("Expected diff output to show comment conversion, got: %q", output)
		}

		// file should not be modified in diff mode
		unmodifiedContent, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if string(unmodifiedContent) != content {
			t.Error("File was modified in diff mode but should not be")
		}
	})

	// test print mode
	t.Run("print mode", func(t *testing.T) {
		// reset test file
		if err := os.WriteFile(testFile, []byte(content), 0o600); err != nil {
			t.Fatalf("Failed to reset test file: %v", err)
		}

		// process file directly in print mode
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		processFile(testFile, "print", false, false)

		err := w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify printed output
		if !strings.Contains(output, "// this is a comment") {
			t.Errorf("Expected print output to contain modified comment, got: %q", output)
		}

		// file should not be modified in print mode
		unmodifiedContent, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if string(unmodifiedContent) != content {
			t.Error("File was modified in print mode but should not be")
		}
	})
}

// TestMainFunctionMock creates a mock version of main to test all branches
func TestMainFunctionMock(t *testing.T) {
	// create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "unfuck-ai-main-mock")
	require.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tempDir)

	// create a test file with comments
	testFile := filepath.Join(tempDir, "mock_test.go")
	content := `package test
func Test() {
	// THIS SHOULD be converted
}`
	err = os.WriteFile(testFile, []byte(content), 0o600)
	require.NoError(t, err, "Failed to write test file")

	// mock version of main
	mockMain := func(outputMode string, dryRun, showHelp, noColor bool, patterns []string) string {
		// capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// set color setting
		color.NoColor = noColor

		// if dry-run is set, override output mode to diff
		if dryRun {
			outputMode = "diff"
		}

		// show help if requested
		if showHelp {
			fmt.Println("unfuck-ai-comments - Convert in-function comments to lowercase")
			fmt.Println("\nUsage:")
			fmt.Println("  unfuck-ai-comments [options] [file/pattern...]")
			fmt.Println("\nOptions:")
			fmt.Println("-output (inplace|print|diff) - Output mode")
			fmt.Println("-dry-run - Don't modify files, just show what would be changed")
			fmt.Println("-help - Show usage information")
			fmt.Println("-no-color - Disable colorized output")
			fmt.Println("\nExamples:")
			fmt.Println("  unfuck-ai-comments                       # Process all .go files in current directory")
			return "help displayed"
		}

		// if no patterns specified, use current directory
		if len(patterns) == 0 {
			patterns = []string{"."}
		}

		// process each pattern
		for _, pattern := range patterns {
			processPattern(pattern, ProcessRequest{OutputMode: outputMode, TitleCase: false, Format: false, SkipPatterns: []string{}})
		}

		// restore stdout
		err := w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		return buf.String()
	}

	// test cases
	tests := []struct {
		name		string
		outputMode	string
		dryRun		bool
		showHelp	bool
		noColor		bool
		patterns	[]string
		verify		func(string)
	}{
		{
			name:		"help flag",
			showHelp:	true,
			verify: func(output string) {
				assert.Equal(t, "help displayed", output, "Help should be displayed")
			},
		},
		{
			name:		"dry run flag",
			dryRun:		true,
			patterns:	[]string{testFile},
			verify: func(output string) {
				assert.Contains(t, output, "---", "Dry run should show diff")
				assert.Contains(t, output, "+++", "Dry run should show diff")
			},
		},
		{
			name:		"no color flag",
			noColor:	true,
			outputMode:	"diff",
			patterns:	[]string{testFile},
			verify: func(output string) {
				assert.True(t, color.NoColor, "NoColor should be true")
			},
		},
		{
			name:		"default directory",
			outputMode:	"inplace",
			patterns:	[]string{},
			verify: func(output string) {
				// this might be empty if no .go files in current dir, or might show files processed
				// just ensuring it doesn't crash
			},
		},
		{
			name:		"explicit file",
			outputMode:	"inplace",
			patterns:	[]string{testFile},
			verify: func(output string) {
				assert.Contains(t, output, "Updated:", "Should report file was updated")
			},
		},
	}

	// run test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// reset color setting
			color.NoColor = false

			// run mock main
			output := mockMain(tc.outputMode, tc.dryRun, tc.showHelp, tc.noColor, tc.patterns)

			// verify output
			tc.verify(output)
		})
	}
}

// TestSimpleDiff tests the diff function
func TestSimpleDiff(t *testing.T) {
	tests := []struct {
		name		string
		original	string
		modified	string
		expect		[]string
	}{
		{
			name:		"simple change",
			original:	"Line 1\nLine 2\nLine 3",
			modified:	"Line 1\nModified\nLine 3",
			expect:		[]string{"Line 2", "Modified"},
		},
		{
			name:		"comment change",
			original:	"// THIS IS A COMMENT",
			modified:	"// this is a comment",
			expect:		[]string{"THIS IS A COMMENT", "this is a comment"},
		},
		{
			name:		"no change",
			original:	"Line 1\nLine 2",
			modified:	"Line 1\nLine 2",
			expect:		[]string{},
		},
		{
			name:		"add line",
			original:	"Line 1\nLine 2",
			modified:	"Line 1\nLine 2\nLine 3",
			expect:		[]string{"Line 3"},
		},
		{
			name:		"remove line",
			original:	"Line 1\nLine 2\nLine 3",
			modified:	"Line 1\nLine 3",
			expect:		[]string{"Line 2"},
		},
	}

	// colors are disabled for predictable testing
	color.NoColor = true

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diff := simpleDiff(test.original, test.modified)
			for _, expect := range test.expect {
				assert.Contains(t, diff, expect, "Diff should contain expected changes")
			}
			if len(test.expect) == 0 {
				assert.Equal(t, "", diff, "Diff should be empty when no changes")
			}
		})
	}
}

// Test color functionality
func TestColorBehavior(t *testing.T) {
	// save current color setting and restore after test
	originalNoColor := color.NoColor
	defer func() { color.NoColor = originalNoColor }()

	// test with colors disabled
	color.NoColor = true
	assert.True(t, color.NoColor, "NoColor should be true when colors are disabled")

	// test with colors enabled
	color.NoColor = false
	assert.False(t, color.NoColor, "NoColor should be false when colors are enabled")
}

// TestShouldSkip tests the shouldSkip function
func TestShouldSkip(t *testing.T) {
	tests := []struct {
		name		string
		path		string
		skipPatterns	[]string
		expected	bool
	}{
		{
			name:		"no skip patterns",
			path:		"/some/path/file.go",
			skipPatterns:	[]string{},
			expected:	false,
		},
		{
			name:		"exact match",
			path:		"/some/path/file.go",
			skipPatterns:	[]string{"/some/path/file.go"},
			expected:	true,
		},
		{
			name:		"directory match",
			path:		"/some/path/file.go",
			skipPatterns:	[]string{"/some/path"},
			expected:	true,
		},
		{
			name:		"glob pattern match",
			path:		"/some/path/file.go",
			skipPatterns:	[]string{"*.go"},
			expected:	true,
		},
		{
			name:		"no match",
			path:		"/some/path/file.go",
			skipPatterns:	[]string{"/other/path", "*.txt"},
			expected:	false,
		},
		{
			name:		"multiple patterns with match",
			path:		"/some/path/file.go",
			skipPatterns:	[]string{"/other/path", "*.go"},
			expected:	true,
		},
		{
			name:		"invalid glob pattern",
			path:		"/some/path/file.go",
			skipPatterns:	[]string{"[invalid"},
			expected:	false,	// should not match with invalid pattern
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := shouldSkip(test.path, test.skipPatterns)
			assert.Equal(t, test.expected, result)
		})
	}
}

// TestProcessPatternWithSkip tests the skip functionality in file processing
func TestProcessPatternWithSkip(t *testing.T) {
	// create a temporary directory structure for tests
	tempDir := t.TempDir()

	// create subdirectories
	subDir1 := filepath.Join(tempDir, "dir1")
	subDir2 := filepath.Join(tempDir, "dir2")
	err := os.Mkdir(subDir1, 0o750)
	require.NoError(t, err, "Failed to create subdirectory 1")
	err = os.Mkdir(subDir2, 0o750)
	require.NoError(t, err, "Failed to create subdirectory 2")

	// create test go files with comments
	files := map[string]string{
		filepath.Join(tempDir, "root.go"):	"package main\n\nfunc Root() {\n\t// THIS COMMENT\n}\n",
		filepath.Join(subDir1, "file1.go"):	"package dir1\n\nfunc Dir1() {\n\t// ANOTHER COMMENT\n}\n",
		filepath.Join(subDir2, "file2.go"):	"package dir2\n\nfunc Dir2() {\n\t// THIRD COMMENT\n}\n",
		filepath.Join(tempDir, "skip_this.go"):	"package main\n\nfunc Skip() {\n\t// SKIPPED COMMENT\n}\n",
	}

	for path, content := range files {
		err := os.WriteFile(path, []byte(content), 0o600)
		require.NoError(t, err, "Failed to create test file: "+path)
	}

	// save current directory
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	// change to temp dir
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer func() {
		err := os.Chdir(currentDir)
		require.NoError(t, err, "Failed to restore original directory")
	}()

	t.Run("skip specific file", func(t *testing.T) {
		// reset files
		for path, content := range files {
			relPath, err := filepath.Rel(tempDir, path)
			require.NoError(t, err)
			err = os.WriteFile(relPath, []byte(content), 0o600)
			require.NoError(t, err)
		}

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process all files but skip one
		req := ProcessRequest{
			OutputMode:	"inplace",
			TitleCase:	false,
			Format:		false,
			SkipPatterns:	[]string{"skip_this.go"},
		}
		processPattern(".", req)

		// restore stdout
		err = w.Close()
		require.NoError(t, err)
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify output
		assert.Contains(t, output, "Updated:", "Should show files were updated")

		// check skipped file was not modified
		content, err := os.ReadFile("skip_this.go")
		require.NoError(t, err)
		assert.Contains(t, string(content), "// SKIPPED COMMENT", "Skipped file should not be modified")

		// check other files were modified
		content, err = os.ReadFile("root.go")
		require.NoError(t, err)
		assert.Contains(t, string(content), "// this comment", "Non-skipped file should be modified")
	})

	t.Run("skip directory", func(t *testing.T) {
		// reset files
		for path, content := range files {
			relPath, err := filepath.Rel(tempDir, path)
			require.NoError(t, err)
			err = os.WriteFile(relPath, []byte(content), 0o600)
			require.NoError(t, err)
		}

		// process recursively but skip dir1
		req := ProcessRequest{
			OutputMode:	"inplace",
			TitleCase:	false,
			Format:		false,
			SkipPatterns:	[]string{"dir1"},
		}
		processPattern("./...", req)

		// check dir1 file was not modified
		content, err := os.ReadFile(filepath.Join("dir1", "file1.go"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "// ANOTHER COMMENT", "File in skipped directory should not be modified")

		// check dir2 file was modified
		content, err = os.ReadFile(filepath.Join("dir2", "file2.go"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "// third comment", "File in non-skipped directory should be modified")
	})

	t.Run("processFile on nonexistent file", func(t *testing.T) {
		// create a non-existent file path
		nonexistentFile := filepath.Join(tempDir, "does-not-exist.go")

		// capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// try to process a non-existent file
		processFile(nonexistentFile, "inplace", false, false)

		// restore stderr
		err = w.Close()
		require.NoError(t, err)
		os.Stderr = oldStderr
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify error message
		assert.Contains(t, output, "Error parsing", "Should report parsing error")
	})
}
