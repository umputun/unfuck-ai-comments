package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

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

	// No comments should be found
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
		"// Comment on the same line as function opening brace SHOULD be modified": true,
		"// Comment on the same line as function closing brace should NOT be modified": false,
		"// Comment in parameter list should NOT be modified": false,
		"// Inline comment in parameter list should NOT be modified": false,
		"// This comment SHOULD be modified": true,
	}

	// Check each comment's classification
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

	// All comments should be inside a function
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
	// Skip test on Go versions before 1.18
	src := `package main

// Generic function comment should NOT be modified
func Generic[T any](param T) {
	// INSIDE generic function should be modified
	
	// Another comment to modify
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "generic.go", src, parser.ParseComments)
	if err != nil {
		// This might fail on older Go versions, so skip test in that case
		if strings.Contains(err.Error(), "expected")  {
			t.Skip("Skipping generic function test on older Go version")
		}
		t.Fatalf("Failed to parse test source: %v", err)
	}

	expectedResults := map[string]bool{
		"// Generic function comment should NOT be modified": false,
		"// INSIDE generic function should be modified": true,
		"// Another comment to modify": true,
	}

	// Check each comment's classification
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
	
	if result != expected {
		t.Errorf("Unicode conversion failed. Expected %q, got %q", expected, result)
	}
}

// TestMultiByteComments tests handling of emojis and other multi-byte characters
func TestMultiByteComments(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "// EMOJI TEST: 😀 😃 😄 👍",
			expected: "// emoji test: 😀 😃 😄 👍",
		},
		{
			input:    "// MIXED CASE with EMOJI: Hello 👋 World",
			expected: "// mixed case with emoji: hello 👋 world",
		},
	}
	
	for _, test := range tests {
		result := convertCommentToLowercase(test.input)
		if result != test.expected {
			t.Errorf("Multi-byte conversion failed. Expected %q, got %q", test.expected, result)
		}
	}
}

// TestHandlingOfBadCode tests the tool's behavior with malformed code
func TestHandlingOfBadCode(t *testing.T) {
	// Test with broken code that can't be parsed
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