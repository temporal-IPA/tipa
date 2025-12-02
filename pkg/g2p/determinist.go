package g2p

import (
	"sort"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/temporal-IPA/tipa/pkg/phono"
)

type Determinist struct {
	dictionary phono.Dictionary
	normalized map[string][]string
	maxKeyLen  int
	tolerant   bool
}

func NewDeterminist(dictionary phono.Dictionary, tolerant bool) *Determinist {
	normalized := dictionary.NormalizedKeys()
	maxKeyLen := 0
	for key := range normalized {
		if l := utf8.RuneCountInString(key); l > maxKeyLen {
			maxKeyLen = l
		}
	}
	g := &Determinist{
		dictionary: dictionary,
		normalized: normalized,
		maxKeyLen:  maxKeyLen,
		tolerant:   tolerant,
	}
	return g
}

// Scan is a brute‑force scanner that walks the input text from left to
// right and greedily selects, at each position, the longest dictionary
// entry that matches the text.
//
// The scan operates on Unicode code points (runes). Fragment.Pos and
// Fragment.Len (as well as RawText.Pos and RawText.Len) are expressed
// as rune offsets in the original text.
//
// The algorithm runs in one or two passes:
//
//   - First pass (always enabled): the text is scanned using the
//     dictionary keys normalized with phono.NormalizeString. Every
//     successful match produces a Fragment. Any text that cannot be
//     matched is emitted as RawText.
//
//   - Second pass (tolerant mode only): the RawText ranges from the
//     first pass are scanned again using a more tolerant normalization
//     that additionally strips diacritic marks from both the dictionary
//     keys and the candidate substrings. This allows, for example,
//     matching "garcon" against a dictionary entry "garçon". RawText
//     segments that are still unmatched after this pass are kept as-is.
//
// For every contiguous piece of unmatched text at most one RawText is
// returned: neighbouring RawText blocks are merged together.
func (g Determinist) Scan(text string) ([]Fragment, []RawText) {
	// Strict pass.
	fragments, rawTexts := g.scanSegment(
		text,
		0,
		g.normalized,
		phono.NormalizeString,
		1.0,
	)

	// If tolerant mode is disabled or everything was recognized, we are done.
	if !g.tolerant || len(rawTexts) == 0 {
		sort.Slice(fragments, func(i, j int) bool {
			if fragments[i].Pos == fragments[j].Pos {
				return fragments[i].Len > fragments[j].Len
			}
			return fragments[i].Pos < fragments[j].Pos
		})
		rawTexts = mergeRawTexts(rawTexts)
		return fragments, rawTexts
	}

	// Build a diacritic-insensitive view of the dictionary and re-process
	// unrecognized spans.
	tolerantNormalized := g.buildTolerantNormalized()

	tolerantFragments := make([]Fragment, 0, len(fragments))
	tolerantFragments = append(tolerantFragments, fragments...)

	tolerantRawTexts := make([]RawText, 0, len(rawTexts))

	for _, rt := range rawTexts {
		segFrags, segRaws := g.scanSegment(
			rt.Text,
			rt.Pos,
			tolerantNormalized,
			tolerantNormalize,
			0.9, // slightly lower confidence for tolerant matches
		)
		if len(segFrags) == 0 {
			// Nothing could be recognized in tolerant mode, keep the
			// original raw block.
			tolerantRawTexts = append(tolerantRawTexts, rt)
			continue
		}
		tolerantFragments = append(tolerantFragments, segFrags...)
		tolerantRawTexts = append(tolerantRawTexts, segRaws...)
	}

	sort.Slice(tolerantFragments, func(i, j int) bool {
		if tolerantFragments[i].Pos == tolerantFragments[j].Pos {
			return tolerantFragments[i].Len > tolerantFragments[j].Len
		}
		return tolerantFragments[i].Pos < tolerantFragments[j].Pos
	})
	tolerantRawTexts = mergeRawTexts(tolerantRawTexts)

	return tolerantFragments, tolerantRawTexts
}

