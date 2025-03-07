package testdata

import "log"

// Remote executes commands on remote server, via ssh. Not thread-safe.
// This comment should NOT be converted.
type Remote struct {
	// These field comments should NOW be converted (when inside struct)
	client   *sshClient `json:"client,omitempty"`
	hostAddr string     `json:"hostAddr,omitempty"`
	hostName string     `json:"hostName,omitempty"`
	logs     log.Logger `json:"logs,omitempty"`
}

// Close connection to remote server.
// This comment should NOT be converted.
func (ex *Remote) Close() error {
	// TODO IMPLEMENT ME - this comment should remain unchanged (special indicator)
	// THIS FUNCTION is not implemented yet
	// ANOTHER Strange Function
	x := 1 // INLINE COMMENT that SHOULD be converted
	_ = x
	return nil
}

// Execute runs a command on remote server.
// This comment should NOT be converted.
func (ex *Remote) Execute(cmd string) (string, error) {
	// THIS is a comment INSIDE function
	if cmd == "" {
		// THIS is another NESTED comment
		return "", nil
	}

	// Complex cases with nested blocks
	for i := 0; i < 10; i++ {
		// COMMENT in for LOOP should be converted
	}

	return "result", nil
}

type sshClient struct {
	// COMMENT INSIDE struct - should NOW be converted
	key string `json:"key,omitempty"` // This is a placeholder for the key field.
	val string `json:"val,omitempty"` // THIS is a PLACEHOLDER for the val field.

	// TODO This comment should remain unchanged (special indicator)

	// FIXME This comment should remain unchanged (special indicator)
}

// Comment between types should NOT be converted

// Package-level var should NOT be converted
var singleVar = "test"

// This comment should NOT be converted (outside block)
var (
	// THIS Comment SHOULD be converted (inside var block)
	debugEnabled bool = false // INLINE Comment SHOULD be converted (inside var block)

	// ANOTHER Comment to PROCESS
	configPath string = "/etc/config.json" // ANOTHER Inline COMMENT to process
)

// Package-level const should NOT be converted
const singleConst = 42

// This comment should NOT be converted (outside block)
const (
	// THIS Comment SHOULD be converted (inside const block)
	statusOK int = 200 // INLINE Comment SHOULD be converted (inside const block)

	// ANOTHER Comment to PROCESS
	maxRetries int = 3 // ANOTHER Inline COMMENT to process
)

// Helper function demonstrates another function.
func helperFunction() {
	// ALL CAPS COMMENT should be converted

	// Another comment TO BE converted

	// Example with camelCase and PascalCase identifiers
	someVariableName := "camelCase" // camelCase should be preserved
	OtherVariable := "PascalCase"   // PascalCase should be preserved
	_ = someVariableName
	_ = OtherVariable
	
	// Testing technical linter directives
	r := true //nolint:gosec // Using math/rand is ACCEPTABLE for tests
	_ = r
	
	// Local vars and consts inside functions
	var (
		// THIS SHOULD be converted (inside function & var block)
		localVar string = "test"
		
		// ANOTHER Local Comment
		count int = 0
	)
	
	const (
		// THIS SHOULD be converted (inside function & const block)
		localConst float64 = 3.14
		
		// ANOTHER Local Comment
		maxCount int = 100
	)
	
	_ = localVar
	_ = count
	_ = localConst
	_ = maxCount
}
