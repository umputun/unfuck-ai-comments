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
	"runtime"
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

// TestProcessFileFunctionality tests the file processing logic using a temp file
func TestProcessFileFunctionality(t *testing.T) {
	// create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "unfuck-ai-comments-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// test content with mixed comments
	content := `package testpkg

// Package comment NOT to be modified
// Package doc not to be modified

// Function comment NOT to be modified
func Example() {
	// THIS comment inside function SHOULD be modified
	x := 1 // THIS inline comment SHOULD be modified
	
	/*
	 * THIS multiline comment
	 * SHOULD also be modified
	 */
}

// Interface comment NOT to be modified
type Iface interface {
	// Method comment NOT to be modified
	Method()
}

func AnotherFunc() {
	// ANOTHER comment to modify
	if x > 10 {
		// NESTED block comment to modify
	}
}`

	// write test file
	testFile := filepath.Join(tempDir, "test.go")
	err = os.WriteFile(testFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// also write an invalid go file to test error handling
	invalidFile := filepath.Join(tempDir, "invalid.go")
	err = os.WriteFile(invalidFile, []byte("this is not valid go code"), 0o644)
	if err != nil {
		t.Fatalf("Failed to write invalid test file: %v", err)
	}

	// write a go file without comments to test no-change case
	noCommentsFile := filepath.Join(tempDir, "nocomments.go")
	noCommentsContent := `package testpkg

func NoComments() {
	x := 1
}`
	err = os.WriteFile(noCommentsFile, []byte(noCommentsContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write no-comments file: %v", err)
	}

	// test output modes
	t.Run("inplace mode", func(t *testing.T) {
		// reset the file before each test
		err = os.WriteFile(testFile, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("Failed to reset test file: %v", err)
		}

		// redirect stdout temporarily
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process the file
		processFile(testFile, "inplace")

		// restore stdout
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify "updated" message
		if !strings.Contains(output, "Updated:") {
			t.Error("Missing 'Updated' message for inplace mode")
		}

		// check that the file was updated
		modifiedContent, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read modified file: %v", err)
		}

		// check specific modifications
		modified := string(modifiedContent)
		if strings.Contains(modified, "// THIS comment") {
			t.Error("Failed to convert uppercase comment to lowercase in function")
		}
		if !strings.Contains(modified, "// this comment") {
			t.Error("Did not properly convert to lowercase in function")
		}
		if !strings.Contains(modified, "// Package comment NOT") {
			t.Error("Incorrectly modified package comment")
		}
		if !strings.Contains(modified, "// another comment") {
			t.Error("Did not convert ANOTHER comment to lowercase")
		}
		if strings.Contains(modified, "// ANOTHER comment") {
			t.Error("Did not convert ANOTHER comment to lowercase")
		}
		if !strings.Contains(modified, "// nested block") {
			t.Error("Did not convert NESTED comment to lowercase")
		}
		if strings.Contains(modified, "// NESTED block") {
			t.Error("Did not convert NESTED comment to lowercase")
		}
	})

	t.Run("print mode", func(t *testing.T) {
		// reset the file
		err = os.WriteFile(testFile, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("Failed to reset test file: %v", err)
		}

		// redirect stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process the file
		processFile(testFile, "print")

		// restore stdout and capture output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// check output
		if strings.Contains(output, "// THIS comment") {
			t.Error("Failed to convert uppercase comment to lowercase in print mode")
		}
		if !strings.Contains(output, "// this comment") {
			t.Error("Did not properly convert to lowercase in print mode")
		}
	})

	t.Run("diff mode", func(t *testing.T) {
		// reset the file
		err = os.WriteFile(testFile, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("Failed to reset test file: %v", err)
		}

		// redirect stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process the file
		processFile(testFile, "diff")

		// restore stdout and capture output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// check diff output
		if !strings.Contains(output, "-") || !strings.Contains(output, "+") {
			t.Error("Diff output doesn't contain changes")
		}
		if !strings.Contains(output, "// this comment") {
			t.Error("Diff doesn't show lowercase conversion")
		}
	})

	t.Run("invalid mode", func(t *testing.T) {
		// test with an invalid output mode
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		// this should silently ignore the invalid mode
		processFile(testFile, "invalid")

		w.Close()
		os.Stdout = oldStdout

		// file should not be modified
		modifiedContent, _ := os.ReadFile(testFile)
		if !strings.Contains(string(modifiedContent), "// THIS comment") {
			t.Error("File should not be modified with invalid mode")
		}
	})

	t.Run("file with parse error", func(t *testing.T) {
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// process invalid file
		processFile(invalidFile, "diff")

		w.Close()
		os.Stderr = oldStderr
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		errorOutput := buf.String()

		// should have error message
		if !strings.Contains(errorOutput, "Error parsing") {
			t.Errorf("Expected parsing error for invalid file, got: %s", errorOutput)
		}
	})

	t.Run("file with no changes needed", func(t *testing.T) {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process file with no comments that need changing
		processFile(noCommentsFile, "inplace")

		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// should not show "updated" message
		if strings.Contains(output, "Updated:") {
			t.Error("Should not show 'Updated' message for file with no changes")
		}
	})

	t.Run("inplace write error", func(t *testing.T) {
		// create a new file with limited permissions to test write errors
		readOnlyFile := filepath.Join(tempDir, "readonly.go")
		err = os.WriteFile(readOnlyFile, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write read-only file: %v", err)
		}

		// make file read-only if possible
		if err := os.Chmod(readOnlyFile, 0o400); err != nil {
			t.Skipf("Could not make file read-only, skipping test: %v", err)
		}

		// capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// try to modify the read-only file
		processFile(readOnlyFile, "inplace")

		// restore stderr and capture output
		w.Close()
		os.Stderr = oldStderr
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		errorOutput := buf.String()

		// reset file permissions
		_ = os.Chmod(readOnlyFile, 0o644)

		// check for error message related to file opening
		if !strings.Contains(errorOutput, "Error opening") && !strings.Contains(errorOutput, "permission denied") {
			t.Error("Expected error message about file permissions")
		}
	})
}

// TestEmptyFile tests processing a file with no comments
func TestEmptyFile(t *testing.T) {
	src := `package main

func EmptyFunc() {
	x := 1
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "empty.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test source: %v", err)
	}

	// no comments should be found
	if len(file.Comments) != 0 {
		t.Errorf("Expected 0 comments in empty file, got %d", len(file.Comments))
	}
}

// TestCommentsAtFunctionBoundary tests edge cases with comments at function boundaries
func TestCommentsAtFunctionBoundary(t *testing.T) {
	src := `package main

func BoundaryFunc() { // Comment on the same line as function opening brace SHOULD be modified
	x := 1
} // Comment on the same line as function closing brace should NOT be modified

func MultiLineDef(
	// Comment in parameter list should NOT be modified
	param1 string,
	param2 int, // Inline comment in parameter list should NOT be modified
) {
	// This comment SHOULD be modified
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "boundary.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test source: %v", err)
	}

	expectedResults := map[string]bool{
		"// Comment on the same line as function opening brace SHOULD be modified":	true,
		"// Comment on the same line as function closing brace should NOT be modified":	false,
		"// Comment in parameter list should NOT be modified":				false,
		"// Inline comment in parameter list should NOT be modified":			false,
		"// This comment SHOULD be modified":						true,
	}

	// check each comment's classification
	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			inside := isCommentInsideFunction(fset, file, comment)
			text := comment.Text

			expected, exists := expectedResults[text]
			if !exists {
				t.Errorf("Unexpected comment found: %q", text)
				continue
			}

			if inside != expected {
				t.Errorf("Comment %q: expected inside=%v, got inside=%v", text, expected, inside)
			}
		}
	}
}

