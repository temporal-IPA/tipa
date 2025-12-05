package g2p

import (
	"sort"
	"strings"

	"github.com/temporal-IPA/tipa/pkg/phono"
)

// Picker implements the strategy used to extract pronunciations and
// relative confidences from a dictionary for a given surface form.
//
// The current implementation is intentionally simple and purely
// dictionary‑based. It returns all distinct pronunciations reachable
// from the candidate keys together with a heuristic confidence in
// [0,1]. These per‑variant confidences are later multiplied by the
// pass‑level confidence used by the scanner (strict vs tolerant).
//
// The "line" parameter accepted by PickAll is the full textual line
// that contains the surface span. It is not used yet but is part of
// the API so that future implementations can take broader context
// (e.g. morphosyntactic analysis) into account without changing the
// scanner.
type Picker struct{}

// PickAll returns all distinct pronunciations associated with the
// candidate dictionary keys for a given surface form.
//
// The returned slice is ordered by decreasing confidence. The current
// heuristic is deliberately simple:
//
//   - keys whose NormalizeString form exactly matches the surface
//     normalization receive a base score of 1.0;
//   - other keys (typically brought in by tolerant normalization)
//     are slightly down‑weighted (0.9);
//   - additional pronunciations for the same key are given a small
//     penalty compared to the first pronunciation.
//
// The "line" parameter is accepted for future, context‑sensitive
// heuristics but is not used yet.
func (Picker) PickAll(dict phono.Dictionary, candidateKeys []string, surface string, line string) []phono.AnnotatedPhonetized {
	if len(candidateKeys) == 0 || len(dict) == 0 {
		return nil
	}

	normalizedSurface := phono.NormalizeString(surface)

	options := make([]phono.AnnotatedPhonetized, 0, len(candidateKeys))
	seen := make(map[phono.Phonetized]struct{})

	for _, key := range candidateKeys {
		prons, ok := dict[key]
		if !ok || len(prons) == 0 {
			continue
		}

		normKey := phono.NormalizeString(key)
		keyWeight := 1.0
		if normKey != "" && normKey != normalizedSurface {
			// The candidate key differs from the surface once normalized,
			// which is typically the case in tolerant mode (missing or
			// mismatched diacritics). Give it a slightly lower weight.
			keyWeight = 0.9
		}

		for i, pron := range prons {
			if pron == "" {
				continue
			}

			p := phono.Phonetized(pron)
			if _, dup := seen[p]; dup {
				// Avoid returning the exact same pronunciation twice when
				// it appears under multiple keys.
				continue
			}
			seen[p] = struct{}{}

			pronWeight := 1.0
			if i > 0 {
				// Alternative pronunciations for the same key are kept but
				// slightly down‑weighted compared to the first one.
				pronWeight = 0.95
			}

			options = append(options, phono.AnnotatedPhonetized{
				S: pron,
				C: keyWeight * pronWeight,
			})
		}
	}

	if len(options) == 0 {
		return nil
	}

	// IPA-specific Filter, for "aimable ɛ.mabl | ɛmabl"
	// we retain only "ɛ.mabl" that is more expressive.

	// First pass: record which "base" forms have at least one dotted variant.
	dottedByBase := make(map[string]bool, len(options))
	for _, opt := range options {
		if strings.Contains(opt.S, ".") {
			base := strings.ReplaceAll(opt.S, ".", "")
			dottedByBase[base] = true
		}
	}

	// Second pass: drop non-dotted variants when a dotted one exists.
	filtered := options[:0]
	for _, opt := range options {
		base := strings.ReplaceAll(opt.S, ".", "")
		hasDottedVariant := dottedByBase[base]
		hasDot := strings.Contains(opt.S, ".")

		if !hasDottedVariant || hasDot {
			filtered = append(filtered, opt)
		}
	}

	options = filtered

	// Order by decreasing confidence while preserving the relative order
	// of options with the same score.
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].C == options[j].C {
			return i < j
		}
		return options[i].C > options[j].C
	})

	return options
}
