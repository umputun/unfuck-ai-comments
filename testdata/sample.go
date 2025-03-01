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

// Helper function demonstrates another function.
func helperFunction() {
	// ALL CAPS COMMENT should be converted

	// Another comment TO BE converted
}
