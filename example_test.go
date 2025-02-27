package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegration runs the actual CLI tool on test files
func TestIntegration(t *testing.T) {

	// Create temp dir and test files
	tempDir, err := os.MkdirTemp("", "unfuck-ai-integration")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file with comments
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
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Build the CLI tool
	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tempDir, "unfuck-ai-comments"))
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build CLI tool: %v", err)
	}

	// Run the integration tests
	t.Run("inplace mode", func(t *testing.T) {
		// Reset the file before test
		err = os.WriteFile(testFile, []byte(testCode), 0o600)
		if err != nil {
			t.Fatalf("Failed to reset test file: %v", err)
		}

		// Run the tool
		cmd := exec.Command(filepath.Join(tempDir, "unfuck-ai-comments"), testFile)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		
		if err := cmd.Run(); err != nil {
			t.Fatalf("Tool execution failed: %v\nStderr: %s", err, stderr.String())
		}

		// Read the modified file
		modifiedBytes, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read modified file: %v", err)
		}
		modified := string(modifiedBytes)

		// Verify the changes
		if strings.Contains(modified, "// THIS comment") {
			t.Error("Failed to convert uppercase comment to lowercase")
		}
		if !strings.Contains(modified, "// this comment") {
			t.Error("Did not properly convert to lowercase")
		}
		if !strings.Contains(modified, "// Package comment should NOT") {
			t.Error("Incorrectly modified package comment")
		}
		if strings.Contains(modified, "// ANOTHER comment") {
			t.Error("Failed to convert comment in second function")
		}
		if !strings.Contains(modified, "// another comment") {
			t.Error("Did not properly convert comment in second function")
		}
	})

	t.Run("dry-run mode", func(t *testing.T) {
		// Reset the file before test
		err = os.WriteFile(testFile, []byte(testCode), 0o600)
		if err != nil {
			t.Fatalf("Failed to reset test file: %v", err)
		}

		// Run the tool with dry-run
		cmd := exec.Command(filepath.Join(tempDir, "unfuck-ai-comments"), "-dry-run", testFile)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		
		if err := cmd.Run(); err != nil {
			t.Fatalf("Tool execution failed: %v\nStderr: %s", err, stderr.String())
		}

		// Verify the output contains diff
		output := stdout.String()
		if !strings.Contains(output, "-") || !strings.Contains(output, "+") {
			t.Error("Dry run output doesn't contain diff markers")
		}
		if !strings.Contains(output, "// this comment") {
			t.Error("Diff doesn't show lowercase conversion")
		}

		// Verify the file was NOT modified
		unmodifiedBytes, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if string(unmodifiedBytes) != testCode {
			t.Error("Dry run modified the file, but it shouldn't have")
		}
	})

	t.Run("print mode", func(t *testing.T) {
		// Run the tool in print mode
		cmd := exec.Command(filepath.Join(tempDir, "unfuck-ai-comments"), "-output=print", testFile)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		
		if err := cmd.Run(); err != nil {
			t.Fatalf("Tool execution failed: %v\nStderr: %s", err, stderr.String())
		}

		// Verify the output contains the modified code
		output := stdout.String()
		if !strings.Contains(output, "// this comment") {
			t.Error("Print output doesn't contain lowercase comments")
		}
		if strings.Contains(output, "// THIS comment") {
			t.Error("Print output contains uppercase comments that should be lowercase")
		}
		if !strings.Contains(output, "// Package comment should NOT") {
			t.Error("Print output doesn't preserve package comments")
		}
	})
}

// Example_outputModes demonstrates the different output modes of the tool
func Example_outputModes() {
	// This is an example that shows how to use different output modes
	// unfuck-ai-comments -output=inplace file.go  # Modify the file in place
	// unfuck-ai-comments -output=print file.go    # Print the modified file to stdout
	// unfuck-ai-comments -output=diff file.go     # Show a diff of the changes
	// unfuck-ai-comments -dry-run file.go         # Same as -output=diff
}

// Example_recursiveProcessing demonstrates processing files recursively
func Example_recursiveProcessing() {
	// This is an example that shows how to process files recursively
	// unfuck-ai-comments ./...                    # Process all .go files recursively
	// unfuck-ai-comments -dry-run ./...           # Show what would be changed recursively
}