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
//   - langDict: a large lexicon that should cover most real‑world tokens
//     and expressions. Keys can be single words ("se") or multi‑word
//     sequences that include spaces and punctuation
//     ("à cette différence près que").
//
//   - finalDict: a smaller "fallback" dictionary used to fill gaps with
//     generic pronunciations for characters, syllables, or short chunks.
//
// The scanner always walks the input text from left to right and, at
// each rune position, tries to match the **longest possible dictionary
// key**, up to the precomputed MaxKeyLen of the dictionary. This gives
// multi‑word entries priority over shorter sub‑entries, e.g. it will
// match:
//
//	"à cette différence près que"
//
// as a single fragment instead of matching shorter keys like "à" or
// "à cette", as long as the full expression exists in the dictionary.
//
// Matching is done on normalized forms:
//
//   - First stage: NormalizeString (lowercased, trimmed).
//   - Optional tolerant stage: diacritic‑insensitive normalization
//     (NormalizeString + diacritic removal), used only on spans that
//     were not recognized in the strict pass.
//
// Any text that cannot be matched is returned as RawText so that callers
// can inspect or post‑process it.
type Determinist struct {
	// langDict is the main phonetic dictionary for the language.
	langDict phono.Dictionary
	// langDictNormKeyMap maps normalized surface forms to the original
	// langDict keys that share the same normalization (strict lowercase view).
	langDictNormKeyMap phono.KeyMap
	// langTolerantKeyMap is the diacritic‑insensitive view of langDict.
	// It is built once in NewDeterminist and reused for every tolerant scan.
	langTolerantKeyMap phono.KeyMap
	// langMaxKeyLen caches the maximum key length (in runes) of langDict.
	langMaxKeyLen int

	// finalDict is an optional fallback dictionary that contains short
	// character sequences used to fill holes left by langDict.
	finalDict phono.Dictionary
	// finalDictNormKeyMap is the normalized (lowercased) view of finalDict.
	finalDictNormKeyMap phono.KeyMap
	// finalTolerantKeyMap is the diacritic‑insensitive view of finalDict.
	finalTolerantKeyMap phono.KeyMap
	// finalMaxKeyLen caches the maximum key length (in runes) of finalDict.
	finalMaxKeyLen int
}

// NewDeterminist creates a new Determinist g2p processor instance.
//
// langDict is the main, usually large, lexicon. final may be nil; when
// non‑nil it is used as a second‑stage dictionary to rescan unmatched
// text spans and fill small gaps.
//
// For efficiency, the constructor precomputes:
//   - The normalized key maps for langDict and finalDict.
//   - Diacritic‑insensitive key maps (for tolerant scans).
//   - MaxKeyLen for both dictionaries.
func NewDeterminist(langDict phono.Dictionary, final phono.Dictionary) *Determinist {
	langNorm := langDict.NormalizedKeys()
	g := &Determinist{
		langDict:           langDict,
		langDictNormKeyMap: langNorm,
		langMaxKeyLen:      langDict.MaxKeyLen(),
		finalDict:          final,
	}

	if len(langNorm) > 0 {
		g.langTolerantKeyMap = buildDiacriticInsensitiveKeyMap(langNorm)
	}

	if final != nil {
		finalNorm := final.NormalizedKeys()
		g.finalDictNormKeyMap = finalNorm
		g.finalMaxKeyLen = final.MaxKeyLen()
		if len(finalNorm) > 0 {
			g.finalTolerantKeyMap = buildDiacriticInsensitiveKeyMap(finalNorm)
		}
	}

	return g
}