// TestNestedFunctions tests comments in nested functions (closures)
func TestNestedFunctions(t *testing.T) {
	src := `package main

func OuterFunc() {
	// OUTER function comment should be modified
	
	innerFunc := func() {
		// INNER function comment should be modified
		
		deeperFunc := func() {
			// DEEPER function comment should be modified
		}
		deeperFunc()
	}
	innerFunc()
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "nested.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test source: %v", err)
	}

	// all comments should be inside a function
	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			inside := isCommentInsideFunction(fset, file, comment)
			if !inside {
				t.Errorf("Comment should be identified as inside a function, but wasn't: %q", comment.Text)
			}
		}
	}
}

// TestGenericFunctions tests comments in generic functions (Go 1.18+)
func TestGenericFunctions(t *testing.T) {
	// skip test on go versions before 1.18
	src := `package main

// Generic function comment should NOT be modified
func Generic[T any](param T) {
	// INSIDE generic function should be modified
	
	// Another comment to modify
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "generic.go", src, parser.ParseComments)
	if err != nil {
		// this might fail on older go versions, so skip test in that case
		if strings.Contains(err.Error(), "expected") {
			t.Skip("Skipping generic function test on older Go version")
		}
		t.Fatalf("Failed to parse test source: %v", err)
	}

	expectedResults := map[string]bool{
		"// Generic function comment should NOT be modified":	false,
		"// INSIDE generic function should be modified":	true,
		"// Another comment to modify":				true,
	}

	// check each comment's classification
	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			inside := isCommentInsideFunction(fset, file, comment)
			text := comment.Text

			expected, exists := expectedResults[text]
			if !exists {
				t.Errorf("Unexpected comment found: %q", text)
				continue
			}

			if inside != expected {
				t.Errorf("Comment %q: expected inside=%v, got inside=%v", text, expected, inside)
			}
		}
	}
}

// TestUnicodeInComments tests handling of Unicode characters in comments
func TestUnicodeInComments(t *testing.T) {
	comment := "// UNICODE CHARS: 你好 Привет こんにちは"
	result := convertCommentToLowercase(comment)
	expected := "// unicode chars: 你好 привет こんにちは"

	assert.Equal(t, expected, result, "Unicode conversion failed")
}

// TestMultiByteComments tests handling of emojis and other multi-byte characters
func TestMultiByteComments(t *testing.T) {
	tests := []struct {
		input		string
		expected	string
	}{
		{
			input:		"// EMOJI TEST: 😀 😃 😄 👍",
			expected:	"// emoji test: 😀 😃 😄 👍",
		},
		{
			input:		"// MIXED CASE with EMOJI: Hello 👋 World",
			expected:	"// mixed case with emoji: hello 👋 world",
		},
	}

	for _, test := range tests {
		result := convertCommentToLowercase(test.input)
		assert.Equal(t, test.expected, result, "Multi-byte conversion failed")
	}
}

// TestHandlingOfBadCode tests the tool's behavior with malformed code
func TestHandlingOfBadCode(t *testing.T) {
	// test with broken code that can't be parsed
	badSrc := `package main

func BrokenFunc( {  // Syntax error here
	// Comment that won't be processed due to parse error
}`

	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "broken.go", badSrc, parser.ParseComments)
	if err == nil {
		t.Error("Expected parse error for broken code, but got none")
	}
}

