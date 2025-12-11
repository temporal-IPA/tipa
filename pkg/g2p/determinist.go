package g2p

import (
	"context"
	"sort"
	"unicode"

	"github.com/benoit-pereira-da-silva/textual/pkg/textual"
	"github.com/temporal-IPA/tipa/pkg/phono"
	"golang.org/x/text/unicode/norm"
)

// DeterministOptions control the behavior of a single Determinist run.
type DeterministOptions struct {
	// DiacriticInsensitive enables the diacritic‑insensitive matching pass.
	DiacriticInsensitive bool `json:"diacriticInsensitive"`

	// AllowPartialMatch controls whether dictionary entries are allowed
	// to match inside longer "expressions" (words or multi‑word sequences).
	AllowPartialMatch bool `json:"allowPartialMatch"`
}

// Determinist is a greedy, dictionary‑based grapheme‑to‑phoneme (g2p)
// processor working on textual.Result.
//
// It implements textual.Processor and can therefore be used directly in
// textual.Chain, Router, IOReaderProcessor, Transformation, etc.
type Determinist struct {
	// langDict is the main phonetic dictionary for the processor.
	langDict phono.Dictionary
	// langDictNormKeyMap maps normalized surface forms to the original
	// dictionary keys that share the same normalization (strict lowercase view).
	langDictNormKeyMap phono.KeyMap
	// langTolerantKeyMap is the diacritic‑insensitive view of langDict.
	langTolerantKeyMap phono.KeyMap
	// langMaxKeyLen caches the maximum key length (in runes).
	langMaxKeyLen int

	// options is the default configuration used by Apply. It can be
	// adjusted via SetOptions before wiring the Determinist into a
	// textual pipeline.
	options DeterministOptions

	// picker encapsulates the strategy used to select pronunciations.
	picker Picker

	// delimiters holds the configurable set of runes that are treated
	// as "expression delimiters" for the AllowPartialMatch option.
	delimiters map[rune]struct{}
}

// DefaultDeterministOptions are the defaults used by NewDeterminist.
var DefaultDeterministOptions = DeterministOptions{
	DiacriticInsensitive: false,
	AllowPartialMatch:    true,
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

// SetOptions changes the default options used by Apply.
//
// Callers are expected to configure options once, before wiring the
// Determinist into a textual pipeline. Concurrent mutation of options
// is not synchronized.
func (d *Determinist) SetOptions(opts DeterministOptions) {
	d.options = opts
}

// SetDelimiters configures the set of runes that are treated as
// expression delimiters when AllowPartialMatch is false.
func (d *Determinist) SetDelimiters(delims []rune) {
	if len(delims) == 0 {
		d.delimiters = nil
		return
	}
	m := make(map[rune]struct{}, len(delims))
	for _, r := range delims {
		m[r] = struct{}{}
	}
	d.delimiters = m
}

// Apply implements the textual.Processor interface.
//
// For each incoming Result, it:
//
//   - computes the current RawTexts from existing fragments;
//   - runs the Determinist on each RawText using the current options
//     and dictionary;
//   - appends newly discovered fragments while preserving existing ones.
//
// Raw texts are not recomputed here; callers can obtain them on demand
// via Result.RawTexts().
func (d *Determinist) Apply(ctx context.Context, in <-chan textual.Result) <-chan textual.Result {
	if ctx == nil {
		ctx = context.Background()
	}

	out := make(chan textual.Result)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				// Drain the input channel to avoid blocking upstream
				// senders, but stop forwarding results.
				for range in {
				}
				return
			case res, ok := <-in:
				if !ok {
					return
				}

				processed := d.applyWithOptions(res, d.options)

				select {
				case <-ctx.Done():
					// Context canceled while trying to send.
					return
				case out <- processed:
				}
			}
		}
	}()

	return out
}

// applyWithOptions is the internal implementation used by Apply.
//
// It processes all RawTexts of the input Result independently, using
// the same dictionary and options, then merges the newly discovered
// fragments and recomputes raw spans via Result.RawTexts().
func (d *Determinist) applyWithOptions(input textual.Result, opts DeterministOptions) textual.Result {
	if len(input.Text) == 0 {
		return input
	}

	// Compute current raw texts from existing fragments.
	rawTexts := input.RawTexts()
	if len(rawTexts) == 0 {
		return input
	}

	out := input

	// Start from existing fragments.
	out.Fragments = make([]textual.Fragment, len(input.Fragments))
	copy(out.Fragments, input.Fragments)

	newRawTexts := make([]textual.RawText, 0, len(rawTexts))

	for _, raw := range rawTexts {
		frag, leftover := d.scan(raw.Text, opts, string(input.Text), raw.Pos)

		// Rebase fragment positions to the original text coordinates.
		for _, f := range frag {
			out.Fragments = append(out.Fragments, f)
		}

		// Rebase leftover raw spans.
		if len(leftover) == 0 {
			continue
		}
		newRawTexts = append(newRawTexts, leftover...)
	}

	// Global ordering
	sortFragments(out.Fragments)
	return out
}

