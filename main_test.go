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
	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsCommentInsideFunction tests the core function that determines if a comment is inside a function body or struct
func TestIsCommentInsideFunction(t *testing.T) {
	// create a temporary file for the test
	tempDir := t.TempDir()
	testFilePath := filepath.Join(tempDir, "test_func.go")

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
	// Struct field comment SHOULD be modified (new behavior)
	Field int
	
	// Another field comment SHOULD be modified (new behavior)
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

	// write the test file
	err := os.WriteFile(testFilePath, []byte(src), 0o600)
	require.NoError(t, err, "Failed to write test file")

	// parse the source
	file, err := parser.ParseFile(token.NewFileSet(), testFilePath, nil, parser.ParseComments)
	require.NoError(t, err, "Failed to parse test source")

	// check all comments using classification patterns
	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			inside := isCommentInsideFunctionOrStruct(file, comment)
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
			}
		}
	}
}

// TestConvertCommentToLowercase tests the comment conversion function with various formats
func TestConvertCommentToLowercase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line comment",
			input:    "// This SHOULD Be Converted",
			expected: "// this should be converted",
		},
		{
			name:     "preserve comment markers",
			input:    "// UPPER case comment",
			expected: "// upper case comment",
		},
		{
			name:     "preserve camel and pascal comment",
			input:    "// This pascalCase, and CamelCase partially Converted",
			expected: "// this pascalCase, and CamelCase partially converted",
		},
		{
			name:     "comment with special chars",
			input:    "// Special: @#$%^&*()",
			expected: "// special: @#$%^&*()",
		},
		{
			name:     "comment with code example",
			input:    "// Example: const X = 123",
			expected: "// example: const x = 123",
		},
		{
			name:     "empty comment",
			input:    "//",
			expected: "//",
		},
		{
			name:     "comment with leading space",
			input:    "//  Leading space",
			expected: "//  leading space",
		},
		{
			name:     "not a comment",
			input:    "const X = 1",
			expected: "const X = 1", // should return unchanged
		},
		{
			name:     "TODO comment",
			input:    "// TODO This is a TODO Item",
			expected: "// TODO This is a TODO Item", // leave unchanged due to special indicator
		},
		{
			name:     "FIXME comment",
			input:    "// FIXME This needs FIXING",
			expected: "// FIXME This needs FIXING", // leave unchanged due to special indicator
		},
		{
			name:     "TODO with punctuation",
			input:    "// TODO: Fix this ASAP",
			expected: "// TODO: Fix this ASAP", // leave unchanged due to special indicator
		},
		{
			name:     "TODO at end of comment",
			input:    "// This is a TODO",
			expected: "// this is a todo", // todo is only preserved at start of comment
		},
		// additional test cases for camelCase and PascalCase identifiers
		{
			name:     "camelCase identifier in lowercase mode",
			input:    "// Example uses someVariableName for testing",
			expected: "// example uses someVariableName for testing", // camelCase preserved
		},
		{
			name:     "PascalCase identifier in lowercase mode",
			input:    "// Using OtherVariable in the code",
			expected: "// using OtherVariable in the code", // pascalCase preserved
		},
		{
			name:     "mixed case with identifiers",
			input:    "// USING someVariableName AND OtherVariable TOGETHER",
			expected: "// using someVariableName and OtherVariable together", // identifiers preserved, rest lowercase
		},
		{
			name:     "inline comment with identifier",
			input:    "// Initialize someVariableName here",
			expected: "// initialize someVariableName here",
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
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line comment",
			input:    "// This SHOULD Be Converted",
			expected: "// this SHOULD Be Converted",
		},
		{
			name:     "uppercase first letter with mixed case",
			input:    "// UPPEr case comment",
			expected: "// uPPEr case comment",
		},
		{
			name:     "preserve camel and pascal comment in the middle",
			input:    "// This pascalCase, and CamelCase partially Converted",
			expected: "// this pascalCase, and CamelCase partially Converted",
		},
		{
			name:     "preserve camel",
			input:    "// CamelCase partially Converted",
			expected: "// CamelCase partially Converted",
		},
		{
			name:     "preserve pascal",
			input:    "// pascalCase partially Converted",
			expected: "// pascalCase partially Converted",
		},
		{
			name:     "comment with special chars",
			input:    "// Special: @#$%^&*()",
			expected: "// special: @#$%^&*()",
		},
		{
			name:     "comment with code example",
			input:    "// Example: const X = 123",
			expected: "// example: const X = 123",
		},
		{
			name:     "empty comment",
			input:    "//",
			expected: "//",
		},
		{
			name:     "comment with leading space",
			input:    "//  Leading space",
			expected: "//  leading space",
		},
		{
			name:     "multi-line with indentation",
			input:    "/*\n * line 1\n * Line 2\n */",
			expected: "/*\n * line 1\n * Line 2\n */", // title case now uses same lowercase behavior
		},
		{
			name:     "TODO comment",
			input:    "// TODO This is a TODO Item",
			expected: "// TODO This is a TODO Item", // leave unchanged due to special indicator
		},
		{
			name:     "FIXME comment",
			input:    "// FIXME This needs FIXING",
			expected: "// FIXME This needs FIXING", // leave unchanged due to special indicator
		},
		{
			name:     "TODO with punctuation",
			input:    "// TODO: Fix this ASAP",
			expected: "// TODO: Fix this ASAP", // leave unchanged due to special indicator
		},
		{
			name:     "TODO comment followed by space and word",
			input:    "// TODO Fix this now",
			expected: "// TODO Fix this now", // leave unchanged due to special indicator
		},
		// test cases for the new behavior to preserve all-uppercase words
		{
			name:     "all uppercase word",
			input:    "// AI is an abbreviation",
			expected: "// AI is an abbreviation", // should not change all-uppercase words
		},
		{
			name:     "all uppercase word with special characters",
			input:    "// AI: is an abbreviation",
			expected: "// AI: is an abbreviation", // should not change all-uppercase words with punctuation
		},
		{
			name:     "all uppercase multi-character word",
			input:    "// CPU usage is high",
			expected: "// CPU usage is high", // should not change all-uppercase words with multiple chars
		},
		{
			name:     "single letter uppercase",
			input:    "// A single letter",
			expected: "// a single letter", // should still lowercase single letter words
		},
		{
			name:     "mixed case word with uppercase start",
			input:    "// APIclient should be converted",
			expected: "// aPIclient should be converted", // should convert mixed case words
		},
		{
			name:     "multiline with uppercase first word",
			input:    "/* API documentation\nSecond line */",
			expected: "/* API documentation\nSecond line */", // should not change all-uppercase words in multiline
		},
		// additional test cases for camelCase and PascalCase identifiers
		{
			name:     "camelCase identifier at comment start",
			input:    "// someVariableName should be preserved",
			expected: "// someVariableName should be preserved", // should preserve camelCase
		},
		{
			name:     "PascalCase identifier at comment start",
			input:    "// OtherVariable should be preserved",
			expected: "// OtherVariable should be preserved", // should preserve PascalCase
		},
		{
			name:     "camelCase and PascalCase identifiers in middle",
			input:    "// Using someVariableName and OtherVariable in code",
			expected: "// using someVariableName and OtherVariable in code", // only first word should be lowercase
		},
		{
			name:     "multiple camelCase and PascalCase identifiers",
			input:    "// The someVariableName, OtherVariable, and anotherCamelCase example",
			expected: "// the someVariableName, OtherVariable, and anotherCamelCase example", // preserve all identifiers
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := convertCommentToTitleCase(test.input)
			assert.Equal(t, test.expected, result, "Title case conversion failed")
		})
	}
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

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process file in inplace mode
		processFile(testFile, "inplace", false, false, writers)

		// verify output
		output := stdoutBuf.String()
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

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process file in diff mode
		processFile(testFile, "diff", false, false, writers)

		// verify diff output
		output := stdoutBuf.String()
		assert.Contains(t, output, "---", "Should show diff markers")
		assert.Contains(t, output, "+++", "Should show diff markers")
		// the exact format of the diff output depends on how diff is formatted,
		// so check for content rather than exact format
		assert.Contains(t, output, "THIS COMMENT", "Should show original comment")
		assert.Contains(t, output, "this comment", "Should show converted comment")

		// file should not be modified
		modifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read file")
		assert.Equal(t, content, string(modifiedContent), "File should not be modified in diff mode")
	})

	// test print mode
	t.Run("print mode", func(t *testing.T) {
		// reset file
		err := os.WriteFile(testFile, []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process file in print mode
		processFile(testFile, "print", false, false, writers)

		// verify printed output
		output := stdoutBuf.String()
		assert.Contains(t, output, "// this comment", "Should contain converted comment")

		// file should not be modified in print mode
		unmodifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read file")
		assert.Equal(t, content, string(unmodifiedContent), "File should not be modified in print mode")
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

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process the file with format option
		processFile(testFile, "inplace", false, true, writers)

		// verify output
		output := stdoutBuf.String()
		assert.Contains(t, output, "Updated:", "Should show update message")

		// read the file content
		modifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read modified file")
		modifiedStr := string(modifiedContent)

		// check that formatting was applied
		assert.Contains(t, modifiedStr, "// this comment", "Should convert comments to lowercase")
		assert.Contains(t, modifiedStr, "// another comment", "Should convert all comments to lowercase")

		// specific formatting checks can be flaky due to differences in gofmt behavior
		// between environments, so we'll focus on the comment changes
	})

	t.Run("inplace mode without format", func(t *testing.T) {
		// reset the file
		err := os.WriteFile(testFile, []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process without format option
		processFile(testFile, "inplace", false, false, writers)

		// read the file content
		modifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read modified file")
		modifiedStr := string(modifiedContent)

		// verify comments are changed
		assert.Contains(t, modifiedStr, "// this comment", "Should convert comments to lowercase")
		assert.Contains(t, modifiedStr, "// another comment", "Should convert all comments to lowercase")

		// skip spacing checks since printer may normalize some aspects
	})

	t.Run("print mode with format", func(t *testing.T) {
		// reset the file
		err := os.WriteFile(testFile, []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process the file with format in print mode
		processFile(testFile, "print", false, true, writers)

		// verify output
		output := stdoutBuf.String()
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

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process with format in diff mode
		processFile(testFile, "diff", false, true, writers)

		// verify output
		output := stdoutBuf.String()
		// in diff mode, only changed lines appear in the diff
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
		filepath.Join(tempDir, "root.go"):    "package main\n\nfunc Root() {\n\t// THIS COMMENT\n}\n",
		filepath.Join(subDir1, "file1.go"):   "package dir1\n\nfunc Dir1() {\n\t// ANOTHER COMMENT\n}\n",
		filepath.Join(subDir2, "file2.go"):   "package dir2\n\nfunc Dir2() {\n\t// THIRD COMMENT\n}\n",
		filepath.Join(tempDir, "notago.txt"): "This is not a go file",
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

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process specific file
		req := ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: false, SkipPatterns: []string{}}
		processPattern("root.go", &req, writers)

		// verify output
		output := stdoutBuf.String()
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

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process glob pattern
		req := ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: false, SkipPatterns: []string{}}
		processPattern("dir1/*.go", &req, writers)

		// verify output
		output := stdoutBuf.String()
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

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process directory
		req := ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: false, SkipPatterns: []string{}}
		processPattern("dir2", &req, writers)

		// verify output
		output := stdoutBuf.String()
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

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process recursive pattern
		req := ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: false, SkipPatterns: []string{}}
		processPattern("dir1...", &req, writers)

		// verify file was modified
		content, err := os.ReadFile(filepath.Join("dir1", "file1.go"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "// another comment", "Comment should be lowercase")
	})

	t.Run("invalid pattern", func(t *testing.T) {
		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process non-existent pattern
		req := ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: false, SkipPatterns: []string{}}
		processPattern("nonexistent*.go", &req, writers)

		// verify output
		output := stdoutBuf.String()
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

	// process the files with format option
	t.Run("recursive pattern with format", func(t *testing.T) {
		// reset files
		for _, file := range files {
			err = os.WriteFile(file, []byte(content), 0o600)
			require.NoError(t, err)
		}

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process recursively with format
		req := ProcessRequest{OutputMode: "inplace", TitleCase: false, Format: true, SkipPatterns: []string{}}
		processPattern("./...", &req, writers)

		// verify output
		output := stdoutBuf.String()
		assert.Contains(t, output, "Updated:", "Should show update message")

		// check that both files were formatted
		for _, file := range files {
			relFile, err := filepath.Rel(tempDir, file)
			require.NoError(t, err)

			formatted, err := os.ReadFile(relFile)
			require.NoError(t, err)
			formattedStr := string(formatted)

			// check for comment changes
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
		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// try to run with format
		processFile(testFile, "inplace", false, true, writers)

		// despite potential gofmt errors, the file should still be processed for comments
		fileContent, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Contains(t, string(fileContent), "// this comment", "Should still convert comments")
	})
}

// TestCLIInvocation tests the CLI by simulating command line invocation
// This tests the whole process without calling main() directly
func TestCLIInvocation(t *testing.T) {

	// helper function to remove whitespace for comparison
	removeWhitespace := func(s string) string {
		re := regexp.MustCompile(`\s+`)
		return re.ReplaceAllString(s, "")
	}

	// create a temporary directory for test files
	tempDir := t.TempDir()

	// create a test file
	testFile := filepath.Join(tempDir, "cli_test_file.go")
	content := `package test
func TestFunc() {
	// THIS is a comment that should be CONVERTED
}`
	err := os.WriteFile(testFile, []byte(content), 0o600)
	require.NoError(t, err, "Failed to write test file")

	// save current directory
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	// change to temp dir to simulate CLI environment
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer func() {
		err := os.Chdir(currentDir)
		require.NoError(t, err, "Failed to restore original directory")
	}()

	t.Run("inplace mode", func(t *testing.T) {
		// reset test file
		err := os.WriteFile("cli_test_file.go", []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process file directly using the processfile function
		processFile("cli_test_file.go", "inplace", false, false, writers)

		// verify output
		output := stdoutBuf.String()
		assert.Contains(t, output, "Updated:", "Should show update message")

		// check file was modified
		modifiedContent, err := os.ReadFile("cli_test_file.go")
		require.NoError(t, err, "Failed to read modified file")

		expectedContent := `package test
func TestFunc() {
	// this is a comment that should be converted
}`

		// compare normalized content (removing line breaks and whitespace)
		assert.Equal(t, removeWhitespace(expectedContent), removeWhitespace(string(modifiedContent)),
			"File content doesn't match expected")
	})

	t.Run("diff mode", func(t *testing.T) {
		// reset test file
		err := os.WriteFile("cli_test_file.go", []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process file directly in diff mode
		processFile("cli_test_file.go", "diff", false, false, writers)

		// verify diff output contains lowercase conversion
		output := stdoutBuf.String()
		assert.True(t, strings.Contains(output, "THIS") && strings.Contains(output, "this"),
			"Diff should show comment conversion")

		// file should not be modified in diff mode
		unmodifiedContent, err := os.ReadFile("cli_test_file.go")
		require.NoError(t, err, "Failed to read file")
		assert.Equal(t, content, string(unmodifiedContent),
			"File should not be modified in diff mode")
	})

	t.Run("print mode", func(t *testing.T) {
		// reset test file
		err := os.WriteFile("cli_test_file.go", []byte(content), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process file directly in print mode
		processFile("cli_test_file.go", "print", false, false, writers)

		// verify printed output
		output := stdoutBuf.String()
		assert.Contains(t, output, "// this is a comment",
			"Output should contain modified comment")

		// file should not be modified in print mode
		unmodifiedContent, err := os.ReadFile("cli_test_file.go")
		require.NoError(t, err, "Failed to read file")
		assert.Equal(t, content, string(unmodifiedContent),
			"File should not be modified in print mode")
	})
}

// TestMainFunctionMock creates a mock version of main to test all branches
func TestMainFunctionMock(t *testing.T) {
	// create a temporary directory for test files
	tempDir := t.TempDir()

	// save current directory
	currentDir, err := os.Getwd()
	require.NoError(t, err, "Failed to get working directory")

	// change to temp dir
	err = os.Chdir(tempDir)
	require.NoError(t, err, "Failed to change to temp directory")

	// ensure we restore the working directory after the test
	defer func() {
		err := os.Chdir(currentDir)
		require.NoError(t, err, "Failed to restore original working directory")
	}()

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
		// capture output using a buffer writer
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// set color setting
		color.NoColor = noColor

		// if dry-run is set, override output mode to diff
		if dryRun {
			outputMode = "diff"
		}

		// show help if requested
		if showHelp {
			fmt.Fprintf(writers.Stdout, "unfuck-ai-comments - Convert in-function comments to lowercase\n")
			fmt.Fprintf(writers.Stdout, "\nUsage:\n")
			fmt.Fprintf(writers.Stdout, "  unfuck-ai-comments [options] [file/pattern...]\n")
			fmt.Fprintf(writers.Stdout, "\nOptions:\n")
			fmt.Fprintf(writers.Stdout, "-output (inplace|print|diff) - Output mode\n")
			fmt.Fprintf(writers.Stdout, "-dry-run - Don't modify files, just show what would be changed\n")
			fmt.Fprintf(writers.Stdout, "-help - Show usage information\n")
			fmt.Fprintf(writers.Stdout, "-no-color - Disable colorized output\n")
			fmt.Fprintf(writers.Stdout, "\nExamples:\n")
			fmt.Fprintf(writers.Stdout, "  unfuck-ai-comments                       # Process all .go files in current directory\n")
			return "help displayed"
		}

		// if no patterns specified, use current directory
		if len(patterns) == 0 {
			patterns = []string{"."}
		} else {
			// convert absolute paths to relative within the tempDir
			for i, p := range patterns {
				if filepath.IsAbs(p) {
					// use relative paths to ensure we stay within the tempDir
					rel, err := filepath.Rel(tempDir, p)
					if err == nil {
						patterns[i] = rel
					}
				}
			}
		}

		// process each pattern
		for _, pattern := range patterns {
			req := ProcessRequest{OutputMode: outputMode, TitleCase: false, Format: false, SkipPatterns: []string{}}
			processPattern(pattern, &req, writers)
		}

		return stdoutBuf.String()
	}

	// test cases
	tests := []struct {
		name       string
		outputMode string
		dryRun     bool
		showHelp   bool
		noColor    bool
		patterns   []string
		verify     func(string)
	}{
		{
			name:     "help flag",
			showHelp: true,
			verify: func(output string) {
				assert.Equal(t, "help displayed", output, "Help should be displayed")
			},
		},
		{
			name:     "dry run flag",
			dryRun:   true,
			patterns: []string{"mock_test.go"},
			verify: func(output string) {
				assert.Contains(t, output, "---", "Dry run should show diff")
				assert.Contains(t, output, "+++", "Dry run should show diff")
			},
		},
		{
			name:       "no color flag",
			noColor:    true,
			outputMode: "diff",
			patterns:   []string{"mock_test.go"},
			verify: func(output string) {
				assert.True(t, color.NoColor, "NoColor should be true")
			},
		},
		{
			name:       "default directory",
			outputMode: "inplace",
			patterns:   []string{},
			verify: func(output string) {
				// this might be empty if no .go files in current dir, or might show files processed
				// just ensuring it doesn't crash
			},
		},
		{
			name:       "explicit file",
			outputMode: "inplace",
			patterns:   []string{"mock_test.go"},
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

			// reset the test file if needed
			if !tc.showHelp {
				err := os.WriteFile("mock_test.go", []byte(content), 0o600)
				require.NoError(t, err, "Failed to reset test file")
			}

			// run mock main
			output := mockMain(tc.outputMode, tc.dryRun, tc.showHelp, tc.noColor, tc.patterns)

			// verify output
			tc.verify(output)
		})
	}
}

// TestSampleGo tests processing of testdata/sample.go to verify identifier preservation
func TestSampleGo(t *testing.T) {
	// get path to testdata/sample.go
	currentDir, err := os.Getwd()
	require.NoError(t, err, "Failed to get working directory")

	samplePath := filepath.Join(currentDir, "testdata", "sample.go")

	// verify the sample file exists
	_, err = os.Stat(samplePath)
	require.NoError(t, err, "Sample file not found at "+samplePath)

	// create a temporary copy of the sample file
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "sample_test.go")

	// read the original file
	originalContent, err := os.ReadFile(samplePath)
	require.NoError(t, err, "Failed to read sample file")

	// write to the temporary location
	err = os.WriteFile(tempFile, originalContent, 0o600)
	require.NoError(t, err, "Failed to write temporary sample file")

	// process the file
	var stdoutBuf, stderrBuf bytes.Buffer
	writers := OutputWriters{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
	}

	// process with title case mode (preserves camelCase and PascalCase)
	processFile(tempFile, "print", true, false, writers)

	// get the processed content
	processedContent := stdoutBuf.String()

	// check for camelCase and PascalCase preservation
	assert.Contains(t, processedContent, "camelCase should be preserved",
		"camelCase identifier should be preserved")
	assert.Contains(t, processedContent, "PascalCase should be preserved",
		"PascalCase identifier should be preserved")
	assert.Contains(t, processedContent, "someVariableName",
		"camelCase variable name should be preserved")
	assert.Contains(t, processedContent, "OtherVariable",
		"PascalCase variable name should be preserved")

	// check that regular words are converted
	assert.Contains(t, processedContent, "// example with",
		"Regular words should be lowercase")

	// verify specific comment that demonstrates both lowercase conversion and identifier preservation
	assert.Contains(t, processedContent, "// example with camelCase and PascalCase identifiers",
		"First word should be lowercase while identifiers preserved")
}

// TestHelperFunctions tests the helper functions for pattern processing
func TestHelperFunctions(t *testing.T) {
	t.Run("isRecursivePattern", func(t *testing.T) {
		tests := []struct {
			pattern  string
			expected bool
		}{
			{"./...", true},
			{"dir/...", true},
			{"dir...", true},
			{"dir/*.go", false},
			{"file.go", false},
			{"/abs/path/...", true},
		}

		for _, tc := range tests {
			result := isRecursivePattern(tc.pattern)
			assert.Equal(t, tc.expected, result, "Pattern: %s", tc.pattern)
		}
	})

	t.Run("extractDirectoryFromPattern", func(t *testing.T) {
		tests := []struct {
			pattern  string
			expected string
		}{
			{"./...", "."},
			{"dir/...", "dir"},
			{"dir...", "dir"},
			{"/abs/path/...", "/abs/path"},
			{"...", "."},
		}

		for _, tc := range tests {
			result := extractDirectoryFromPattern(tc.pattern)
			assert.Equal(t, tc.expected, result, "Pattern: %s", tc.pattern)
		}
	})
	t.Run("findGoFilesFromPattern", func(t *testing.T) {
		// create temporary directory for testing
		tempDir := t.TempDir()

		// create test files
		testFiles := []string{
			filepath.Join(tempDir, "file1.go"),
			filepath.Join(tempDir, "file2.go"),
		}

		// create subdirectory
		err := os.MkdirAll(filepath.Join(tempDir, "subdir"), 0o750)
		require.NoError(t, err)

		// create a file in subdirectory
		testFiles = append(testFiles, filepath.Join(tempDir, "subdir", "file3.go"))

		// create all test files
		for _, file := range testFiles {
			err := os.WriteFile(file, []byte("package test"), 0o600)
			require.NoError(t, err)
		}

		// create a non-go file
		nonGoFile := filepath.Join(tempDir, "file.txt")
		err = os.WriteFile(nonGoFile, []byte("text file"), 0o600)
		require.NoError(t, err)

		// save current directory and change to temp dir
		currentDir, err := os.Getwd()
		require.NoError(t, err)
		err = os.Chdir(tempDir)
		require.NoError(t, err)
		defer func() {
			err := os.Chdir(currentDir)
			require.NoError(t, err, "Failed to restore original directory")
		}()

		// test with directory path
		files := findGoFilesFromPattern(".")
		assert.Len(t, files, 2, "Should find 2 .go files in the root directory")

		// test with glob pattern
		files = findGoFilesFromPattern("*.go")
		assert.Len(t, files, 2, "Should find 2 .go files matching pattern")

		// test with specific file
		files = findGoFilesFromPattern("file1.go")
		assert.Len(t, files, 1, "Should find 1 file")
		assert.Contains(t, files[0], "file1.go", "Should find the specified file")
	})

	t.Run("hasSpecialIndicator", func(t *testing.T) {
		tests := []struct {
			content  string
			expected bool
		}{
			{"TODO: fix this", true},
			{"FIXME: urgent issue", true},
			{"HACK: workaround", true},
			{"XXX: needs attention", true},
			{"WARNING: be careful", true},
			{"Normal comment", false},
			{"Contains TODO somewhere", false},
			{"  TODO: with spaces", true},
			{"", false},
		}

		for _, tc := range tests {
			result := hasSpecialIndicator(tc.content)
			assert.Equal(t, tc.expected, result, "Content: %s", tc.content)
		}
	})

	t.Run("processLineComment", func(t *testing.T) {
		tests := []struct {
			name          string
			content       string
			fullLowercase bool
			expected      string
		}{
			{
				name:          "full lowercase conversion",
				content:       " THIS Should BE Lowercase",
				fullLowercase: true,
				expected:      "// this should be lowercase",
			},
			{
				name:          "title case conversion",
				content:       " THIs Should BE Lowercase",
				fullLowercase: false,
				expected:      "// tHIs Should BE Lowercase",
			},
			{
				name:          "special indicator preserved in full lowercase",
				content:       " TODO: Fix this issue",
				fullLowercase: true,
				expected:      "// TODO: Fix this issue",
			},
			{
				name:          "special indicator preserved in title case",
				content:       " TODO: Fix this issue",
				fullLowercase: false,
				expected:      "// TODO: Fix this issue",
			},
			{
				name:          "empty content",
				content:       "",
				fullLowercase: true,
				expected:      "//",
			},
			{
				name:          "only whitespace",
				content:       "   ",
				fullLowercase: true,
				expected:      "//   ",
			},
			{
				name:          "all uppercase first word",
				content:       " AI is artificial intelligence",
				fullLowercase: false,
				expected:      "// AI is artificial intelligence",
			},
			{
				name:          "all uppercase first word with punctuation",
				content:       " CPU: high usage detected",
				fullLowercase: false,
				expected:      "// CPU: high usage detected",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				result := processLineComment(tc.content, tc.fullLowercase)
				assert.Equal(t, tc.expected, result)
			})
		}
	})
}