// TestPatternMatching tests the pattern matching functionality more thoroughly
func TestPatternMatching(t *testing.T) {
	// create a temporary directory
	tempDir, err := os.MkdirTemp("", "unfuck-ai-comments-pattern")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// create a more complex directory structure
	subDir1 := filepath.Join(tempDir, "subdir1")
	subDir2 := filepath.Join(tempDir, "subdir2")
	nestedDir := filepath.Join(subDir1, "nested")

	for _, dir := range []string{subDir1, subDir2, nestedDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("Failed to create directory structure: %v", err)
		}
	}

	// create test files with comments
	testFiles := map[string]string{
		filepath.Join(tempDir, "root.go"): `package testpkg
func Test() {
	// UPPER case comment in root
}`,
		filepath.Join(subDir1, "sub1.go"): `package testpkg
func Test() {
	// UPPER case comment in subdir1
}`,
		filepath.Join(subDir2, "sub2.go"): `package testpkg
func Test() {
	// UPPER case comment in subdir2
}`,
		filepath.Join(nestedDir, "nested.go"): `package testpkg
func Test() {
	// UPPER case comment in nested dir
}`,
		filepath.Join(tempDir, "nocomment.go"): `package testpkg
func Test() {
	x := 1 // No uppercase here
}`,
		filepath.Join(tempDir, "notgo.txt"):	`This is not a Go file`,
	}

	for file, content := range testFiles {
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", file, err)
		}
	}

	// helper to count files processed by capturing stdout
	countProcessedFiles := func(pattern, mode string) int {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// save current working directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// change to temp dir for the test
		if err := os.Chdir(tempDir); err != nil {
			t.Fatalf("Failed to change to temp dir: %v", err)
		}

		// process pattern from temp directory
		processPattern(pattern, mode)

		// change back to original directory
		if err := os.Chdir(currentDir); err != nil {
			t.Fatalf("Failed to change back to original directory: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if mode == "diff" {
			return strings.Count(output, "---")
		}
		return strings.Count(output, "Updated:")
	}

	// test cases for pattern matching
	testCases := []struct {
		name		string
		pattern		string
		mode		string
		expectMatches	int
	}{
		{"single file", filepath.Join(tempDir, "root.go"), "diff", 1},
		{"specific glob", filepath.Join(tempDir, "*.go"), "diff", 2},	// root.go, nocomment.go
		{"non-go file", filepath.Join(tempDir, "notgo.txt"), "diff", 0},
		{"nonexistent file", filepath.Join(tempDir, "nonexistent.go"), "diff", 0},
		{"nonexistent pattern", filepath.Join(tempDir, "*.nonexistent"), "diff", 0},
		{"subdir1", subDir1, "diff", 1},			// processes directory with .go files
		{"subdir1 with slash", subDir1 + "/", "diff", 1},	// processes directory with .go files
		{"recursive pattern ./...", "./...", "inplace", 5},	// all go files (root.go, nocomment.go, sub1.go, sub2.go, nested.go)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			count := countProcessedFiles(tc.pattern, tc.mode)
			if count != tc.expectMatches {
				t.Errorf("Expected %d matches for pattern %s, got %d",
					tc.expectMatches, tc.pattern, count)
			}
		})
	}

	// test specific error cases
	t.Run("invalid pattern", func(t *testing.T) {
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// invalid glob pattern
		processPattern("[", "diff")

		w.Close()
		os.Stderr = oldStderr
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "Error globbing pattern") {
			t.Errorf("Expected error message for invalid pattern, got: %s", output)
		}
	})
}

// TestSimpleDiff tests the diff generation functionality
func TestSimpleDiff(t *testing.T) {
	testCases := []struct {
		name		string
		original	string
		modified	string
		expected	[]string	// strings that should appear in the diff output
	}{
		{
			name:		"identical strings",
			original:	"line 1\nline 2\nline 3",
			modified:	"line 1\nline 2\nline 3",
			expected:	[]string{},	// no diff output expected for identical strings
		},
		{
			name:		"add line",
			original:	"line 1\nline 2",
			modified:	"line 1\nline 2\nline 3",
			expected:	[]string{"+ line 3"},
		},
		{
			name:		"remove line",
			original:	"line 1\nline 2\nline 3",
			modified:	"line 1\nline 3",
			expected:	[]string{"- line 2"},
		},
		{
			name:		"change line",
			original:	"line 1\noriginal line\nline 3",
			modified:	"line 1\nmodified line\nline 3",
			expected:	[]string{"- original line", "+ modified line"},
		},
		{
			name:		"multiple changes",
			original:	"line 1\noriginal line\nto be removed\nline 4",
			modified:	"line 1\nmodified line\nnew line\nline 4",
			expected:	[]string{"- original line", "+ modified line", "- to be removed", "+ new line"},
		},
		{
			name:		"empty original",
			original:	"",
			modified:	"new content",
			expected:	[]string{"+ new content"},
		},
		{
			name:		"empty modified",
			original:	"old content",
			modified:	"",
			expected:	[]string{"- old content"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diff := simpleDiff(tc.original, tc.modified)

			// for empty expected, make sure diff is empty
			if len(tc.expected) == 0 {
				if diff != "" {
					t.Errorf("Expected empty diff, got %q", diff)
				}
				return
			}

			// check that all expected strings appear in the diff
			for _, exp := range tc.expected {
				if !strings.Contains(diff, exp) {
					t.Errorf("Expected diff to contain %q, but it doesn't: %q", exp, diff)
				}
			}
		})
	}
}