// Scan converts the given text into phonetic fragments and raw spans.
//
// The method runs in up to two dictionary stages, each of which can
// itself perform one or two normalization passes:
//
//  1. Main dictionary (langDict):
//     - Strict pass (always): greedy longest‑match scan using
//     NormalizeString (lowercase + trim).
//     - Tolerant pass (optional): only if tolerant==true and some
//     spans were left as RawText by the strict pass. Those raw spans
//     are rescanned using a diacritic‑insensitive normalization.
//     This makes "garcon" match a dictionary entry "garçon", etc.
//
//  2. Final dictionary (finalDict, optional):
//     - The RawText spans left after stage 1 are rescanned with
//     finalDict using the same strict/tolerant pipeline.
//     - Any fragments found are merged with the original fragments,
//     with positions adjusted so that Fragment.Pos / RawText.Pos
//     are expressed as rune offsets in the original text.
//
// The returned Fragments slice is sorted by Pos (ascending) and, for
// equal positions, by longer Len first. RawTexts are merged so that
// at most one RawText covers any contiguous region of unmatched text.
func (d Determinist) Scan(text string, tolerant bool) Result {
	////////////////////////////////////////
	// #1 Scan the text using the langDict.
	// This is the main phase and should recognize the majority of the text.
	////////////////////////////////////////

	fragments, rawTexts := d.scan(
		text,
		d.langDict,
		d.langDictNormKeyMap,
		d.langTolerantKeyMap,
		d.langMaxKeyLen,
		tolerant,
	)

	////////////////////////////////////////////
	// #2 Scan the remaining raw texts with the final dictionary (if any).
	// Each raw text is rescanned and can generate:
	//   - additional fragments (subFragments)
	//   - new raw texts (subRawTexts) that replace the original raw segment
	////////////////////////////////////////////

	var finalFragments []Fragment
	var finalRawText []RawText

	if len(rawTexts) > 0 && d.finalDict != nil {
		// Start from the fragments already found in phase #1.
		finalFragments = fragments

		// finalRawText will be rebuilt from the rawTexts slice,
		// replacing each original RawText by its eventual subRawTexts.
		finalRawText = make([]RawText, 0, len(rawTexts))

		for _, rawText := range rawTexts {
			subFragments, subRawTexts := d.scan(
				rawText.Text,
				d.finalDict,
				d.finalDictNormKeyMap,
				d.finalTolerantKeyMap,
				d.finalMaxKeyLen,
				tolerant,
			)

			// If fragments are found inside this raw text, add them to the
			// final fragment list with adjusted positions.
			for _, sub := range subFragments {
				sub.Pos = rawText.Pos + sub.Pos
				finalFragments = append(finalFragments, sub)
			}

			// If the second scan still has raw texts, they replace the original rawText.
			if len(subRawTexts) > 0 {
				for _, sub := range subRawTexts {
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
		Text:      text,
		Fragments: finalFragments,
		RawTexts:  finalRawText,
	}
}

// scan applies the two‑pass (strict + tolerant) pipeline for a single
// dictionary:
//
//   - strict pass: NormalizeString (lowercase) view of the dictionary;
//   - tolerant pass (optional): diacritic‑insensitive view, applied
//     only to the RawText spans left by the strict pass.
//
// The dictionary‑specific parameters (key maps and MaxKeyLen) are
// provided so that the same logic can be reused for langDict and
// finalDict.
func (d Determinist) scan(
	text string,
	dictionary phono.Dictionary,
	normKeyMap phono.KeyMap,
	tolerantKeyMap phono.KeyMap,
	maxKeyLen int,
	tolerant bool,
) ([]Fragment, []RawText) {
	// Strict pass (lowercase).
	fragments, rawTexts := d.scanSegment(
		text,
		0,
		dictionary,
		normKeyMap,
		maxKeyLen,
		phono.NormalizeString,
		1.0,
	)

	// If tolerant mode is disabled or everything was recognized, we are done.
	if !tolerant || len(rawTexts) == 0 || len(tolerantKeyMap) == 0 {
		sort.Slice(fragments, func(i, j int) bool {
			if fragments[i].Pos == fragments[j].Pos {
				return fragments[i].Len > fragments[j].Len
			}
			return fragments[i].Pos < fragments[j].Pos
		})
		rawTexts = mergeRawTexts(rawTexts)
		return fragments, rawTexts
	}

	// Tolerant pass (diacritic‑insensitive) on the remaining RawText spans.
	tolerantFragments := make([]Fragment, 0, len(fragments))
	tolerantFragments = append(tolerantFragments, fragments...)
	tolerantRawTexts := make([]RawText, 0, len(rawTexts))

	for _, rt := range rawTexts {
		// Slightly lower confidence for tolerant matches.
		segFrags, segRaws := d.scanSegment(
			rt.Text,
			rt.Pos,
			dictionary,
			tolerantKeyMap,
			maxKeyLen,
			tolerantNormalize,
			0.9,
		)

		if len(segFrags) == 0 {
			// Nothing could be recognized in tolerant mode: keep the
			// original raw block as‑is.
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

// scanSegment performs a greedy longest‑match scan of a single text
// segment using the provided normalization function and dictionary view.
// The offset parameter indicates the rune position of the first rune of
// text within the original input string.
//
// The algorithm is purely left‑to‑right and operates on raw runes,
// without pre‑splitting into "words". This allows dictionary entries to
// span spaces and punctuation:
//
//	"à cette différence près que"
//
// The core logic at each rune position i is:
//
//  1. If the rune is whitespace, it is always treated as RawText and
//     merged with neighbouring raw spans.
//
//  2. Otherwise, try all candidate substrings starting at i whose
//     length L satisfies 1 <= L <= MaxKeyLen and whose last rune is
//     **not** whitespace. Candidates are tried in descending order of
//     length, so the longest matching dictionary key always wins.
//
//  3. For the first candidate whose normalized form is found in the
//     dictionary key map:
//     - flush any pending RawText preceding i,
//     - emit a Fragment covering exactly that substring,
//     - advance i by L and repeat.
//
//  4. If no candidate matches, the current rune is absorbed into the
//     current RawText span and i is advanced by one.
//
// This strategy ensures that:
//   - multi‑word expressions are recognized as a single fragment when
//     they are present in the dictionary;
//   - shorter keys such as "à" or "à cette" are only used when no
//     longer key starting at the same position exists;
//   - trailing spaces are never absorbed into fragments just because
//     NormalizeString trims them (matches never start or end on
//     whitespace).
func (d Determinist) scanSegment(
	text string,
	offset int,
	dictionary phono.Dictionary,
	normalized phono.KeyMap,
	maxKeyLen int,
	normalizeCandidate func(string) string,
	confidence float64,
) ([]Fragment, []RawText) {
	runes := []rune(text)
	n := len(runes)

	fragments := make([]Fragment, 0)
	rawTexts := make([]RawText, 0)

	if n == 0 {
		return fragments, rawTexts
	}

	// If the dictionary is empty or has no usable keys, the whole segment
	// is raw.
	if maxKeyLen <= 0 || len(normalized) == 0 || len(dictionary) == 0 {
		rawTexts = append(rawTexts, RawText{
			Text: text,
			Pos:  offset,
			Len:  n,
		})
		return fragments, rawTexts
	}

	currentRawStart := -1
	i := 0

	for i < n {
		r := runes[i]

		// Whitespace is never part of a dictionary match boundary: we keep
		// it as raw context. This also prevents NormalizeString's trimming
		// from extending matches over spaces.
		if unicode.IsSpace(r) {
			if currentRawStart == -1 {
				currentRawStart = i
			}
			i++
			continue
		}

		remaining := n - i
		lmax := maxKeyLen
		if lmax > remaining {
			lmax = remaining
		}

		found := false

		// Try candidates from the longest possible down to length 1.
		for l := lmax; l > 0; l-- {
			// Do not allow matches that end on whitespace: this avoids
			// fragments swallowing trailing spaces that would be trimmed
			// away by NormalizeString.
			if unicode.IsSpace(runes[i+l-1]) {
				continue
			}

			candidate := string(runes[i : i+l])
			normCandidate := normalizeCandidate(candidate)
			keys, ok := normalized[normCandidate]
			if !ok || len(keys) == 0 {
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

			phonetized := d.pickPhon(dictionary, keys, candidate)
			fragments = append(fragments, Fragment{
				Phonetized: phonetized,
				Pos:        offset + i,
				Len:        l,
				Confidence: confidence,
			})

			i += l
			found = true
			break
		}

		if !found {
			// No dictionary entry begins at this rune: extend raw text.
			if currentRawStart == -1 {
				currentRawStart = i
			}
			i++
		}
	}

	// Flush trailing raw text, if any.
	if currentRawStart != -1 && currentRawStart < n {
		rawTexts = append(rawTexts, RawText{
			Text: string(runes[currentRawStart:n]),
			Pos:  offset + currentRawStart,
			Len:  n - currentRawStart,
		})
	}

	return fragments, rawTexts
}

// pickPhon selects the IPA transcription associated with one of the
// dictionary keys that matched a given surface form.
//
// In the strict (lowercase) pass, all candidateKeys share the same
// NormalizeString form as the surface. In the tolerant pass, several
// keys that differ only by diacritics (e.g. "garçon" and "garcon")
// can be grouped together. In that case we prefer the key whose
// NormalizeString form is closest to the surface normalization.
//
// The function currently returns the first pronunciation of the
// selected key.
func (d Determinist) pickPhon(dict phono.Dictionary, candidateKeys []string, surface string) phono.Phonetized {
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
//
// It takes an existing normalized key map (NormalizeString view) and
// re‑indexes it using tolerantNormalize, so that:
//
//   - "garçon" and "garcon" share the same tolerant key "garcon".
func buildDiacriticInsensitiveKeyMap(keyMap phono.KeyMap) phono.KeyMap {
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
