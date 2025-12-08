package phono

import "strings"

// stripInlineCommentAndTrim removes leading/trailing whitespace and strips
// inline comments introduced by '#' (one or more). Lines that are empty
// or pure comments return the empty string.
func stripInlineCommentAndTrim(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	return line
}
