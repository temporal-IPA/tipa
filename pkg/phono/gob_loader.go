package phono

import (
	"encoding/gob"
	"fmt"
	"io"
	"unicode/utf8"
)

// GobLoader handles gob-encoded map[string][]string dictionaries.
type GobLoader struct{}

// Kind reports the preloader kind identifier for gob dictionaries.
func (g *GobLoader) Kind() Kind { return KindGOB }

// Sniff identifies gob payloads using filename hints and binary heuristics.
//
// GobLoader files are:
//   - always selected when the path ends with ".gob";
//   - never selected when the path looks like a text dictionary
//     (".txt", ".txtipa", ".ipa");
//   - otherwise, detected when the sniff bytes are not valid UTF-8 or contain
//     NUL bytes. This avoids misclassifying regular text dictionaries as gob.
func (g *GobLoader) Sniff(sniff []byte, isEOF bool) bool {
	if len(sniff) == 0 {
		return false
	}
	// If it's not valid UTF-8, it's very likely a binary (gob) payload.
	if !utf8.Valid(sniff) {
		return true
	}
	// Heuristic: presence of NUL bytes strongly suggests a binary format.
	for _, b := range sniff {
		if b == 0 {
			return true
		}
	}
	return false
}

// LoadAll deserializes a Dictionary
func (g *GobLoader) LoadAll(r io.Reader) (Dictionary, error) {
	dec := gob.NewDecoder(r)
	dict := make(Dictionary)
	err := dec.Decode(&dict)
	return dict, err
}

// Load decodes a gob-encoded Dictionary and emits all entries.
// Don't use this method if possible, use LoadAll
func (g *GobLoader) Load(r io.Reader, emit OnEntryFunc) error {
	dec := gob.NewDecoder(r)
	dict := make(Dictionary)
	if err := dec.Decode(&dict); err != nil {
		return fmt.Errorf("decode gob: %w", err)
	}
	for w, prons := range dict {
		if len(prons) == 0 {
			continue
		}
		if err := emit(w, prons); err != nil {
			return err
		}
	}
	return nil
}
