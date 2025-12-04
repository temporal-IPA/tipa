// Package phonodict provides helpers to preload, merge and represent
// pronunciation dictionaries. It supports multiple input formats via
// pluggable "Loader" implementations and exposes a functional API
// for line-based textual preloaders.
package phono

import (
	"strings"
	"unicode/utf8"
)

// Expression is a sequence of grapheme encode as an UTF8 string.
type Expression = string

// IPA is an international phonetic alphabet composed string.
type IPA = string

type Phonetized = string

type Dictionary map[Expression][]IPA

// Representation holds the internal dictionary representation used
// by the scanner and the preloaders.
type Representation struct {
	Entries        Dictionary
	SeenWordPron   map[string]struct{}
	PreloadedWords map[string]struct{}
}

// NewRepresentation creates an empty Representation with reasonable
// initial capacities.
func NewRepresentation() *Representation {
	return &Representation{
		Entries:        make(Dictionary, 1<<16),
		SeenWordPron:   make(map[string]struct{}, 1<<18),
		PreloadedWords: make(map[string]struct{}),
	}
}

type KeyMap map[string][]string

// NormalizedKeys returns a dictionary with all the corresponding keys
// for a given normalized key (NormalizeString view).
func (d Dictionary) NormalizedKeys() KeyMap {
	keys := make(KeyMap)
	for k := range d {
		lk := NormalizeString(k)
		keys[lk] = append(keys[lk], k)
	}
	return keys
}

// MaxKeyLen returns the maximum key length in runes. It is used by the
// scanner to bound the size of candidate substrings when performing
// greedy longest‑match scans.
func (d Dictionary) MaxKeyLen() int {
	maxKeyLen := 0
	for key := range d {
		if l := utf8.RuneCountInString(key); l > maxKeyLen {
			maxKeyLen = l
		}
	}
	return maxKeyLen
}

// NormalizeString is the func used to normalize the keys.
func NormalizeString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
