package g2p

import (
	"context"
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
// each rune position, tries to match **the longest possible dictionary
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
//
// When several dictionary entries / pronunciations match the same
// surface span, all variants are preserved as individual Fragment
// instances that share the same Pos / Len but differ by Phonetized,
// Variant, and (optionally) Confidence. The ordering of variants for a
// given span is stable and reflects their relative confidence.
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

	// picker encapsulates the strategy used to select pronunciations
	// (and their relative confidence) for a given surface span.
	picker Picker
}

// streamedResult is an internal helper type used by the channel‑based
// scanning implementation. It carries a per‑line Result together with
// bookkeeping information needed to reconstruct positions in the
// original multi‑line text.
type streamedResult struct {
	// Result contains the phonetic analysis for a single logical line
	// (without the trailing newline).
	Result Result

	// startOffset is the rune offset of the first rune of Result.Text
	// in the original input string.
	startOffset int

	// hasTrailingNewline reports whether this line in the original text
	// was immediately followed by a '\n' rune. When true, Scan() emits
	// a RawText span for that newline so that behaviour matches the
	// non‑streaming implementation.
	hasTrailingNewline bool
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
		picker:             Picker{},
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
// The returned Fragments slice is sorted by Pos (ascending). For equal
// positions, fragments with longer Len come first; for identical spans
// (same Pos and Len), variants are ordered by decreasing Confidence and
// then by Variant index. RawTexts are merged so that at most one
// RawText covers any contiguous region of unmatched text.
//
// Internally, Scan now uses the same channel‑based, line‑oriented
// implementation as StreamScan and then merges all per‑line results
// back into a single Result that refers to the original multi‑line
// input.
func (d Determinist) Scan(text string, tolerant bool) Result {
	// Use a background context here; callers who need cancellation or
	// incremental consumption should use StreamScan directly.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var allFragments []Fragment
	var allRawTexts []RawText

	// streamScanWithOffsets performs the actual line‑by‑line scanning
	// and sends one streamedResult per logical line.
	for sr := range d.streamScanWithOffsets(ctx, text, tolerant) {
		res := sr.Result

		// Rebase fragment positions from line‑relative offsets to rune
		// offsets in the original text.
		for _, f := range res.Fragments {
			adjusted := f
			adjusted.Pos += sr.startOffset
			allFragments = append(allFragments, adjusted)
		}

		// Same for raw text spans produced while scanning the line.
		for _, rt := range res.RawTexts {
			adjusted := rt
			adjusted.Pos += sr.startOffset
			allRawTexts = append(allRawTexts, adjusted)
		}

		// When the original text had a '\n' right after this line, keep
		// it as a RawText span so that the behaviour matches the original
		// single‑pass Scan implementation.
		if sr.hasTrailingNewline {
			newlinePos := sr.startOffset + len([]rune(res.Text))
			allRawTexts = append(allRawTexts, RawText{
				Text: "\n",
				Pos:  newlinePos,
				Len:  1,
			})
		}
	}

	// Global ordering and merging guarantees.
	sortFragments(allFragments)
	allRawTexts = mergeRawTexts(allRawTexts)

	return Result{
		Text:      text,
		Fragments: allFragments,
		RawTexts:  allRawTexts,
	}
}

// StreamScan scans the input text line by line and returns a read‑only
// channel of Result values.
//
// Each emitted Result corresponds to a single logical line of the
// original text, i.e. the runes between two '\n' delimiters, without
// the trailing newline itself. Fragment.Pos and RawText.Pos are rune
// offsets relative to the beginning of that line.
//
// The returned channel is closed once all lines have been processed or
// the context is cancelled. Cancellation is observed between lines;
// a very long single line is processed atomically.
func (d Determinist) StreamScan(ctx context.Context, text string, tolerant bool) <-chan Result {
	out := make(chan Result)

	internal := d.streamScanWithOffsets(ctx, text, tolerant)

	go func() {
		defer close(out)

		for sr := range internal {
			select {
			case <-ctx.Done():
				return
			case out <- sr.Result:
			}
		}
	}()

	return out
}

// streamScanWithOffsets is the internal streaming implementation used
// by both Scan (which merges back to a single Result) and StreamScan
// (which exposes per‑line Results). It walks the input in rune space,
// splits it on '\n', and for each logical line:
//
//   - runs the full single‑line pipeline (scanText);
//   - sends a streamedResult containing the per‑line Result, its rune
//     start offset in the original text, and whether it was followed
//     by a newline in the original text.
func (d Determinist) streamScanWithOffsets(ctx context.Context, text string, tolerant bool) <-chan streamedResult {
	out := make(chan streamedResult)
	go func() {
		defer close(out)

		if len(text) == 0 {
			// Empty input: nothing to emit, keep Scan behaviour consistent.
			return
		}

		runes := []rune(text)
		n := len(runes)

		lineStart := 0

		for i := 0; i < n; i++ {
			if runes[i] == '\n' {
				// The logical line is the slice [lineStart, i) (possibly empty).
				lineText := string(runes[lineStart:i])

				// Observe cancellation between lines.
				if err := ctx.Err(); err != nil {
					return
				}

				res := d.scanText(lineText, tolerant)

				// This line in the original text is immediately followed
				// by a newline.
				select {
				case <-ctx.Done():
					return
				case out <- streamedResult{
					Result:             res,
					startOffset:        lineStart,
					hasTrailingNewline: true,
				}:
				}

				lineStart = i + 1
			}
		}

		// Final segment after the last newline (if any). When the text
		// ends with '\n', lineStart == n and there is no additional line
		// after the trailing newline, which matches the behaviour of the
		// original non‑streaming Scan.
		if lineStart < n {
			lineText := string(runes[lineStart:n])

			if err := ctx.Err(); err != nil {
				return
			}

			res := d.scanText(lineText, tolerant)

			select {
			case <-ctx.Done():
				return
			case out <- streamedResult{
				Result:             res,
				startOffset:        lineStart,
				hasTrailingNewline: false,
			}:
			}
		}
	}()

	return out
}

// scanText runs the full two‑stage pipeline (langDict + optional
// finalDict) on a single text string and returns the corresponding
// Result. It is the non‑streaming core used for each logical line.
//
// The behaviour is equivalent to the original Scan implementation
// applied to that string: greedy longest‑match scan on langDict,
// optional tolerant pass, then optional rescan of raw spans with
// finalDict, followed by global fragment ordering and raw‑text merge.
func (d Determinist) scanText(text string, tolerant bool) Result {
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
		text,
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
				text,
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
	sortFragments(finalFragments)
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
//
// The "line" parameter is the full line of text being processed (the
// same value that was passed to Scan). It is forwarded to the Picker
// so that more context‑sensitive selection heuristics can be added
// without changing the scanning logic.
func (d Determinist) scan(
	text string,
	dictionary phono.Dictionary,
	normKeyMap phono.KeyMap,
	tolerantKeyMap phono.KeyMap,
	maxKeyLen int,
	tolerant bool,
	line string,
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
		line,
	)

	// If tolerant mode is disabled or everything was recognized, we are done.
	if !tolerant || len(rawTexts) == 0 || len(tolerantKeyMap) == 0 {
		sortFragments(fragments)
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
			line,
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

	sortFragments(tolerantFragments)
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
//     dictionary key map and for which at least one pronunciation is
//     returned by the Picker:
//     - flush any pending RawText preceding i,
//     - emit one Fragment per pronunciation variant, all sharing the
//     same Pos / Len but different Variant / Confidence,
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
	passConfidence float64,
	line string,
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

			// Ask the picker for all available pronunciation variants.
			options := d.picker.PickAll(dictionary, keys, candidate, line)
			if len(options) == 0 {
				// No usable pronunciation for this candidate: keep looking
				// for a shorter match at the same position.
				continue
			}

			// Flush any pending raw text before emitting fragments.
			if currentRawStart != -1 && currentRawStart < i {
				rawTexts = append(rawTexts, RawText{
					Text: string(runes[currentRawStart:i]),
					Pos:  offset + currentRawStart,
					Len:  i - currentRawStart,
				})
				currentRawStart = -1
			}

			// Emit one fragment per pronunciation variant, all sharing the
			// same span but carrying their own Variant index and confidence.
			for variantIndex, opt := range options {
				fragments = append(fragments, Fragment{
					Phonetized: opt.S,
					Pos:        offset + i,
					Len:        l,
					Confidence: passConfidence * opt.C,
					Variant:    variantIndex,
				})
			}

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

// sortFragments orders fragments by position and span length, and, for
// identical spans, by decreasing confidence then increasing variant.
func sortFragments(frags []Fragment) {
	if len(frags) < 2 {
		return
	}

	sort.Slice(frags, func(i, j int) bool {
		fi := frags[i]
		fj := frags[j]

		if fi.Pos == fj.Pos {
			if fi.Len == fj.Len {
				if fi.Confidence == fj.Confidence {
					return fi.Variant < fj.Variant
				}
				return fi.Confidence > fj.Confidence
			}
			return fi.Len > fj.Len
		}
		return fi.Pos < fj.Pos
	})
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
