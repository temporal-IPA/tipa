package phono

import (
	"bufio"
	"fmt"
	"io"
)

// NewLineLoader constructs a Loader that reads a text source
// line by line and delegates actual parsing to the provided LineParser.
// This makes it easy to support additional textual formats (e.g. Lexique,
// Flexique, custom tab- or space-separated dictionaries).
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
//
// It receives a single logical line (with surrounding whitespace
// and inline comments already stripped by the loader) and should
// return the expression and its pronunciations.
// If the line should be ignored, it can return expression == "" or
// len(phones) == 0.
type LineParser func(line string) (expression string, phones []string, err error)

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
	err := p.Load(r, func(expression string, phones []string) error {
		dict[expression] = phones
		return nil
	})
	return dict, err
}

func (p *lineLoader) Load(r io.Reader, emit OnEntryFunc) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := stripInlineCommentAndTrim(scanner.Text())
		if line == "" {
			continue
		}
		expression, phones, err := p.parseLine(line)
		if err != nil {
			return fmt.Errorf("(%s): parse line %q: %w", p.kind, line, err)
		}
		if expression == "" || len(phones) == 0 {
			continue
		}
		if err := emit(expression, phones); err != nil {
			return err
		}
	}
	return scanner.Err()
}
