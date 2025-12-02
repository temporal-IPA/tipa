// Package phonodict provides helpers to preload, merge and represent
// pronunciation dictionaries. It supports multiple input formats via
// pluggable "Loader" implementations and exposes a functional API
// for line-based textual preloaders.
package phono

import "strings"

// Expression is a sequence of grapheme encode as an UTF8 string.
type Expression = string

// IPA is an international phonetic alphabet composed string.
type IPA = string

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

// NormalizedKeys returns a dictionary with all the corresponding keys for a given normalized key.
func (d Dictionary) NormalizedKeys() map[string][]string {
	keys := make(map[string][]string)
	i := 0
	for k, _ := range d {
		lk := NormalizeString(k)
		if _, ok := keys[lk]; !ok {
			keys[lk] = make([]string, 0)
		}
		keys[lk] = append(keys[lk], k)
		i++
	}
	return keys
}

// NormalizeString is the func used to normalize the keys.
func NormalizeString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
