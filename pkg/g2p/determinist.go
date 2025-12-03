package g2p

import (
	"sort"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/temporal-IPA/tipa/pkg/phono"
)

// Determinist is a greedy, dictionary‑based grapheme‑to‑phoneme (g2p)
// processor.
//
// It works with two dictionaries per language:
//
//   - langDict: a large lexicon that should cover most real‑world tokens.
//   - finalDict: a small "fallback" dictionary used to fill gaps with
//     generic pronunciations for characters, syllables or short chunks.
//
// The scanning logic always walks the input text from left to right and
// tries to cover it with the longest possible dictionary keys. Any text
// that cannot be matched is returned as RawText so that callers can
// inspect or post‑process it.
type Determinist struct {
	// langDict is the main phonetic dictionary for the language.
	langDict phono.Dictionary
	// langDictNormKeyMap maps normalized surface forms to the original
	// langDict keys that share the same normalization.
	langDictNormKeyMap phono.KeyMap

	// finalDict is an optional fallback dictionary that contains short
	// character sequences used to fill holes left by langDict.
	finalDict phono.Dictionary
	// finalDictNormKeyMap is the normalized view of finalDict.
	finalDictNormKeyMap phono.KeyMap
}

// NewDeterminist creates a new Determinist g2p processor instance.
//
// langDict is the main, usually large, lexicon. final may be nil; when
// non‑nil it is used as a second‑stage dictionary to rescan unmatched
// text spans and fill small gaps.
func NewDeterminist(langDict phono.Dictionary, final phono.Dictionary) *Determinist {
	g := &Determinist{
		langDict:            langDict,
		langDictNormKeyMap:  langDict.NormalizedKeys(),
		finalDict:           final,
		finalDictNormKeyMap: final.NormalizedKeys(),
	}
	return g
}

// Scan converts the given text into phonetic fragments and raw spans.
//
// The method runs in up to two stages:
//
//  1. The main dictionary (langDict) is used to scan the whole text
//     using a greedy, longest‑match strategy. This is implemented by
//     scan and scanSegment.
//
//  2. If finalDict is non‑nil, every RawText span left by stage 1 is
//     rescanned using finalDict. Any new fragments found in those
//     spans are merged with the fragments from stage 1 and their
//     positions are adjusted so that every Fragment.Pos / RawText.Pos
//     is expressed in rune offsets relative to the original text.
//
// If tolerant is true, each scan internally performs an additional
// pass that is diacritic‑insensitive. This allows, for example,
// matching "garcon" against a dictionary entry "garçon" in both
// langDict and finalDict.
//
// The returned Fragments slice is sorted by Pos (ascending) and, for
// equal positions, by longer Len first. RawTexts are merged so that
// at most one RawText covers any contiguous region of unmatched text.
func (d Determinist) Scan(text string, tolerant bool) Result {

	////////////////////////////////////////
	// #1 Scan the text using the langDict.
	// This is the most important phase.
	// A well‑adapted dict should not return a lot of raw texts.
	////////////////////////////////////////

	fragments, rawTexts := d.scan(text, d.langDict, d.langDictNormKeyMap, tolerant)

	////////////////////////////////////////////
	// #2 Scan the remaining raw texts with the final dictionary.
	// Each raw text is rescanned and can generate:
	//   - additional fragments (subFragments)
	//   - new raw texts (subRawTexts) that replace the original raw segment
	////////////////////////////////////////////
	var finalFragments []Fragment
	var finalRawText []RawText

	// The second stage only runs if there are raw texts and a finalDict.
	if len(rawTexts) > 0 && d.finalDict != nil {
		// Start from the fragments already found in phase #1.
		finalFragments = fragments

		// finalRawText will be rebuilt from the rawTexts slice,
		// replacing each original RawText by its eventual subRawTexts.
		finalRawText = make([]RawText, 0, len(rawTexts))

		// Scan each raw text with the final dictionary.
		for _, rawText := range rawTexts {
			subFragments, subRawTexts := d.scan(rawText.Text, d.finalDict, d.finalDictNormKeyMap, tolerant)

			// If some fragments are found inside this raw text,
			// add them to the final fragment list with adjusted positions.
			for _, sub := range subFragments {
				// Readjust the sub fragment positions to match the original text position.
				sub.Pos = rawText.Pos + sub.Pos
				finalFragments = append(finalFragments, sub)
			}

			// If the second scan still has raw texts, they replace the original rawText.
			if len(subRawTexts) > 0 {
				for _, sub := range subRawTexts {
					// Readjust the sub raw text positions to match the original text position.
					sub.Pos = rawText.Pos + sub.Pos
					finalRawText = append(finalRawText, sub)
				}
			} else {
				// No better decomposition was found for this raw text,
				// so keep the original one.
				finalRawText = append(finalRawText, rawText)
			}
		}
	} else {
		// No second stage: keep the initial scan result as‑is.
		finalFragments = fragments
		finalRawText = rawTexts
	}

	// Ensure the same ordering and merging guarantees as scan().
	sort.Slice(finalFragments, func(i, j int) bool {
		if finalFragments[i].Pos == finalFragments[j].Pos {
			return finalFragments[i].Len > finalFragments[j].Len
		}
		return finalFragments[i].Pos < finalFragments[j].Pos
	})
	finalRawText = mergeRawTexts(finalRawText)

	return Result{
		Fragments: finalFragments,
		RawTexts:  finalRawText,
	}
}

