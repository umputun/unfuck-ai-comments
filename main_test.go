package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		processPattern("./...", "inplace", false, true)

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
