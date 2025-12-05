package phono

import (
	"bufio"
	"bytes"
	"strings"
)

// sniffTextIpa detects the native ipadict text format:
//
//	<word>\t<IPA1> | <IPA2> | ...
func sniffTextIpa(sniff []byte, isEOF bool) bool {
	if len(sniff) == 0 {
		return false
	}
	reader := bytes.NewReader(sniff)
	scanner := bufio.NewScanner(reader)
	i := 2 // scan 2 lines.
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
		if strings.HasSuffix(s[1], "/") {
			return false
		}
		i--
		if i == 0 {
			break
		}
	}
	return true
}

// parseTextIpaLine parses a single line of the native text format:
//
//	<word>\t<IPA1> | <IPA2> | ...
func parseTextIpaLine(line string) (string, []string, error) {
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return "", nil, nil
	}
	word := strings.TrimSpace(parts[0])
	rawProns := strings.TrimSpace(parts[1])
	if word == "" || rawProns == "" {
		return "", nil, nil
	}

	chunks := strings.Split(rawProns, "|")
	prons := make([]string, 0, len(chunks))
	for _, c := range chunks {
		p := strings.TrimSpace(c)
		if p == "" {
			continue
		}
		prons = append(prons, p)
	}
	if len(prons) == 0 {
		return "", nil, nil
	}
	return word, prons, nil
}
