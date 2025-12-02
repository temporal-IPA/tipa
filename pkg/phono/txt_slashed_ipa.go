package phono

import (
	"bufio"
	"bytes"
	"strings"
)

// sniffTxtSlashedIpa detects the ipa_dict_txt format, e.g.:
//
//	a\t/a/
//	à aucun moment\t/aokœ̃mɔmɑ̃/
//
// i.e. a tab, then an IPA string surrounded by slashes.
func sniffTxtSlashedIpa(sniff []byte, isEOF bool) bool {
	if len(sniff) == 0 {
		return false
	}
	reader := bytes.NewReader(sniff)
	scanner := bufio.NewScanner(reader)
	i := 10 // scan 10 lines.
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "\t") {
			return false
		}
		s := strings.Split(line, "\t")
		if len(s) != 2 {
			return false
		}
		if !strings.HasSuffix(s[1], "/") && !strings.HasSuffix(s[1], "/") {
			return false
		}
		i--
		if i == 0 {
			break
		}
	}
	return true
}

// parseTxtSlashedIpaLine parses a single line of the ipa_dict_txt format:
//
//	<word>\t/<IPA>/
//	<word>\t/<IPA1>/ /<IPA2>/
//
// All IPA values are converted to the internal representation without
// surrounding slashes.
func parseTxtSlashedIpaLine(line string) (string, []string, error) {
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return "", nil, nil
	}
	word := strings.TrimSpace(parts[0])
	raw := strings.TrimSpace(parts[1])
	if word == "" || raw == "" {
		return "", nil, nil
	}

	var prons []string
	rest := raw

	// Extract segments between /.../; this supports multiple IPA
	// forms on the same line, such as "/ipa1/ /ipa2/".
	for {
		start := strings.Index(rest, "/")
		if start == -1 {
			break
		}
		end := strings.Index(rest[start+1:], "/")
		if end == -1 {
			break
		}
		end += start + 1
		pron := strings.TrimSpace(rest[start+1 : end])
		if pron != "" {
			prons = append(prons, pron)
		}
		if end+1 >= len(rest) {
			break
		}
		rest = rest[end+1:]
	}

	if len(prons) == 0 {
		// Fallback: treat the whole RHS as a single pronunciation,
		// removing a single pair of surrounding slashes if present.
		trimmed := strings.Trim(raw, " ")
		if strings.HasPrefix(trimmed, "/") && strings.HasSuffix(trimmed, "/") && len(trimmed) > 2 {
			trimmed = trimmed[1 : len(trimmed)-1]
		}
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			prons = []string{trimmed}
		}
	}

	if len(prons) == 0 {
		return "", nil, nil
	}
	return word, prons, nil
}
