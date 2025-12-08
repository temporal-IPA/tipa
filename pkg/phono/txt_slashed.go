package phono

import (
	"bufio"
	"bytes"
	"strings"
)

// sniffSlashedTxt detects the "slashed" phonetic text format, e.g.:
//
//	expression\t/phon/
//	expression   /phon1/;/phon2/
//
// The separator between the expression and the first '/.../' can be
// a tab or any amount of whitespace. Lines that are empty or comments
// (starting with '#') are ignored during sniff.
func sniffSlashedTxt(sniff []byte, isEOF bool) bool {
	if len(sniff) == 0 {
		return false
	}
	reader := bytes.NewReader(sniff)
	scanner := bufio.NewScanner(reader)

	sawValid := false
	for scanner.Scan() {
		line := stripInlineCommentAndTrim(scanner.Text())
		if line == "" {
			continue
		}

		firstSlash := strings.Index(line, "/")
		if firstSlash <= 0 {
			// Either no slash or slash as first char: not a valid
			// "expression /phones/" line.
			return false
		}

		// Require a non-empty expression before the first slash.
		expr := strings.TrimSpace(line[:firstSlash])
		if expr == "" {
			return false
		}

		// There must be at least one more slash after the first.
		if strings.Index(line[firstSlash+1:], "/") == -1 {
			return false
		}

		sawValid = true
		// Checking one good data line is enough to classify.
		break
	}

	return sawValid
}

// parseSlashedTxtLine parses a single line of the slashed-phonetic format:
//
//	<expression> <space-or-tab> /phon/            # single form
//	<expression> <space-or-tab> /phon1/;/phon2/   # separated by ';'
//	<expression> <space-or-tab> /phon1/ , /phon2  # separated by ','
//
// Everything between two phonetic segments is ignored. For example,
// "/p1/;/p2/" and "/p1/ , /p2" both yield the two forms "p1", "p2".
//
// The expression is trimmed; e.g. "  benoit pereira da silva  "
// yields the key "benoit pereira da silva".
func parseSlashedTxtLine(line string) (string, []string, error) {
	line = stripInlineCommentAndTrim(line)
	if line == "" {
		return "", nil, nil
	}

	// Find the first slash that starts the phonetic part.
	firstSlash := strings.Index(line, "/")
	if firstSlash <= 0 {
		return "", nil, nil
	}

	expression := strings.TrimSpace(line[:firstSlash])
	if expression == "" {
		return "", nil, nil
	}

	raw := strings.TrimSpace(line[firstSlash:])
	if raw == "" {
		return "", nil, nil
	}

	var phones []string
	rest := raw

	// Extract segments between /.../, ignoring anything in between.
	for {
		start := strings.Index(rest, "/")
		if start == -1 {
			break
		}

		// Look for the closing slash after start+1.
		next := strings.Index(rest[start+1:], "/")
		var segment string
		if next == -1 {
			// No closing slash: treat everything after start as the last form.
			segment = rest[start+1:]
			rest = ""
		} else {
			end := start + 1 + next
			segment = rest[start+1 : end]
			if end+1 >= len(rest) {
				rest = ""
			} else {
				rest = rest[end+1:]
			}
		}

		phone := strings.TrimSpace(segment)
		if phone != "" {
			phones = append(phones, phone)
		}

		if rest == "" {
			break
		}
	}

	// Fallback: if we did not manage to extract anything using /.../,
	// treat the whole right-hand side as a single phonetic string,
	// optionally removing one surrounding pair of slashes.
	if len(phones) == 0 {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "/") && strings.HasSuffix(trimmed, "/") && len(trimmed) > 2 {
			trimmed = trimmed[1 : len(trimmed)-1]
		}
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			phones = []string{trimmed}
		}
	}

	if len(phones) == 0 {
		return "", nil, nil
	}
	return expression, phones, nil
}