// scan is a brute‑force scanner that walks the input text from left to
// right and greedily selects, at each position, the longest dictionary
// key that matches the text.
//
// The scan operates on Unicode code points (runes). Fragment.Pos and
// Fragment.Len (as well as RawText.Pos and RawText.Len) are expressed
// as rune offsets in the original text.
//
// The algorithm runs in one or two passes:
//
//   - First pass (always enabled): the text is scanned using the
//     dictionary and its normalized key map with phono.NormalizeString.
//     Every successful match produces a Fragment. Any text that cannot
//     be matched is emitted as RawText.
//
//   - Second pass (tolerant mode only): the RawText ranges from the
//     first pass are scanned again using a more tolerant normalization
//     that additionally strips diacritic marks from both the
//     dictionary keys and the candidate substrings. This allows, for
//     example, matching "garcon" against a dictionary entry "garçon".
//     RawText segments that are still unmatched after this pass are
//     kept as‑is.
//
// For every contiguous piece of unmatched text at most one RawText is
// returned: neighbouring RawText blocks are merged together.
func (d Determinist) scan(text string, dictionary phono.Dictionary, normKeyMap phono.KeyMap, tolerant bool) ([]Fragment, []RawText) {

	// Strict pass.
	fragments, rawTexts := d.scanSegment(text, 0, dictionary, normKeyMap, phono.NormalizeString, 1.0)
	// If the tolerant mode is disabled or everything was recognized, we are done.
	if !tolerant || len(rawTexts) == 0 {
		sort.Slice(fragments, func(i, j int) bool {
			if fragments[i].Pos == fragments[j].Pos {
				return fragments[i].Len > fragments[j].Len
			}
			return fragments[i].Pos < fragments[j].Pos
		})
		rawTexts = mergeRawTexts(rawTexts)
		return fragments, rawTexts
	}

	// Build a diacritic‑insensitive view of the dictionary and re‑process
	// unrecognized spans.
	tolerantNormalized := d.buildDiacriticInsensitiveKeyMap(normKeyMap)
	tolerantFragments := make([]Fragment, 0, len(fragments))
	tolerantFragments = append(tolerantFragments, fragments...)
	tolerantRawTexts := make([]RawText, 0, len(rawTexts))

	for _, rt := range rawTexts {
		// slightly lower confidence for tolerant matches
		segFrags, segRaws := d.scanSegment(rt.Text, rt.Pos, dictionary, tolerantNormalized, tolerantNormalize, 0.9)
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

// scanSegment performs a greedy, longest‑match scan of a single text
// segment using the provided normalization function and dictionary
// view. The offset parameter indicates the rune position of the first
// rune of text within the original input string.
//
// The function advances strictly from left to right: it never rewinds
// the scanning index. This guarantees that, even when a dictionary
// entry cannot be found for some intermediate characters, later
// entries can still be matched. For example, with a dictionary that
// contains "benoit" and "le" but not "gros", scanning the text
// "Le GrosBenoit" will:
//
//   - Emit a Fragment for "Le".
//   - Emit a RawText block for " Gros" (space + "Gros").
//   - Emit a Fragment for "Benoit".
func (d Determinist) scanSegment(text string, offset int, dictionary phono.Dictionary, normalized phono.KeyMap, normalizeCandidate func(string) string, confidence float64) ([]Fragment, []RawText) {
	runes := []rune(text)
	n := len(runes)
	fragments := make([]Fragment, 0)
	rawTexts := make([]RawText, 0)
	maxLen := dictionary.MaxKeyLen()
	currentRawStart := -1
	for i := 0; i < n; {
		// Do not attempt to search longer keys than both the configured
		// maximum and the remaining number of runes in this segment.
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

			ipa := d.pickIPA(dictionary, keys, candidate)

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
// returns the first pronunciation of the first key whose
// NormalizeString form matches the surface; if none match exactly, it
// falls back to the first key that has at least one pronunciation.
func (d Determinist) pickIPA(dict phono.Dictionary, candidateKeys []string, surface string) phono.IPA {
	if len(candidateKeys) == 0 {
		return ""
	}
	normalizedSurface := phono.NormalizeString(surface)
	// Prefer keys whose NormalizeString form is identical to the surface.
	for _, k := range candidateKeys {
		if phono.NormalizeString(k) == normalizedSurface {
			if prons := dict[k]; len(prons) > 0 {
				return prons[0]
			}
		}
	}
	// Fallback: first key that has at least one pronunciation.
	for _, k := range candidateKeys {
		if prons := dict[k]; len(prons) > 0 {
			return prons[0]
		}
	}
	return ""
}

// buildDiacriticInsensitiveKeyMap constructs a diacritic‑insensitive view of
// the dictionary keys. Multiple normalized keys can map to the same
// tolerant key; in that case, their original spellings are concatenated
// in the resulting slice.
func (d Determinist) buildDiacriticInsensitiveKeyMap(keyMap phono.KeyMap) phono.KeyMap {
	tolerant := make(phono.KeyMap, len(keyMap))
	for normKey, keys := range keyMap {
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

// removeDiacritics returns a copy of s where all non‑spacing marks
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
