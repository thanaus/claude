package output

import (
	"fmt"
	"strings"
)

// Format represents the desired output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
)

// ValidFormats lists the accepted output formats.
var ValidFormats = []string{string(FormatTable), string(FormatJSON)}

// IsValid reports whether the format is supported.
func (f Format) IsValid() bool {
	for _, v := range ValidFormats {
		if string(f) == v {
			return true
		}
	}
	return false
}

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

func writeIndentedBlock(sb *strings.Builder, value string) {
	for _, line := range strings.Split(value, "\n") {
		sb.WriteString(fmt.Sprintf("  %s\n", line))
	}
}