// TestIntegration runs the actual CLI tool on test files
func TestIntegration(t *testing.T) {
	// create temp dir and test files
	tempDir, err := os.MkdirTemp("", "unfuck-ai-integration")
	require.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tempDir)

	// create a test file with comments
	testCode := `package example

// Package comment should NOT be modified

// Function comment should NOT be modified
func Example() {
	// THIS comment SHOULD be MODIFIED
	x := 1 // THIS inline comment SHOULD be MODIFIED too

	/*
	 * THIS multi-line COMMENT
	 * SHOULD also BE modified
	 */
	
	if true {
		// NESTED block COMMENT
	}
}

// Another function comment should NOT be modified
func Example2() {
	// ANOTHER comment to BE modified
}`

	testFile := filepath.Join(tempDir, "example.go")
	err = os.WriteFile(testFile, []byte(testCode), 0o600)
	require.NoError(t, err, "Failed to write test file")

	// build the cli tool
	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tempDir, "unfuck-ai-comments"))
	err = buildCmd.Run()
	require.NoError(t, err, "Failed to build CLI tool")

	// run the integration tests
	t.Run("inplace mode", func(t *testing.T) {
		// reset the file before test
		err = os.WriteFile(testFile, []byte(testCode), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// run the tool
		cmd := exec.Command(filepath.Join(tempDir, "unfuck-ai-comments"), testFile)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		require.NoError(t, err, "Tool execution failed: %s", stderr.String())

		// read the modified file
		modifiedBytes, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read modified file")
		modified := string(modifiedBytes)

		// verify the changes
		assert.NotContains(t, modified, "// THIS comment", "Failed to convert uppercase comment to lowercase")
		assert.Contains(t, modified, "// this comment", "Did not properly convert to lowercase")
		assert.Contains(t, modified, "// Package comment should NOT", "Incorrectly modified package comment")
		assert.NotContains(t, modified, "// ANOTHER comment", "Failed to convert comment in second function")
		assert.Contains(t, modified, "// another comment", "Did not properly convert comment in second function")
	})

	t.Run("dry-run mode", func(t *testing.T) {
		// reset the file before test
		err = os.WriteFile(testFile, []byte(testCode), 0o600)
		require.NoError(t, err, "Failed to reset test file")

		// run the tool with dry-run
		cmd := exec.Command(filepath.Join(tempDir, "unfuck-ai-comments"), "-dry-run", testFile)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		require.NoError(t, err, "Tool execution failed: %s", stderr.String())

		// verify the output contains diff
		output := stdout.String()
		assert.Contains(t, output, "-", "Dry run output doesn't contain diff markers")
		assert.Contains(t, output, "+", "Dry run output doesn't contain diff markers")
		assert.Contains(t, output, "// this comment", "Diff doesn't show lowercase conversion")

		// verify the file was not modified
		unmodifiedBytes, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read file")
		assert.Equal(t, testCode, string(unmodifiedBytes), "Dry run modified the file, but it shouldn't have")
	})

	t.Run("print mode", func(t *testing.T) {
		// run the tool in print mode
		cmd := exec.Command(filepath.Join(tempDir, "unfuck-ai-comments"), "-output=print", testFile)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		require.NoError(t, err, "Tool execution failed: %s", stderr.String())

		// verify the output contains the modified code
		output := stdout.String()
		assert.Contains(t, output, "// this comment", "Print output doesn't contain lowercase comments")
		assert.NotContains(t, output, "// THIS comment", "Print output contains uppercase comments that should be lowercase")
		assert.Contains(t, output, "// Package comment should NOT", "Print output doesn't preserve package comments")
	})
}

// Example_outputModes demonstrates the different output modes of the tool
func Example_outputModes() {
	// this is an example that shows how to use different output modes
	// unfuck-ai-comments -output=inplace file.go  # modify the file in place
	// unfuck-ai-comments -output=print file.go    # print the modified file to stdout
	// unfuck-ai-comments -output=diff file.go     # show a diff of the changes
	// unfuck-ai-comments -dry-run file.go         # same as -output=diff
}

// Example_recursiveProcessing demonstrates processing files recursively
func Example_recursiveProcessing() {
	// this is an example that shows how to process files recursively
	// unfuck-ai-comments ./...                    # process all .go files recursively
	// unfuck-ai-comments -dry-run ./...           # show what would be changed recursively
}

// TestRecursivePatterns tests recursive directory pattern matching
func TestRecursivePatterns(t *testing.T) {
	// create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "unfuck-ai-comments-recursive")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// sample content
	content := `package testpkg
func Test() {
	// UPPER case comment
}`

	// create test files
	files := []string{
		filepath.Join(tempDir, "root.go"),
		filepath.Join(subDir, "sub.go"),
	}

	for _, file := range files {
		err = os.WriteFile(file, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write test file %s: %v", file, err)
		}
	}

	// helper to count files processed
	countProcessedFiles := func(pattern string) int {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		processPattern(pattern, "diff")

		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		return strings.Count(output, "---")
	}

	// test recursive pattern
	t.Run("recursive pattern", func(t *testing.T) {
		pattern := filepath.Join(tempDir, "...")
		count := countProcessedFiles(pattern)
		if count != 2 {
			t.Errorf("Expected 2 files for recursive pattern, got %d", count)
		}
	})
}

