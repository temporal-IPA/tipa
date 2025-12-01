package ipa

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
)

var Charset string

func init() {
	var err error
	Charset, err = buildIPACharSet()
	if err != nil {
		panic(err)
	}
}

//go:embed tipa.json
var tipaSpecJSON []byte

// tipaSpec models the subset of tipa.json needed to derive the IPA character set.
type tipaSpec struct {
	IPA struct {
		BaseLetters                        []string `json:"base_letters"`
		LegacyLetters                      []string `json:"legacy_letters"`
		DiacriticsAndModifiers             []string `json:"diacritics_and_modifiers"`
		ToneDiacritics                     []string `json:"tone_diacritics"`
		SuprasegmentalsAndSpacingModifiers []string `json:"suprasegmentals_and_spacing_modifiers"`
		ToneLetters                        []string `json:"tone_letters"`
		OtherIPASymbols                    []string `json:"other_ipa_symbols"`
	} `json:"ipa"`
	ExtIPA struct {
		Letters             []string `json:"letters"`
		CombiningDiacritics []string `json:"combining_diacritics"`
		SpacingSymbols      []string `json:"spacing_symbols"`
	} `json:"extipa"`
	TIPAReservedCharacters []string `json:"tipa_reserved_characters"`
}

var ipaChars string

// buildIPACharSet builds the IPA character set from the embedded tipa.json.
//
// The function:
//   - parses tipa.json,
//   - adds all characters from IPA and ExtIPA sections,
//   - strips the placeholder '◌' from diacritics ("◌̥" → only '̥'),
//   - excludes all TIPA reserved characters.
//
// The final string is deduplicated and sorted by rune value to keep it stable.
func buildIPACharSet() (string, error) {
	var spec tipaSpec
	if err := json.Unmarshal(tipaSpecJSON, &spec); err != nil {
		panic(fmt.Sprintf("decode TIPA spec: %s", err.Error()))
	}

	// Collect TIPA reserved characters so they can be excluded.
	reserved := make(map[rune]struct{}, len(spec.TIPAReservedCharacters))
	for _, s := range spec.TIPAReservedCharacters {
		for _, r := range s {
			reserved[r] = struct{}{}
		}
	}

	// Use a set of runes to deduplicate.
	uniq := make(map[rune]struct{})

	// Helper to add characters from slices of strings.
	// When stripPlaceholder is true, rune '◌' is skipped to avoid
	// inserting the placeholder itself from strings such as "◌̥".
	addRunes := func(list []string, stripPlaceholder bool) {
		for _, s := range list {
			for _, r := range s {
				if stripPlaceholder && r == '◌' {
					continue
				}
				if _, skip := reserved[r]; skip {
					continue
				}
				uniq[r] = struct{}{}
			}
		}
	}

	// IPA section.
	addRunes(spec.IPA.BaseLetters, false)
	addRunes(spec.IPA.LegacyLetters, false)
	addRunes(spec.IPA.DiacriticsAndModifiers, true)
	addRunes(spec.IPA.ToneDiacritics, true)
	addRunes(spec.IPA.SuprasegmentalsAndSpacingModifiers, false)
	addRunes(spec.IPA.ToneLetters, false)
	addRunes(spec.IPA.OtherIPASymbols, false)

	// ExtIPA section.
	addRunes(spec.ExtIPA.Letters, false)
	addRunes(spec.ExtIPA.CombiningDiacritics, true)
	addRunes(spec.ExtIPA.SpacingSymbols, false)

	// Materialize as a sorted string for reproducibility.
	runes := make([]rune, 0, len(uniq))
	for r := range uniq {
		runes = append(runes, r)
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })

	return string(runes), nil
}
