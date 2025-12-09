// Package phonodict provides helpers to preload, merge and represent
// pronunciation dictionaries. It supports multiple input formats via
// pluggable "Loader" implementations and exposes a functional API
// for line-based textual loaders.
package phono

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Expression is a sequence of graphemes encoded as a UTF‑8 string.
type Expression = string

type Phonetized = string

type AnnotatedPhonetized struct {
	S Phonetized `json:"s"`
	C float64    `json:"c"`
}

type Dictionary map[Expression][]Phonetized

// Representation holds the internal dictionary representation used
// by the scanner and the loaders.
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

// splitSymbols splits a phonetic string into "symbols" instead of
// single runes. A symbol is defined here as:
//
//	baseRune + any following combining marks (Unicode category Mn)
//
// This allows us to treat sequences such as "œ̃" as a single unit,
// even though it is encoded as multiple runes.
func splitSymbols(p Phonetized) []string {
	var (
		symbols []string
		buf     []rune
	)

	for _, r := range p {
		if len(buf) == 0 {
			// start a new symbol
			buf = append(buf, r)
			continue
		}

		// combining marks (Mn) are attached to the current symbol
		if unicode.Is(unicode.Mn, r) {
			buf = append(buf, r)
			continue
		}

		// new base rune → flush previous symbol and start a new one
		symbols = append(symbols, string(buf))
		buf = buf[:0]
		buf = append(buf, r)
	}

	if len(buf) > 0 {
		symbols = append(symbols, string(buf))
	}

	return symbols
}

// Symbols returns all unique phonetic symbols observed in the dictionary.
// A symbol is a base rune plus its combining marks (see splitSymbols).
// The returned slice is not sorted: callers may sort it if needed.
func (d Dictionary) Symbols() []string {
	seen := make(map[string]struct{})

	for _, prons := range d {
		for _, p := range prons {
			for _, sym := range splitSymbols(p) {
				if _, ok := seen[sym]; !ok {
					seen[sym] = struct{}{}
				}
			}
		}
	}

	res := make([]string, 0, len(seen))
	for s := range seen {
		res = append(res, s)
	}
	return res
}

// SymbolsWithSample returns, for each phonetic symbol, one **key**
// (orthographic expression) that uses this symbol.
//
// When several keys contain the same symbol, the **shortest key**
// (in bytes) is selected, not the shortest pronunciation. For
// example, if the symbol /a/ appears in entries:
//
//	"à"           → /a/
//	"à aucun prix" → /aokœ̃pʁi/
//
// the chosen sample for the symbol /a/ will be the key "à" because
// it is shorter than "à aucun prix".
func (d Dictionary) SymbolsWithSample() map[string]Expression {
	res := make(map[string]Expression)

	for key, prons := range d {
		for _, p := range prons {
			for _, sym := range splitSymbols(p) {
				existingKey, ok := res[sym]
				if !ok {
					// first time we see this symbol → take current key
					res[sym] = key
					continue
				}

				// prefer the shortest key as the sample for this symbol
				if len(key) < len(existingKey) {
					res[sym] = key
				}
			}
		}
	}

	return res
}