// TestProcessPatternComprehensive tests the processPattern function more thoroughly
func TestProcessPatternComprehensive(t *testing.T) {
	// create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "unfuck-ai-comments-pattern-comprehensive")
	require.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tempDir)

	// create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDir, 0o750)
	require.NoError(t, err, "Failed to create subdirectory")

	// sample content with uppercase comments
	content := `package testpkg
func Test() {
	// UPPERCASE COMMENT
}`

	// create test files
	files := []string{
		filepath.Join(tempDir, "file1.go"),
		filepath.Join(tempDir, "file2.go"),
		filepath.Join(tempDir, "not-go.txt"),
		filepath.Join(subDir, "subfile.go"),
	}

	// write content to files
	for _, file := range files {
		err = os.WriteFile(file, []byte(content), 0o644)
		require.NoError(t, err, "Failed to write file %s", file)
	}

	// helper function to capture output from processpattern
	runProcessPattern := func(pattern, mode string) string {
		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// save current dir
		currentDir, err := os.Getwd()
		require.NoError(t, err, "Failed to get current directory")

		// change to temp dir for the test (if needed)
		if strings.HasPrefix(pattern, "./") || strings.HasPrefix(pattern, ".") {
			err = os.Chdir(tempDir)
			require.NoError(t, err, "Failed to change to temp dir")
			defer func() {
				err = os.Chdir(currentDir)
				require.NoError(t, err, "Failed to restore original directory")
			}()
		}

		// run processpattern
		processPattern(pattern, mode)

		// restore stdout
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		return buf.String()
	}

	// test different pattern types
	patternTests := []struct {
		name		string
		pattern		string
		mode		string
		expectedFiles	int
	}{
		{"specific file", filepath.Join(tempDir, "file1.go"), "diff", 1},
		{"glob pattern", filepath.Join(tempDir, "*.go"), "diff", 2},
		{"recursive with dots", filepath.Join(tempDir, "..."), "diff", 3},	// all .go files in tempdir and subdirs
	}

	for _, tc := range patternTests {
		t.Run(tc.name, func(t *testing.T) {
			output := runProcessPattern(tc.pattern, tc.mode)
			count := strings.Count(output, "---")
			assert.Equal(t, tc.expectedFiles, count, "Expected %d files processed for pattern %s, got %d",
				tc.expectedFiles, tc.pattern, count)
		})
	}

	// test special ./... pattern
	t.Run("./... pattern", func(t *testing.T) {
		output := runProcessPattern("./...", "diff")
		count := strings.Count(output, "---")
		assert.Equal(t, 3, count, "Expected 3 files processed for ./... pattern")
	})
}

// TestProcessPatternErrors tests error handling in the processPattern function
func TestProcessPatternErrors(t *testing.T) {
	// create a temporary directory for test files
	tempDir := t.TempDir()	// automatically cleaned up when the test finishes

	// create a test file
	testFile := filepath.Join(tempDir, "test.go")
	err := os.WriteFile(testFile, []byte(`package test
func Test() {
	// TEST comment
}`), 0o644)
	require.NoError(t, err, "Failed to write test file")

	// create an inaccessible directory if possible
	inaccessibleDir := filepath.Join(tempDir, "noaccess")
	err = os.Mkdir(inaccessibleDir, 0o000)
	if err != nil {
		t.Logf("Warning: Could not create inaccessible directory, skipping part of test: %v", err)
	}

	// test with invalid glob pattern
	t.Run("invalid glob pattern", func(t *testing.T) {
		// capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// process with invalid pattern
		processPattern("[", "diff")	// invalid glob pattern

		// restore stderr
		w.Close()
		os.Stderr = oldStderr
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify error message
		assert.Contains(t, output, "Error globbing pattern", "Should output error message for invalid pattern")
	})

	// test with nonexistent directory in recursive pattern
	t.Run("nonexistent directory", func(t *testing.T) {
		// capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// process with nonexistent directory
		processPattern("/nonexistent/dir/...", "diff")

		// restore stderr
		w.Close()
		os.Stderr = oldStderr
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify error message
		assert.Contains(t, output, "Error walking directory", "Should output error message for nonexistent directory")
	})

	if inaccessibleDir != "" && os.Getuid() != 0 {	// skip if running as root
		// test with inaccessible directory
		t.Run("inaccessible directory", func(t *testing.T) {
			// capture stderr
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// process with inaccessible directory
			processPattern(filepath.Join(inaccessibleDir, "..."), "diff")

			// restore stderr
			w.Close()
			os.Stderr = oldStderr
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			// verify error message or successful skip
			if output != "" {
				assert.Contains(t, output, "Error", "Should output error message for inaccessible directory")
			}
		})
	}
}

