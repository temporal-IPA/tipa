package phono

import (
	"bufio"
	"bytes"
	"strings"
)

// sniffTextIpa detects the native tab-separated text format:
//
//	<expression>\t<phon1> | <phon2> | ...
//
// Lines that are empty or comments (starting with '#') are ignored during sniff.
func sniffTextIpa(sniff []byte, isEOF bool) bool {
	if len(sniff) == 0 {
		return false
	}
	reader := bytes.NewReader(sniff)
	scanner := bufio.NewScanner(reader)

	validLines := 0
	for scanner.Scan() {
		line := stripInlineCommentAndTrim(scanner.Text())
		if line == "" {
			continue
		}

		if !strings.Contains(line, "\t") {
			return false
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			return false
		}
		// Native text format must not look like a slashed-phonetic format.
		if strings.HasSuffix(strings.TrimSpace(parts[1]), "/") {
			return false
		}

		validLines++
		if validLines >= 2 {
			break
		}
	}
	return validLines > 0
}

// parseTextIpaLine parses a single line of the native text format:
//
//	<expression>\t<phon1> | <phon2> | ...
//
// The expression is trimmed, and inline comments starting with '#' are
// stripped by the caller.
func parseTextIpaLine(line string) (string, []string, error) {
	line = stripInlineCommentAndTrim(line)
	if line == "" {
		return "", nil, nil
	}

	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return "", nil, nil
	}
	expression := strings.TrimSpace(parts[0])
	rawPhones := strings.TrimSpace(parts[1])
	if expression == "" || rawPhones == "" {
		return "", nil, nil
	}

	chunks := strings.Split(rawPhones, "|")
	phones := make([]string, 0, len(chunks))
	for _, c := range chunks {
		p := strings.TrimSpace(c)
		if p == "" {
			continue
		}
		phones = append(phones, p)
	}
	if len(phones) == 0 {
		return "", nil, nil
	}
	return expression, phones, nil
}
