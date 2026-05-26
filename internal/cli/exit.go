package cli

import "fmt"

// ExitError signals a non-zero process exit code without printing an extra message.
type ExitError struct {
	Code int
	Msg  string
}

func (e *ExitError) Error() string {
	if e.Msg == "" {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Msg
}