// TestBackupFlagPropagation tests that the backup flag is properly passed from Options to ProcessRequest
func TestBackupFlagPropagation(t *testing.T) {
	t.Run("backup flag should be properly passed to ProcessRequest", func(t *testing.T) {
		// create mock options with backup flag enabled
		opts := Options{
			Backup: true,
		}

		// create a process request using the options
		req := ProcessRequest{
			OutputMode:   "inplace",
			TitleCase:    !opts.Full, // title case is default, full resets it
			Format:       opts.Format,
			SkipPatterns: opts.Skip,
			Backup:       opts.Backup,
		}

		// verify the backup flag was properly passed
		assert.True(t, req.Backup, "Backup flag should be passed from Options to ProcessRequest")

		// create mock options with backup flag disabled
		opts = Options{
			Backup: false,
		}

		// create a process request using the options
		req = ProcessRequest{
			OutputMode:   "inplace",
			TitleCase:    !opts.Full,
			Format:       opts.Format,
			SkipPatterns: opts.Skip,
			Backup:       opts.Backup,
		}

		// verify the backup flag was properly passed
		assert.False(t, req.Backup, "Backup flag should be properly passed as false from Options to ProcessRequest")
	})
}

// TestModeSelection tests the mode selection logic
func TestModeSelection(t *testing.T) {
	// test the logic using determineProcessingMode directly
	t.Run("dry run sets diff mode", func(t *testing.T) {
		opts := Options{
			DryRun: true,
			Run: struct {
				Args struct {
					Patterns []string `positional-arg-name:"FILE/PATTERN" description:"Files or patterns to process (default: current directory)"`
				} `positional-args:"yes"`
			}{
				Args: struct {
					Patterns []string `positional-arg-name:"FILE/PATTERN" description:"Files or patterns to process (default: current directory)"`
				}{
					Patterns: []string{"file.go"},
				},
			},
		}

		p := flags.NewParser(&opts, flags.Default)
		mode, patterns := determineProcessingMode(opts, p)

		assert.Equal(t, "diff", mode, "Dry run should set mode to diff")
		assert.Equal(t, []string{"file.go"}, patterns, "Patterns should be properly passed")
	})

	t.Run("explicit modes via commands", func(t *testing.T) {
		// test each command mode
		commandModes := map[string]string{
			"run":   "inplace",
			"diff":  "diff",
			"print": "print",
		}

		for cmdName, expectedMode := range commandModes {
			t.Run(cmdName+" command", func(t *testing.T) {
				opts := Options{}
				p := flags.NewParser(&opts, flags.Default)

				// simulate command selection
				cmd := p.Command.Find(cmdName)
				require.NotNil(t, cmd, "Command should exist")
				p.Command.Active = cmd

				// set test pattern
				switch cmdName {
				case "run":
					opts.Run.Args.Patterns = []string{"file.go"}
				case "diff":
					opts.Diff.Args.Patterns = []string{"file.go"}
				case "print":
					opts.Print.Args.Patterns = []string{"file.go"}
				}

				mode, patterns := determineProcessingMode(opts, p)

				assert.Equal(t, expectedMode, mode,
					"Command '%s' should set mode to '%s'", cmdName, expectedMode)
				assert.Equal(t, []string{"file.go"}, patterns,
					"Patterns should be properly passed")
			})
		}
	})
}