// scan applies the two‑pass (strict + tolerant) pipeline for a single
// dictionary on the given text string, using the Determinist's
// dictionary views and the provided options.
//
// Positions in the returned fragments and raw spans are rune offsets
// relative to the beginning of the original text (using offset).
func (d *Determinist) scan(
	text textual.UTF8String,
	opts DeterministOptions,
	line string,
	offset int,
) ([]textual.Fragment, []textual.RawText) {
	// Strict pass (lowercase).
	fragments, rawTexts := d.scanSegment(
		text,
		offset,
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
		return fragments, rawTexts
	}

	// Tolerant pass (diacritic‑insensitive) on the remaining RawText spans.
	tolerantFragments := make([]textual.Fragment, 0, len(fragments))
	tolerantFragments = append(tolerantFragments, fragments...)
	tolerantRawTexts := make([]textual.RawText, 0, len(rawTexts))

	for _, rt := range rawTexts {
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
	return tolerantFragments, tolerantRawTexts
}

// scanSegment performs a greedy longest‑match scan of a single text
// segment using the provided normalization function and dictionary view.
// The offset parameter indicates the rune position of the first rune of
// text within the original input string.
func (d *Determinist) scanSegment(
	text textual.UTF8String,
	offset int,
	dictionary phono.Dictionary,
	normalized phono.KeyMap,
	maxKeyLen int,
	normalizeCandidate func(string) string,
	passConfidence float64,
	line string,
	opts DeterministOptions,
) ([]textual.Fragment, []textual.RawText) {
	runes := []rune(text)
	n := len(runes)

	fragments := make([]textual.Fragment, 0)
	rawTexts := make([]textual.RawText, 0)

	if n == 0 {
		return fragments, rawTexts
	}

	// If the dictionary is empty or has no usable keys, the whole segment
	// is raw.
	if maxKeyLen <= 0 || len(normalized) == 0 || len(dictionary) == 0 {
		rawTexts = append(rawTexts, textual.RawText{
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
		// it as raw context.
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
			if unicode.IsSpace(runes[i+l-1]) {
				continue
			}

			if !opts.AllowPartialMatch && !d.candidateIsWholeExpression(runes, i, l) {
				continue
			}

			candidate := string(runes[i : i+l])
			normCandidate := normalizeCandidate(candidate)
			keys, ok := normalized[normCandidate]
			if !ok || len(keys) == 0 {
				continue
			}

			options := d.picker.PickAll(dictionary, keys, candidate, line)
			if len(options) == 0 {
				continue
			}

			// Flush any pending raw text before emitting fragments.
			if currentRawStart != -1 && currentRawStart < i {
				rawTexts = append(rawTexts, textual.RawText{
					Text: textual.UTF8String(runes[currentRawStart:i]),
					Pos:  offset + currentRawStart,
					Len:  i - currentRawStart,
				})
				currentRawStart = -1
			}

			for variantIndex, opt := range options {
				fragments = append(fragments, textual.Fragment{
					Transformed: opt.S,
					Pos:         offset + i,
					Len:         l,
					Confidence:  passConfidence * opt.C,
					Variant:     variantIndex,
				})
			}

			i += l
			found = true
			break
		}

		if !found {
			if currentRawStart == -1 {
				currentRawStart = i
			}
			i++
		}
	}

	// Flush trailing raw text, if any.
	if currentRawStart != -1 && currentRawStart < n {
		rawTexts = append(rawTexts, textual.RawText{
			Text: textual.UTF8String(runes[currentRawStart:n]),
			Pos:  offset + currentRawStart,
			Len:  n - currentRawStart,
		})
	}

	return fragments, rawTexts
}

// candidateIsWholeExpression reports whether the candidate substring
// runes[start : start+length] coincides with "expression" boundaries.
func (d *Determinist) candidateIsWholeExpression(runes []rune, start, length int) bool {
	if length <= 0 {
		return false
	}
	if start < 0 || start >= len(runes) {
		return false
	}

	end := start + length
	if end > len(runes) {
		return false
	}

	if start > 0 && !d.isDelimiterRune(runes[start-1]) {
		return false
	}
	if end < len(runes) && !d.isDelimiterRune(runes[end]) {
		return false
	}
	return true
}

// isDelimiterRune reports whether r is treated as an "expression
// delimiter" for the purposes of AllowPartialMatch.
func (d *Determinist) isDelimiterRune(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}

	if d == nil || d.delimiters == nil {
		return unicode.IsPunct(r)
	}

	_, ok := d.delimiters[r]
	return ok
}

// buildDiacriticInsensitiveKeyMap constructs a diacritic‑insensitive view of
// the dictionary keys.
func buildDiacriticInsensitiveKeyMap(keyMap phono.KeyMap) phono.KeyMap {
	tolerant := make(phono.KeyMap, len(keyMap))
	for normKey, keys := range keyMap {
		tKey := tolerantNormalize(normKey)
		tolerant[tKey] = append(tolerant[tKey], keys...)
	}
	return tolerant
}

// tolerantNormalize applies dictionary normalization then strips diacritics.
func tolerantNormalize(s string) string {
	return phono.NormalizeString(removeDiacritics(s))
}

// removeDiacritics strips non‑spacing marks (Unicode category Mn)
// after canonical decomposition.
func removeDiacritics(s string) string {
	decomposed := norm.NFD.String(s)
	out := make([]rune, 0, len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

// sortFragments orders fragments by position and span length, and, for
// identical spans, by decreasing confidence then increasing variant.
func sortFragments(frags []textual.Fragment) {
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
