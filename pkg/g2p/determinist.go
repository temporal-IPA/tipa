package g2p

import (
	"context"
	"sort"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/temporal-IPA/tipa/pkg/phono"
)

// DeterministOptions control the behavior of a single Determinist run.
//
// Options are attached to a Determinist instance and can also be
// overridden per call via ScanWithOptions.
type DeterministOptions struct {
	// DiacriticInsensitive enables the diacritic‑insensitive matching pass.
	//
	// When true, spans that remain unmatched after the strict pass are
	// rescanned using a diacritic‑insensitive normalization so that
	// "garcon" can match "garçon", etc.
	DiacriticInsensitive bool

	// SingleGraphemeOnlyAsWord restricts the use of single‑rune dictionary
	// entries to isolated one‑letter words.
	//
	// When true, entries whose key is a single rune are only matched when
	// that rune does not appear inside a longer alphanumeric token. This
	// prevents single‑letter keys from being used to decompose arbitrary
	// unknown words character by character, while still allowing them for
	// genuine one‑letter words such as "a".
	SingleGraphemeOnlyAsWord bool
}

// Determinist is a greedy, dictionary‑based grapheme‑to‑phoneme (g2p)
// processor.
//
// It works with a single dictionary per instance. To model multi‑stage
// pipelines (large lexicon, then tolerant fallback, then grapheme /
// diphone dictionary), instantiate several Determinist processors with
// different dictionaries and options and chain them using the Processor
// interface:
//
//	res0 := Result{Text: text, RawTexts: []RawText{{Text: text, Pos: 0, Len: len([]rune(text))}}}
//	res1 := detStrict.Apply(res0)
//	res2 := detTolerant.Apply(res1)
//	res3 := detGraphemes.Apply(res2)
//
// Each stage only processes the RawTexts left by the previous ones.
//
// Matching is done on normalized forms:
//
//   - strict pass: NormalizeString (lowercased, trimmed);
//   - optional tolerant pass: diacritic‑insensitive normalization
//     (NormalizeString + diacritic removal), used only on spans that
//     were not recognized in the strict pass.
//
// Any text that cannot be matched is returned as RawText so that
// callers can inspect or post‑process it.
//
// When several dictionary entries / pronunciations match the same
// surface span, all variants are preserved as individual Fragment
// instances that share the same Pos / Len but differ by Phonetized,
// Variant, and Confidence. The ordering of variants for a given span
// is stable and reflects their relative confidence.
type Determinist struct {
	// langDict is the main phonetic dictionary for the processor.
	langDict phono.Dictionary
	// langDictNormKeyMap maps normalized surface forms to the original
	// dictionary keys that share the same normalization (strict lowercase view).
	langDictNormKeyMap phono.KeyMap
	// langTolerantKeyMap is the diacritic‑insensitive view of langDict.
	// It is built once in the constructor and reused for every tolerant pass.
	langTolerantKeyMap phono.KeyMap
	// langMaxKeyLen caches the maximum key length (in runes) of langDict.
	langMaxKeyLen int

	// options is the default configuration used by Scan / Apply.
	options DeterministOptions

	// picker encapsulates the strategy used to select pronunciations
	// (and their relative confidence) for a given surface span.
	picker Picker
}

// Ensure Determinist implements the pipeline interfaces.
var (
	_ Processor            = (*Determinist)(nil)
	_ CancellableProcessor = (*Determinist)(nil)
)

// DefaultDeterministOptions are the defaults used by NewDeterminist.
//
// They correspond to a strict scan:
//   - DiacriticInsensitive: false (no tolerant pass)
//   - SingleGraphemeOnlyAsWord: false (single‑rune entries may be used
//     inside longer tokens).
var DefaultDeterministOptions = DeterministOptions{
	DiacriticInsensitive:     false,
	SingleGraphemeOnlyAsWord: false,
}

// NewDeterminist creates a new Determinist with the given dictionary
// and DefaultDeterministOptions (strict mode).
func NewDeterminist(langDict phono.Dictionary) *Determinist {
	return NewDeterministWithOptions(langDict, DefaultDeterministOptions)
}

// NewDeterministWithOptions creates a Determinist bound to langDict and
// configured with the provided options.
func NewDeterministWithOptions(langDict phono.Dictionary, opts DeterministOptions) *Determinist {
	langNorm := langDict.NormalizedKeys()
	d := &Determinist{
		langDict:           langDict,
		langDictNormKeyMap: langNorm,
		langMaxKeyLen:      langDict.MaxKeyLen(),
		options:            opts,
		picker:             Picker{},
	}

	if len(langNorm) > 0 {
		d.langTolerantKeyMap = buildDiacriticInsensitiveKeyMap(langNorm)
	}

	return d
}

// Options returns the current default options of the Determinist.
func (d *Determinist) Options() DeterministOptions {
	return d.options
}

// SetOptions changes the default options used by Scan / Apply.
func (d *Determinist) SetOptions(opts DeterministOptions) {
	d.options = opts
}

// Scan is a convenience helper that runs the Determinist on raw text
// using its current default options.
//
// It is equivalent to:
//
//	opts := d.Options()
//	d.ScanWithOptions(text, opts)
func (d *Determinist) Scan(text string) Result {
	return d.ScanWithOptions(text, d.options)
}