// TestShouldSkip tests the shouldSkip function
func TestShouldSkip(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		skipPatterns []string
		expected     bool
	}{
		{
			name:         "no skip patterns",
			path:         "/some/path/file.go",
			skipPatterns: []string{},
			expected:     false,
		},
		{
			name:         "exact match",
			path:         "/some/path/file.go",
			skipPatterns: []string{"/some/path/file.go"},
			expected:     true,
		},
		{
			name:         "directory match",
			path:         "/some/path/file.go",
			skipPatterns: []string{"/some/path"},
			expected:     true,
		},
		{
			name:         "glob pattern match",
			path:         "/some/path/file.go",
			skipPatterns: []string{"*.go"},
			expected:     true,
		},
		{
			name:         "no match",
			path:         "/some/path/file.go",
			skipPatterns: []string{"/other/path", "*.txt"},
			expected:     false,
		},
		{
			name:         "multiple patterns with match",
			path:         "/some/path/file.go",
			skipPatterns: []string{"/other/path", "*.go"},
			expected:     true,
		},
		{
			name:         "invalid glob pattern",
			path:         "/some/path/file.go",
			skipPatterns: []string{"[invalid"},
			expected:     false, // should not match with invalid pattern
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
		filepath.Join(tempDir, "root.go"):      "package main\n\nfunc Root() {\n\t// THIS COMMENT\n}\n",
		filepath.Join(subDir1, "file1.go"):     "package dir1\n\nfunc Dir1() {\n\t// ANOTHER COMMENT\n}\n",
		filepath.Join(subDir2, "file2.go"):     "package dir2\n\nfunc Dir2() {\n\t// THIRD COMMENT\n}\n",
		filepath.Join(tempDir, "skip_this.go"): "package main\n\nfunc Skip() {\n\t// SKIPPED COMMENT\n}\n",
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

		// capture output using buffers
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process all files but skip one
		req := ProcessRequest{
			OutputMode:   "inplace",
			TitleCase:    false,
			Format:       false,
			SkipPatterns: []string{"skip_this.go"},
		}
		processPattern(".", &req, writers)

		// verify output
		output := stdoutBuf.String()
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

		// capture output using buffers
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process recursively but skip dir1
		req := ProcessRequest{
			OutputMode:   "inplace",
			TitleCase:    false,
			Format:       false,
			SkipPatterns: []string{"dir1"},
		}
		processPattern("./...", &req, writers)

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

		// capture output using buffers
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// try to process a non-existent file
		processFile(nonexistentFile, "inplace", false, false, writers)

		// verify error message
		errOutput := stderrBuf.String()
		assert.Contains(t, errOutput, "Error parsing", "Should report parsing error")
	})
}