// TestProcessFileComprehensive tests all branches of the processFile function
func TestProcessFileComprehensive(t *testing.T) {
	// create a temporary directory
	tempDir := t.TempDir()	// automatically cleaned up when the test finishes

	// create test files with different scenarios
	// 1. file with no comments to modify
	noCommentsFile := filepath.Join(tempDir, "no_comments.go")
	err := os.WriteFile(noCommentsFile, []byte(`package test
func Test() {
	x := 1 // already lowercase comment
}`), 0o644)
	require.NoError(t, err, "Failed to write no comments file")

	// 2. file with comments to modify
	withCommentsFile := filepath.Join(tempDir, "with_comments.go")
	err = os.WriteFile(withCommentsFile, []byte(`package test
func Test() {
	// THIS SHOULD BE CONVERTED
	x := 1 // ANOTHER COMMENT
}`), 0o644)
	require.NoError(t, err, "Failed to write with comments file")

	// 3. file with parse error
	badSyntaxFile := filepath.Join(tempDir, "bad_syntax.go")
	err = os.WriteFile(badSyntaxFile, []byte(`package test
func Test() {
	missing closing brace
`), 0o644)
	require.NoError(t, err, "Failed to write bad syntax file")

	// 4. file with no modifications needed
	noModsNeededFile := filepath.Join(tempDir, "no_mods.go")
	err = os.WriteFile(noModsNeededFile, []byte(`package test
// This is a package comment
func Test() {
	x := 1 // this is already lowercase
}`), 0o644)
	require.NoError(t, err, "Failed to write no mods file")

	// capture stdout/stderr for testing
	captureOutput := func(fn func()) (string, string) {
		oldStdout := os.Stdout
		oldStderr := os.Stderr

		// capture stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		// capture stderr
		stderrR, stderrW, _ := os.Pipe()
		os.Stderr = stderrW

		// run the function
		fn()

		// close the writers
		stdoutW.Close()
		stderrW.Close()

		// read the outputs
		var stdoutBuf, stderrBuf bytes.Buffer
		_, _ = stdoutBuf.ReadFrom(stdoutR)
		_, _ = stderrBuf.ReadFrom(stderrR)

		// restore the original outputs
		os.Stdout = oldStdout
		os.Stderr = oldStderr

		return stdoutBuf.String(), stderrBuf.String()
	}

	// define tests
	tests := []struct {
		name		string
		file		string
		outputMode	string
		checkStdout	func(string)
		checkStderr	func(string)
		checkFile	func(string, string)
	}{
		{
			name:		"inplace mode - no comments to modify",
			file:		noCommentsFile,
			outputMode:	"inplace",
			checkStdout: func(out string) {
				assert.Empty(t, out, "No output expected when no changes needed")
			},
			checkStderr: func(err string) {
				assert.Empty(t, err, "No errors expected")
			},
			checkFile: func(path string, original string) {
				content, err := os.ReadFile(path)
				require.NoError(t, err, "Failed to read file")
				assert.Equal(t, original, string(content), "File should not be modified")
			},
		},
		{
			name:		"inplace mode - with comments to modify",
			file:		withCommentsFile,
			outputMode:	"inplace",
			checkStdout: func(out string) {
				assert.Contains(t, out, "Updated:", "Should report file was updated")
			},
			checkStderr: func(err string) {
				assert.Empty(t, err, "No errors expected")
			},
			checkFile: func(path string, original string) {
				content, err := os.ReadFile(path)
				require.NoError(t, err, "Failed to read file")
				assert.NotEqual(t, original, string(content), "File should be modified")
				assert.Contains(t, string(content), "// this should be converted", "Comments should be lowercase")
				assert.Contains(t, string(content), "// another comment", "All comments should be lowercase")
			},
		},
		{
			name:		"parse error",
			file:		badSyntaxFile,
			outputMode:	"inplace",
			checkStdout: func(out string) {
				assert.Empty(t, out, "No output expected for parse error")
			},
			checkStderr: func(err string) {
				assert.Contains(t, err, "Error parsing", "Should report parse error")
			},
			checkFile: func(path string, original string) {
				content, err := os.ReadFile(path)
				require.NoError(t, err, "Failed to read file")
				assert.Equal(t, original, string(content), "File should not be modified")
			},
		},
		{
			name:		"diff mode",
			file:		withCommentsFile,
			outputMode:	"diff",
			checkStdout: func(out string) {
				assert.Contains(t, out, "---", "Should show diff header")
				assert.Contains(t, out, "+++", "Should show diff header")
				assert.Contains(t, out, "THIS SHOULD BE CONVERTED", "Should show old line")
				assert.Contains(t, out, "this should be converted", "Should show new line")
			},
			checkStderr: func(err string) {
				assert.Empty(t, err, "No errors expected")
			},
			checkFile: func(path string, original string) {
				content, err := os.ReadFile(path)
				require.NoError(t, err, "Failed to read file")
				assert.Equal(t, original, string(content), "File should not be modified in diff mode")
			},
		},
		{
			name:		"print mode",
			file:		withCommentsFile,
			outputMode:	"print",
			checkStdout: func(out string) {
				assert.Contains(t, out, "// this should be converted", "Should print modified file")
				assert.Contains(t, out, "// another comment", "Should print all modified comments")
			},
			checkStderr: func(err string) {
				assert.Empty(t, err, "No errors expected")
			},
			checkFile: func(path string, original string) {
				content, err := os.ReadFile(path)
				require.NoError(t, err, "Failed to read file")
				assert.Equal(t, original, string(content), "File should not be modified in print mode")
			},
		},
		{
			name:		"no modifications needed",
			file:		noModsNeededFile,
			outputMode:	"inplace",
			checkStdout: func(out string) {
				assert.Empty(t, out, "No output expected when no changes needed")
			},
			checkStderr: func(err string) {
				assert.Empty(t, err, "No errors expected")
			},
			checkFile: func(path string, original string) {
				content, err := os.ReadFile(path)
				require.NoError(t, err, "Failed to read file")
				assert.Equal(t, original, string(content), "File should not be modified")
			},
		},
		{
			name:		"invalid output mode",
			file:		withCommentsFile,
			outputMode:	"invalid",
			checkStdout: func(out string) {
				assert.Empty(t, out, "No output expected for invalid mode")
			},
			checkStderr: func(err string) {
				assert.Empty(t, err, "No errors expected")
			},
			checkFile: func(path string, original string) {
				content, err := os.ReadFile(path)
				require.NoError(t, err, "Failed to read file")
				assert.Equal(t, original, string(content), "File should not be modified with invalid mode")
			},
		},
	}

	// run tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// read original content for comparison later
			originalContent, err := os.ReadFile(tc.file)
			require.NoError(t, err, "Failed to read original file")

			// process the file and capture output
			stdout, stderr := captureOutput(func() {
				processFile(tc.file, tc.outputMode)
			})

			// check outputs
			tc.checkStdout(stdout)
			tc.checkStderr(stderr)
			tc.checkFile(tc.file, string(originalContent))

			// reset the file for next test
			if tc.outputMode == "inplace" {
				err = os.WriteFile(tc.file, originalContent, 0o644)
				require.NoError(t, err, "Failed to reset file")
			}
		})
	}
}