// ScanWithOptions converts the given text into phonetic fragments and
// raw spans, using the provided options.
//
// It is implemented on top of the Processor API: the text is wrapped
// in a Result that contains a single RawText covering the entire
// string, and then Apply is called with the given options.
func (d *Determinist) ScanWithOptions(text string, opts DeterministOptions) Result {
	initial := Result{
		Text: text,
	}

	runes := []rune(text)
	if len(runes) > 0 {
		initial.RawTexts = []RawText{{
			Text: text,
			Pos:  0,
			Len:  len(runes),
		}}
	}

	return d.applyWithOptions(initial, opts)
}

// Apply implements the Processor interface.
//
// It scans only the RawTexts of the input Result using the
// Determinist's current options and dictionary:
//
//   - existing Fragments are preserved;
//   - new Fragments are added for portions of RawTexts that can be
//     recognized;
//   - RawTexts are replaced by the unmatched portions of those spans.
//
// This makes it easy to chain multiple Determinist instances with
// different dictionaries / options.
func (d *Determinist) Apply(input Result) Result {
	return d.applyWithOptions(input, d.options)
}

// StreamApply implements the CancellableProcessor interface.
//
// The current implementation emits a single Result on the returned
// channel. Cancellation is observed before and after the processing.
func (d *Determinist) StreamApply(ctx context.Context, input Result) <-chan Result {
	out := make(chan Result, 1)

	go func() {
		defer close(out)

		select {
		case <-ctx.Done():
			return
		default:
		}

		res := d.applyWithOptions(input, d.options)

		select {
		case <-ctx.Done():
			return
		case out <- res:
		}
	}()

	return out
}

// applyWithOptions is the internal implementation used by ScanWithOptions,
// Apply, and StreamApply.
//
// It processes all RawTexts of the input Result independently, using
// the same dictionary and options, then merges the newly discovered
// fragments and leftover raw spans.
func (d *Determinist) applyWithOptions(input Result, opts DeterministOptions) Result {
	// Nothing to do if there is no text or no raw spans.
	if len(input.Text) == 0 || len(input.RawTexts) == 0 {
		return input
	}

	out := input

	// Start from existing fragments.
	out.Fragments = make([]Fragment, len(input.Fragments))
	copy(out.Fragments, input.Fragments)

	newRawTexts := make([]RawText, 0, len(input.RawTexts))

	for _, raw := range input.RawTexts {
		// Scan the raw span in its own coordinate system (positions start at 0).
		frag, leftover := d.scan(raw.Text, opts, input.Text)

		// Rebase fragment positions to the original text coordinates.
		for _, f := range frag {
			f.Pos += raw.Pos
			out.Fragments = append(out.Fragments, f)
		}

		// Rebase leftover raw spans as well.
		if len(leftover) == 0 {
			continue
		}
		for _, rt := range leftover {
			rt.Pos += raw.Pos
			newRawTexts = append(newRawTexts, rt)
		}
	}

	// Global ordering and merging guarantees.
	sortFragments(out.Fragments)
	out.RawTexts = mergeRawTexts(newRawTexts)

	return out
}

// scan applies the two‑pass (strict + tolerant) pipeline for a single
// dictionary on the given text string, using the Determinist's
// dictionary views and the provided options.
//
// Positions in the returned fragments and raw spans are rune offsets
// relative to the beginning of text.
func (d *Determinist) scan(
	text string,
	opts DeterministOptions,
	line string,
) ([]Fragment, []RawText) {
	// Strict pass (lowercase).
	fragments, rawTexts := d.scanSegment(
		text,
		0,
		d.langDict,
		d.langDictNormKeyMap,
		d.langMaxKeyLen,
		phono.NormalizeString,
		1.0,
		line,
		opts,
	)

	// If tolerant mode is disabled or everything was recognized, we are done.
	if !opts.DiacriticInsensitive || len(rawTexts) == 0 || len(d.langTolerantKeyMap) == 0 {
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
			d.langDict,
			d.langTolerantKeyMap,
			d.langMaxKeyLen,
			tolerantNormalize,
			0.9,
			line,
			opts,
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
// When opts.SingleGraphemeOnlyAsWord is true, single‑rune dictionary
// entries are only used when they form an isolated one‑letter word
// (i.e. they are not surrounded by other letters or digits).
func (d *Determinist) scanSegment(
	text string,
	offset int,
	dictionary phono.Dictionary,
	normalized phono.KeyMap,
	maxKeyLen int,
	normalizeCandidate func(string) string,
	passConfidence float64,
	line string,
	opts DeterministOptions,
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

			// Optional restriction: prevent single‑rune dictionary entries
			// from being used inside longer alphanumeric tokens.
			if opts.SingleGraphemeOnlyAsWord && l == 1 && !isIsolatedSingleRuneWord(runes, i) {
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

// isIsolatedSingleRuneWord reports whether the rune at position i in
// the slice represents a one‑letter "word" for the purposes of the
// SingleGraphemeOnlyAsWord option.
//
// A rune is considered an isolated word if:
//   - it is itself a letter or a digit, and
//   - the previous rune (if any) is not a letter or digit,
//   - the next rune (if any) is not a letter or digit.
func isIsolatedSingleRuneWord(runes []rune, i int) bool {
	if i < 0 || i >= len(runes) {
		return false
	}

	if !isAlnumRune(runes[i]) {
		return false
	}

	if i > 0 && isAlnumRune(runes[i-1]) {
		return false
	}
	if i+1 < len(runes) && isAlnumRune(runes[i+1]) {
		return false
	}

	return true
}

// isAlnumRune reports whether r is considered part of an alphanumeric
// token (letter or digit) for the purposes of
// SingleGraphemeOnlyAsWord.
func isAlnumRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
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
