package output

import (
	"fmt"
	"strings"
)

// ValidationError represents a validation error with a message and an optional hint.
type ValidationError struct {
	Field   string
	Message string
	Hint    string
	Usage   string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// RuntimeError represents a runtime error with a user-facing message,
// an optional hint, and an optional technical cause.
type RuntimeError struct {
	Message string
	Hint    string
	Cause   error
}

func (e *RuntimeError) Error() string {
	return e.Message
}

// FormatValidationErrors formats a list of validation errors into a single string.
func FormatValidationErrors(errs []*ValidationError) string {
	if len(errs) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, e := range errs {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("✗ %s\n", e.Message))

		switch {
		case e.Usage != "":
			if e.Hint != "" {
				sb.WriteString("\nHint:\n")
				writeIndentedBlock(&sb, e.Hint)
			}

			sb.WriteString("\nUsage:\n")
			writeIndentedBlock(&sb, e.Usage)
		case e.Hint != "":
			sb.WriteString(fmt.Sprintf("→ %s\n", e.Hint))
		}
	}
	return sb.String()
}

// FormatRuntimeError formats a runtime error into a single string.
func FormatRuntimeError(e *RuntimeError) string {
	if e == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✗ %s\n", e.Message))

	if e.Hint != "" {
        sb.WriteString("\nHint:\n")
		writeIndentedBlock(&sb, e.Hint)
	}

	if e.Cause != nil {
		sb.WriteString("\nDetails:\n")
		writeIndentedBlock(&sb, e.Cause.Error())
	}

	return sb.String()
}

func writeIndentedBlock(sb *strings.Builder, value string) {
	for _, line := range strings.Split(value, "\n") {
		sb.WriteString(fmt.Sprintf("  %s\n", line))
	}
}