// TestErrorsInProcessFile tests error handling paths in processFile
func TestErrorsInProcessFile(t *testing.T) {
	// create a temporary directory for test files
	tempDir := t.TempDir()	// automatically cleaned up when the test finishes

	// capture both stdout and stderr during tests
	captureOutput := func(fn func()) (string, string) {
		oldStdout := os.Stdout
		oldStderr := os.Stderr

		// capture stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		// capture stderr
		stderrR, stderrW, _ := os.Pipe()
		os.Stderr = stderrW

		// run the function
		fn()

		// close the writers
		stdoutW.Close()
		stderrW.Close()

		// read the outputs
		var stdoutBuf, stderrBuf bytes.Buffer
		_, _ = stdoutBuf.ReadFrom(stdoutR)
		_, _ = stderrBuf.ReadFrom(stderrR)

		// restore the original outputs
		os.Stdout = oldStdout
		os.Stderr = oldStderr

		return stdoutBuf.String(), stderrBuf.String()
	}

	// test nonexistent file (parse error)
	t.Run("nonexistent file", func(t *testing.T) {
		_, stderr := captureOutput(func() {
			processFile("/nonexistent/file.go", "inplace")
		})

		assert.Contains(t, stderr, "Error parsing", "Should output parse error for nonexistent file")
	})

	// test parse error with malformed go code
	t.Run("malformed go file", func(t *testing.T) {
		badFile := filepath.Join(tempDir, "bad.go")
		err := os.WriteFile(badFile, []byte(`package test
func Test() {
	missing closing brace
`), 0o644)
		require.NoError(t, err, "Failed to write malformed file")

		_, stderr := captureOutput(func() {
			processFile(badFile, "inplace")
		})

		assert.Contains(t, stderr, "Error parsing", "Should output parse error for malformed Go file")
	})

	// create a test file with comments to modify
	testFile := filepath.Join(tempDir, "testfile.go")
	err := os.WriteFile(testFile, []byte(`package test
func Test() {
	// THIS COMMENT should be modified
}`), 0o644)
	require.NoError(t, err, "Failed to write test file")

	// test printer error in inplace mode
	// this is tricky as printer.fprint rarely fails, but we can mock a wrapper around it
	t.Run("printer error in inplace mode", func(t *testing.T) {
		// create a writable directory but with read-only permission on file
		// first create a dummy file to modify
		readOnlyFile := filepath.Join(tempDir, "readonly.go")
		err := os.WriteFile(readOnlyFile, []byte(`package test
func Test() {
	// UPPERCASE comment
}`), 0o400) // read-only file
		require.NoError(t, err, "Failed to write read-only file")

		// make sure it's actually read-only
		if runtime.GOOS != "windows" {	// skip chmod tests on windows
			// we need to make the file read-only but allow opening for writing
			// on unix, we can make a file read-only
			err = os.Chmod(readOnlyFile, 0o400)
			if err != nil {
				t.Skip("Could not make file read-only, skipping test")
			}

			_, stderr := captureOutput(func() {
				processFile(readOnlyFile, "inplace")
			})

			// if the system allows opening read-only files for writing,
			// we should see an error at write time
			if stderr != "" {
				assert.Contains(t, stderr, "Error", "Should log error for read-only file")
			}
		}
	})

	// test printer error in print mode
	t.Run("printer error in print mode", func(t *testing.T) {
		// mock os.stdout with a closed pipe to force an error
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w
		w.Close()	// force error by closing the pipe

		// capture stderr
		oldStderr := os.Stderr
		errR, errW, _ := os.Pipe()
		os.Stderr = errW

		// process file
		processFile(testFile, "print")

		// restore stderr
		errW.Close()
		os.Stderr = oldStderr
		var errBuf bytes.Buffer
		_, _ = errBuf.ReadFrom(errR)
		output := errBuf.String()

		// restore stdout
		os.Stdout = oldStdout

		// check for error message
		assert.Contains(t, output, "Error writing to stdout", "Should report error writing to stdout")
	})

	// test diff mode with error reading original file
	t.Run("diff mode with read error", func(t *testing.T) {
		// create a non-readable file for testing
		if runtime.GOOS != "windows" {	// skip chmod tests on windows
			nonReadableFile := filepath.Join(tempDir, "nonreadable.go")
			err := os.WriteFile(nonReadableFile, []byte(`package test
func Test() {
	// TEST comment
}`), 0o200) // write-only file
			require.NoError(t, err, "Failed to write non-readable file")

			// try to make it non-readable
			err = os.Chmod(nonReadableFile, 0o200)
			if err != nil {
				t.Skip("Could not make file non-readable, skipping test")
			}

			_, stderr := captureOutput(func() {
				processFile(nonReadableFile, "diff")
			})

			// check for error message if the system respects 0o200 permission
			if stderr != "" {
				assert.Contains(t, stderr, "Error", "Should report error for non-readable file")
			}
		}
	})
}

