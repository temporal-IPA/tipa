package phono

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// NewLineLoader constructs a Loader that reads a text source
// line by line and delegates actual parsing to the provided LineParser.
// This makes it easy to support additional textual formats (e.g. Lexique, Flexique, custom tab-separated dictionaries).
func NewLineLoader(
	kind Kind,
	sniff func(sniff []byte, isEOF bool) bool,
	parser LineParser,
) Loader {
	return &lineLoader{
		kind:      kind,
		sniffFunc: sniff,
		parseLine: parser,
	}
}

// LineParser is a per-line parser for text-based formats.
// It receives a single logical line (with surrounding whitespace
// already trimmed) and should return the word and its pronunciations.
// If the line should be ignored, it can return word == "" or len(prons) == 0.
type LineParser func(line string) (word string, prons []string, err error)

// lineLoader is a generic implementation for textual formats where
// each entry fits on a single line.
type lineLoader struct {
	kind      Kind
	sniffFunc func(sniff []byte, isEOF bool) bool
	parseLine LineParser
}

func (p *lineLoader) Kind() Kind { return p.kind }

func (p *lineLoader) Sniff(sniff []byte, isEOF bool) bool {
	if p.sniffFunc == nil {
		return false
	}
	return p.sniffFunc(sniff, isEOF)
}

func (p *lineLoader) LoadAll(r io.Reader) (Dictionary, error) {
	dict := make(Dictionary)
	err := p.Load(r, func(word string, prons []string) error {
		dict[word] = prons
		return nil
	})
	return dict, err
}

func (p *lineLoader) Load(r io.Reader, emit OnEntryFunc) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		word, prons, err := p.parseLine(line)
		if err != nil {
			return fmt.Errorf("(%s): parse line %q: %w", p.kind, line, err)
		}
		if word == "" || len(prons) == 0 {
			continue
		}
		if err := emit(word, prons); err != nil {
			return err
		}
	}
	return scanner.Err()
}