// scanSegment performs a greedy, longest-match scan of a single text
// segment using the provided normalization function and dictionary
// view. The offset parameter indicates the rune position of the first
// rune of text within the original input string.
func (g Determinist) scanSegment(
	text string,
	offset int,
	normalized map[string][]string,
	normalizeCandidate func(string) string,
	confidence float64,
) ([]Fragment, []RawText) {
	runes := []rune(text)
	n := len(runes)
	fragments := make([]Fragment, 0)
	rawTexts := make([]RawText, 0)

	currentRawStart := -1

	for i := 0; i < n; {
		// Do not attempt to search longer keys than both the configured
		// maximum and the remaining number of runes in this segment.
		maxLen := g.maxKeyLen
		remaining := n - i
		if maxLen > remaining {
			maxLen = remaining
		}

		found := false
		for l := maxLen; l > 0; l-- {
			candidate := string(runes[i : i+l])
			normCandidate := normalizeCandidate(candidate)
			keys, ok := normalized[normCandidate]
			if !ok {
				continue
			}

			// Flush any pending raw text before emitting a fragment.
			if currentRawStart != -1 && currentRawStart < i {
				rawTexts = append(rawTexts, RawText{
					Text: string(runes[currentRawStart:i]),
					Pos:  offset + currentRawStart,
					Len:  i - currentRawStart,
				})
				currentRawStart = -1
			}

			ipa := g.pickIPA(keys, candidate)

			fragments = append(fragments, Fragment{
				IPA:        ipa,
				Pos:        offset + i,
				Len:        l,
				Confidence: confidence,
			})

			i += l
			found = true
			break
		}

		if !found {
			// No dictionary entry begins at this rune: mark / extend raw text.
			if currentRawStart == -1 {
				currentRawStart = i
			}
			i++
		}
	}

	// Flush the trailing raw text, if any.
	if currentRawStart != -1 && currentRawStart < n {
		rawTexts = append(rawTexts, RawText{
			Text: string(runes[currentRawStart:n]),
			Pos:  offset + currentRawStart,
			Len:  n - currentRawStart,
		})
	}

	return fragments, rawTexts
}

// pickIPA selects the IPA transcription associated with one of the
// dictionary keys that matched a given surface form. It currently
// returns the first pronunciation of the first key whose normalized
// form matches the surface; if none match exactly, it falls back to
// the first key that has at least one pronunciation.
func (g Determinist) pickIPA(candidateKeys []string, surface string) phono.IPA {
	if len(candidateKeys) == 0 {
		return ""
	}

	normalizedSurface := phono.NormalizeString(surface)

	// Prefer keys whose normalized form is identical to the surface.
	for _, k := range candidateKeys {
		if phono.NormalizeString(k) == normalizedSurface {
			if prons := g.dictionary[k]; len(prons) > 0 {
				return prons[0]
			}
		}
	}

	// Fallback: first key that has at least one pronunciation.
	for _, k := range candidateKeys {
		if prons := g.dictionary[k]; len(prons) > 0 {
			return prons[0]
		}
	}

	return ""
}

// buildTolerantNormalized constructs a diacritic-insensitive view of
// the dictionary keys. Multiple normalized keys can map to the same
// tolerant key; in that case, their original spellings are concatenated.
func (g Determinist) buildTolerantNormalized() map[string][]string {
	tolerant := make(map[string][]string, len(g.normalized))
	for normKey, keys := range g.normalized {
		tKey := tolerantNormalize(normKey)
		tolerant[tKey] = append(tolerant[tKey], keys...)
	}
	return tolerant
}

// tolerantNormalize applies the standard dictionary normalization and
// additionally strips diacritic marks. It is used both when building
// the tolerant dictionary view and when normalizing candidate substrings.
func tolerantNormalize(s string) string {
	return phono.NormalizeString(removeDiacritics(s))
}

// removeDiacritics returns a copy of s where all non-spacing marks
// (Unicode category Mn) have been removed after canonical decomposition.
// This makes "é" and "e" compare equal, as well as "garçon" and "garcon".
func removeDiacritics(s string) string {
	// Decompose into base characters + combining marks.
	decomposed := norm.NFD.String(s)

	out := make([]rune, 0, len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			// Skip all combining marks.
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

// mergeRawTexts merges adjacent RawText entries that represent
// consecutive ranges in the original text (i.e. where previous.Pos +
// previous.Len == next.Pos). The input slice is not assumed to be
// ordered; it is sorted by Pos before merging.
func mergeRawTexts(raws []RawText) []RawText {
	if len(raws) < 2 {
		return raws
	}

	sort.Slice(raws, func(i, j int) bool {
		return raws[i].Pos < raws[j].Pos
	})

	merged := make([]RawText, 0, len(raws))
	current := raws[0]

	for i := 1; i < len(raws); i++ {
		next := raws[i]
		if current.Pos+current.Len == next.Pos {
			current.Text += next.Text
			current.Len += next.Len
			continue
		}
		merged = append(merged, current)
		current = next
	}
	merged = append(merged, current)

	return merged
}