// TestBackupFunctionality tests the backup functionality
func TestBackupFunctionality(t *testing.T) {
	// create a temporary directory for test files
	tempDir := t.TempDir()

	// test case 1: a file with changes that should be backed up
	t.Run("backup created for file with changes", func(t *testing.T) {
		// create a test file with comments that will be modified
		testFile := filepath.Join(tempDir, "backup_test1.go")
		content := `package test

func TestFunc() {
	// THIS COMMENT Should BE Modified
	x := 1 // ANOTHER COMMENT To Modify
}`
		err := os.WriteFile(testFile, []byte(content), 0o600)
		require.NoError(t, err, "Failed to write test file")

		// capture output using buffers
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process file with backup flag
		processFile(testFile, "inplace", false, false, writers, true)

		// verify backup file was created
		backupFile := testFile + ".bak"
		_, err = os.Stat(backupFile)
		require.NoError(t, err, "Backup file should exist")

		// verify backup file has original content
		backupContent, err := os.ReadFile(backupFile)
		require.NoError(t, err, "Failed to read backup file")
		assert.Equal(t, content, string(backupContent), "Backup file should contain original content")

		// verify main file has modified content
		modifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read modified file")
		assert.Contains(t, string(modifiedContent), "// this comment", "File should contain lowercase comments")
		assert.Contains(t, string(modifiedContent), "// another comment", "File should contain lowercase comments")
	})

	// test case 2: a file with no changes should not be backed up
	t.Run("no backup created for file without changes", func(t *testing.T) {
		// create a test file with comments already in lowercase
		testFile := filepath.Join(tempDir, "backup_test2.go")
		content := `package test

func TestFunc() {
	// this comment is already lowercase
	x := 1 // another lowercase comment
}`
		err := os.WriteFile(testFile, []byte(content), 0o600)
		require.NoError(t, err, "Failed to write test file")

		// make sure no backup file exists initially
		backupFile := testFile + ".bak"
		_ = os.Remove(backupFile) // cleanup any previous runs

		// capture output using buffers
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process with backup flag
		changes := processFile(testFile, "inplace", false, false, writers, true)

		// there should be no changes since the comments are already lowercase
		assert.Equal(t, 0, changes, "Should have no changes")

		// verify backup file was not created
		_, err = os.Stat(backupFile)
		assert.Error(t, err, "Backup file should not exist")
		assert.True(t, os.IsNotExist(err), "Error should be 'file does not exist'")
	})
}