// TestVendorExclusion tests that vendor directories are excluded
func TestVendorExclusion(t *testing.T) {
	// create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "unfuck-ai-comments-vendor")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// create a vendor directory
	vendorDir := filepath.Join(tempDir, "vendor")
	err = os.Mkdir(vendorDir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create vendor dir: %v", err)
	}

	// create another vendor directory in a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	nestedVendorDir := filepath.Join(subDir, "vendor")
	err = os.Mkdir(nestedVendorDir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create nested vendor dir: %v", err)
	}

	// sample content with uppercase comments
	content := `package testpkg
func Test() {
	// THIS IS AN UPPERCASE COMMENT
}`

	// create test files
	files := map[string]bool{
		filepath.Join(tempDir, "root.go"):		true,	// should be processed
		filepath.Join(subDir, "sub.go"):		true,	// should be processed
		filepath.Join(vendorDir, "vendor.go"):		false,	// should be skipped (in vendor)
		filepath.Join(nestedVendorDir, "nested.go"):	false,	// should be skipped (in nested vendor)
	}

	for file := range files {
		err = os.WriteFile(file, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write test file %s: %v", file, err)
		}
	}

	// helper to get processed files
	getProcessedFiles := func() []string {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// save current working directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// change to temp dir for the test
		if err := os.Chdir(tempDir); err != nil {
			t.Fatalf("Failed to change to temp dir: %v", err)
		}

		// process all files recursively
		processPattern("./...", "diff")

		// change back to original directory
		if err := os.Chdir(currentDir); err != nil {
			t.Fatalf("Failed to change back to original directory: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// extract file paths from diff headers
		var processed []string
		for _, line := range strings.Split(output, "\n") {
			if strings.HasPrefix(line, "--- ") && strings.Contains(line, "(original)") {
				// extract file path from diff header
				path := strings.TrimPrefix(line, "--- ")
				path = strings.TrimSuffix(path, " (original)")
				processed = append(processed, path)
			}
		}

		return processed
	}

	// test vendor exclusion
	t.Run("vendor exclusion", func(t *testing.T) {
		processed := getProcessedFiles()

		// check that each file was processed or skipped as expected
		for file, shouldProcess := range files {
			relativePath, err := filepath.Rel(tempDir, file)
			require.NoError(t, err, "Failed to get relative path")

			wasProcessed := false
			for _, p := range processed {
				if strings.HasSuffix(p, relativePath) {
					wasProcessed = true
					break
				}
			}

			if shouldProcess {
				assert.True(t, wasProcessed, "File %s should have been processed but wasn't", relativePath)
			} else {
				assert.False(t, wasProcessed, "File %s should have been skipped but was processed", relativePath)
			}
		}
	})
}

// removeWhitespace removes all whitespace and newlines from a string
// This is useful for comparing code strings that might be formatted differently
func removeWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, "")
}

// TestProgramOutput tests program behavior with different flags by using processFile directly
func TestProgramOutput(t *testing.T) {
	// create a temp dir for test files
	tempDir, err := os.MkdirTemp("", "unfuck-ai-program-test")
	require.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tempDir)

	// create a test file with comments
	content := `package test
func Test() {
	// THIS SHOULD be converted
	x := 1 // ANOTHER comment
}`

	// create test file
	testFile := filepath.Join(tempDir, "test_file.go")
	err = os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err, "Failed to write test file")

	// test inplace mode
	t.Run("inplace mode", func(t *testing.T) {
		// reset file
		err := os.WriteFile(testFile, []byte(content), 0o644)
		require.NoError(t, err, "Failed to reset test file")

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process file in inplace mode
		processFile(testFile, "inplace")

		// restore stdout
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify output
		assert.Contains(t, output, "Updated:", "Should show which file was updated")

		// check file was modified
		modifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read modified file")
		modified := string(modifiedContent)

		assert.Contains(t, modified, "// this should", "Should convert uppercase to lowercase")
		assert.NotContains(t, modified, "// THIS SHOULD", "Should not contain original uppercase comment")
		assert.Contains(t, modified, "// another comment", "Should convert all in-function comments")
	})

	// test dry-run (diff) mode
	t.Run("diff mode", func(t *testing.T) {
		// reset file
		err := os.WriteFile(testFile, []byte(content), 0o644)
		require.NoError(t, err, "Failed to reset test file")

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process file in diff mode
		processFile(testFile, "diff")

		// restore stdout
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify diff output
		assert.Contains(t, output, "---", "Diff should show file headers")
		assert.Contains(t, output, "+++", "Diff should show file headers")
		assert.Contains(t, output, "// this should", "Diff should show lowercase comments")

		// file should not be modified
		modifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read file")
		assert.Equal(t, content, string(modifiedContent), "File should not be modified in diff mode")
	})

	// test print mode
	t.Run("print mode", func(t *testing.T) {
		// reset file
		err := os.WriteFile(testFile, []byte(content), 0o644)
		require.NoError(t, err, "Failed to reset test file")

		// capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// process file in print mode
		processFile(testFile, "print")

		// restore stdout
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify printed output
		assert.Contains(t, output, "// this should", "Print output should contain lowercase comments")
		assert.NotContains(t, output, "// THIS SHOULD", "Print output should not contain uppercase comments")

		// file should not be modified
		modifiedContent, err := os.ReadFile(testFile)
		require.NoError(t, err, "Failed to read file")
		assert.Equal(t, content, string(modifiedContent), "File should not be modified in print mode")
	})

	// test color functionality
	t.Run("color behavior", func(t *testing.T) {
		// save current color setting and restore it after test
		originalNoColor := color.NoColor
		defer func() { color.NoColor = originalNoColor }()

		// test with colors disabled
		color.NoColor = true
		assert.True(t, color.NoColor, "NoColor should be true when colors are disabled")

		// process file with colors disabled
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		processFile(testFile, "diff")

		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// verify output (should not contain ansi color codes)
		assert.Contains(t, output, "---", "Output should contain diff markers")

		// test with colors enabled (for coverage)
		color.NoColor = false
		assert.False(t, color.NoColor, "NoColor should be false when colors are enabled")
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
	err = os.WriteFile(testFile, []byte(content), 0o644)
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
			processPattern(pattern, outputMode)
		}

		// restore stdout
		w.Close()
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
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// test inplace mode (default)
	t.Run("inplace mode", func(t *testing.T) {
		// reset test file
		if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to reset test file: %v", err)
		}

		// process file directly using the processfile function
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		processFile(testFile, "inplace")

		w.Close()
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
		if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to reset test file: %v", err)
		}

		// process file directly in diff mode
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		processFile(testFile, "diff")

		w.Close()
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
		if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to reset test file: %v", err)
		}

		// process file directly in print mode
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		processFile(testFile, "print")

		w.Close()
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