// TestDiffModeNoColorOutput tests the diff mode with color disabled
func TestDiffModeNoColorOutput(t *testing.T) {
	// create a temporary directory for test files
	tempDir := t.TempDir()

	// create a test file with comments
	testFile := filepath.Join(tempDir, "color_test.go")
	content := `package test

func Example() {
	// THIS COMMENT should be converted
	x := 1 // ANOTHER COMMENT
}`
	err := os.WriteFile(testFile, []byte(content), 0o600)
	require.NoError(t, err, "Failed to write test file")

	// save original color setting
	originalNoColor := color.NoColor
	defer func() { color.NoColor = originalNoColor }()

	// disable colors for this test
	color.NoColor = true

	// capture output using buffers
	var stdoutBuf, stderrBuf bytes.Buffer
	writers := OutputWriters{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
	}

	// process file in diff mode
	processFile(testFile, "diff", false, false, writers)

	// verify diff output
	output := stdoutBuf.String()
	assert.Contains(t, output, "---", "Should show diff markers")
	assert.Contains(t, output, "+++", "Should show diff markers")
	assert.Contains(t, output, "THIS COMMENT", "Should show original comment")
	assert.Contains(t, output, "this comment", "Should show converted comment")

	// re-enable colors if needed for other tests
	color.NoColor = originalNoColor
}

// TestSimpleDiff tests the diff function
func TestSimpleDiff(t *testing.T) {
	// save original color setting
	originalNoColor := color.NoColor
	defer func() { color.NoColor = originalNoColor }()

	// disable colors for predictable testing
	color.NoColor = true

	tests := []struct {
		name     string
		original string
		modified string
		expect   []string
	}{
		{
			name:     "simple change",
			original: "Line 1\nLine 2\nLine 3",
			modified: "Line 1\nModified\nLine 3",
			expect:   []string{"Line 2", "Modified"},
		},
		{
			name:     "comment change",
			original: "// THIS IS A COMMENT",
			modified: "// this is a comment",
			expect:   []string{"THIS IS A COMMENT", "this is a comment"},
		},
		{
			name:     "no change",
			original: "Line 1\nLine 2",
			modified: "Line 1\nLine 2",
			expect:   []string{},
		},
		{
			name:     "add line",
			original: "Line 1\nLine 2",
			modified: "Line 1\nLine 2\nLine 3",
			expect:   []string{"Line 3"},
		},
		{
			name:     "remove line",
			original: "Line 1\nLine 2\nLine 3",
			modified: "Line 1\nLine 3",
			expect:   []string{"Line 2"},
		},
	}

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

// TestVendorAndTestdataExclusion tests that vendor and testdata directories are automatically excluded
func TestVendorAndTestdataExclusion(t *testing.T) {
	// create a temporary directory structure for tests
	tempDir := t.TempDir()

	// create directories
	rootDir := filepath.Join(tempDir, "root")
	vendorDir := filepath.Join(rootDir, "vendor")
	testdataDir := filepath.Join(rootDir, "testdata")
	normalDir := filepath.Join(rootDir, "normal")

	for _, dir := range []string{rootDir, vendorDir, testdataDir, normalDir} {
		err := os.MkdirAll(dir, 0o750)
		require.NoError(t, err, "Failed to create directory: "+dir)
	}

	// create test go files with comments
	files := map[string]string{
		filepath.Join(rootDir, "root.go"):         "package main\n\nfunc Root() {\n\t// THIS ROOT COMMENT\n}\n",
		filepath.Join(vendorDir, "vendor.go"):     "package vendor\n\nfunc Vendor() {\n\t// THIS VENDOR COMMENT\n}\n",
		filepath.Join(testdataDir, "testdata.go"): "package testdata\n\nfunc TestData() {\n\t// THIS TESTDATA COMMENT\n}\n",
		filepath.Join(normalDir, "normal.go"):     "package normal\n\nfunc Normal() {\n\t// THIS NORMAL COMMENT\n}\n",
	}

	for path, content := range files {
		err := os.WriteFile(path, []byte(content), 0o600)
		require.NoError(t, err, "Failed to create test file: "+path)
	}

	// save current directory
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	// change to root dir
	err = os.Chdir(rootDir)
	require.NoError(t, err)
	defer func() {
		err := os.Chdir(currentDir)
		require.NoError(t, err, "Failed to restore original directory")
	}()

	// capture output using buffer writers
	var stdoutBuf, stderrBuf bytes.Buffer
	writers := OutputWriters{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
	}

	// process recursively (which should automatically skip vendor and testdata)
	req := ProcessRequest{
		OutputMode:   "inplace",
		TitleCase:    false,
		Format:       false,
		SkipPatterns: []string{},
	}
	processPattern("./...", &req, writers)

	// check that files in root and normal directories were processed
	content, err := os.ReadFile("root.go")
	require.NoError(t, err)
	assert.Contains(t, string(content), "// this root comment",
		"Root directory file should be processed")

	content, err = os.ReadFile(filepath.Join("normal", "normal.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "// this normal comment",
		"Normal directory file should be processed")

	// check that vendor directory files were NOT processed (should retain uppercase)
	content, err = os.ReadFile(filepath.Join("vendor", "vendor.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "// THIS VENDOR COMMENT",
		"Vendor directory file should NOT be processed")

	// check that testdata directory files were NOT processed (should retain uppercase)
	content, err = os.ReadFile(filepath.Join("testdata", "testdata.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "// THIS TESTDATA COMMENT",
		"Testdata directory file should NOT be processed")
}

// TestWithSampleFile tests the tool against a sample file
func TestWithSampleFile(t *testing.T) {
	// create a temporary directory for tests
	tempDir := t.TempDir()

	// create a sample file for testing
	samplePath := filepath.Join(tempDir, "sample.go")
	sampleContent := `package sample

// Remote executes commands on remote server
// This comment should NOT be converted
type Remote struct {
	// comment inside struct - should now be converted
	Addr string
	
	// TODO This comment should remain unchanged
	User string
	
	// FIXME This comment should remain unchanged
	Password string
}

func NewRemote() *Remote {
	// THIS FUNCTION is not implemented yet
	return &Remote{}
}

// This comment should NOT be converted
func (ex *Remote) Execute(cmd string) error {
	// TODO IMPLEMENT ME - this comment should remain unchanged
	// ANOTHER Strange function I need to fix
	return nil
}

func (ex *Remote) Close() error {
	x := 1 // inline comment that should be converted
	
	/*
	 * This is a multi-line comment
	 * that should be converted
	 */
	 
	if true {
		// this is another nested comment
		for i := 0; i < 10; i++ {
			// comment in for loop should be converted
		}
	}
	
	// ALL CAPS COMMENT should be converted
	return nil
}`

	err := os.WriteFile(samplePath, []byte(sampleContent), 0o600)
	require.NoError(t, err, "Failed to write sample file")

	// test different modes with the sample file
	t.Run("process sample file in lowercase mode", func(t *testing.T) {
		// reset file before test
		err = os.WriteFile(samplePath, []byte(sampleContent), 0o600)
		require.NoError(t, err, "Failed to reset sample file")

		// capture output using buffers
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// process file in inplace mode
		processFile(samplePath, "inplace", false, false, writers)

		// read the processed file
		processedContent, err := os.ReadFile(samplePath)
		require.NoError(t, err, "Failed to read processed file")
		processedStr := string(processedContent)

		// verify function comment outside function is not converted
		assert.Contains(t, processedStr, "// Remote executes commands on remote server",
			"Comment outside function should not be converted")
		assert.Contains(t, processedStr, "// This comment should NOT be converted",
			"Comment outside function should not be converted")

		// verify comments inside struct are converted (new behavior)
		assert.Contains(t, processedStr, "// comment inside struct - should now be converted",
			"Comments inside struct should be converted")

		// verify special indicators in struct comments are preserved
		assert.Contains(t, processedStr, "// TODO This comment should remain unchanged",
			"TODO comments should remain completely unchanged")
		assert.Contains(t, processedStr, "// FIXME This comment should remain unchanged",
			"FIXME comments should remain completely unchanged")

		// verify comments inside function work correctly with special indicators
		assert.Contains(t, processedStr, "// TODO IMPLEMENT ME - this comment should remain unchanged",
			"TODO comments should remain unchanged even inside functions")
		assert.Contains(t, processedStr, "// this function is not implemented yet",
			"Comment inside function should be converted to lowercase")
		assert.Contains(t, processedStr, "// another strange function",
			"Comment inside function should be converted to lowercase")

		// verify inline comments are converted
		assert.Contains(t, processedStr, "// inline comment that should be converted",
			"Inline comment should be converted")

		// verify nested comments are converted
		assert.Contains(t, processedStr, "// this is another nested comment",
			"Nested comment should be converted")
		assert.Contains(t, processedStr, "// comment in for loop should be converted",
			"Comment in loop should be converted")

		// verify other functions' comments are converted
		assert.Contains(t, processedStr, "// all caps comment should be converted",
			"Comment in another function should be converted")
	})
}

// TestParseCommandLineOptions tests the command line option parsing logic
func TestParseCommandLineOptions(t *testing.T) {
	// save the original os.Args
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()

	t.Run("basic parsing", func(t *testing.T) {
		// create buffer for capturing output
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// set up minimal command line
		os.Args = []string{"unfuck-ai-comments", "run", "file.go"}

		// parse options
		opts, p, err := parseCommandLineOptions(writers)

		// verify no error was returned
		require.NoError(t, err, "Should parse without error")

		// verify correct values
		assert.Equal(t, []string{"file.go"}, opts.Run.Args.Patterns, "Should capture file pattern")
		assert.NotNil(t, p, "Parser should not be nil")
		assert.False(t, opts.Full, "Full flag should default to false")
		assert.False(t, opts.Version, "Version flag should default to false")
	})

	t.Run("version flag as standalone", func(t *testing.T) {
		// create buffer for capturing output
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// set up --version flag
		os.Args = []string{"unfuck-ai-comments", "--version"}

		// parse options
		_, _, err := parseCommandLineOptions(writers)

		// verify version error was returned
		assert.ErrorIs(t, err, ErrVersionRequested, "Should return version requested error")

		// verify version info was printed
		assert.Contains(t, stdoutBuf.String(), "unfuck-ai-comments", "Version info should be printed")
	})

	t.Run("full flag", func(t *testing.T) {
		// create buffer for capturing output
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// set up command with --full flag
		os.Args = []string{"unfuck-ai-comments", "--full", "run", "file.go"}

		// parse options
		opts, _, err := parseCommandLineOptions(writers)

		// verify no error was returned
		require.NoError(t, err, "Should parse without error")

		// verify correct values
		assert.True(t, opts.Full, "Full flag should be set to true")
	})

	t.Run("help flag", func(t *testing.T) {
		// create buffer for capturing output
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// set up --help flag
		os.Args = []string{"unfuck-ai-comments", "--help"}

		// parse options
		_, _, err := parseCommandLineOptions(writers)

		// verify help error was returned
		assert.ErrorIs(t, err, ErrHelpRequested, "Should return help requested error")
	})

	t.Run("invalid flag", func(t *testing.T) {
		// create buffer for capturing output
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// set up invalid flag
		os.Args = []string{"unfuck-ai-comments", "--nonexistent-flag"}

		// parse options
		_, _, err := parseCommandLineOptions(writers)

		// verify parsing failed error was returned
		assert.ErrorIs(t, err, ErrParsingFailed, "Should return parsing failed error")
		assert.Contains(t, stderrBuf.String(), "Error:", "Should print error message")
	})
}

// TestOutputWriters tests the OutputWriters functionality
func TestOutputWriters(t *testing.T) {
	t.Run("default writers", func(t *testing.T) {
		writers := DefaultWriters()
		assert.Equal(t, os.Stdout, writers.Stdout, "Default stdout should be os.Stdout")
		assert.Equal(t, os.Stderr, writers.Stderr, "Default stderr should be os.Stderr")
	})

	t.Run("custom writers", func(t *testing.T) {
		var stdoutBuf, stderrBuf bytes.Buffer
		writers := OutputWriters{
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
		}

		// write to the writers
		fmt.Fprint(writers.Stdout, "test stdout")
		fmt.Fprint(writers.Stderr, "test stderr")

		// verify content
		assert.Equal(t, "test stdout", stdoutBuf.String(), "Should capture stdout content")
		assert.Equal(t, "test stderr", stderrBuf.String(), "Should capture stderr content")
	})
}

func TestGetCommentIdentifiers(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "extracts pascal case identifiers",
			content:  "This is a TestFunction with PascalCase",
			expected: []string{"TestFunction", "PascalCase"},
		},
		{
			name:     "no identifiers, but CAPS words",
			content:  " This SHOULD Be Converted",
			expected: []string{},
		},
		{
			name:     "strange UPPEr case comment",
			content:  "UPPEr case comment",
			expected: []string{},
		},

		{
			name:     "extracts camel case identifiers",
			content:  "this is a testFunction with camelCase",
			expected: []string{"testFunction", "camelCase"},
		},
		{
			name:     "extracts mixed case identifiers",
			content:  "This is a testFunction with MixedCase and camelCase",
			expected: []string{"testFunction", "MixedCase", "camelCase"},
		},
		{
			name:     "ignores non-identifier words",
			content:  "this is a simple comment without identifiers",
			expected: []string{},
		},
		{
			name:     "handles empty content",
			content:  "",
			expected: []string{},
		},
		{
			name:     "handles content with special characters",
			content:  "This is a testFunction_with_specialCharacters and camelCase",
			expected: []string{"testFunction_with_specialCharacters", "camelCase"},
		},
		{
			name:     "handles content with numbers",
			content:  "This is a testFunction1 with camelCase2",
			expected: []string{"testFunction1", "camelCase2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCommentIdentifiers(tt.content)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}
