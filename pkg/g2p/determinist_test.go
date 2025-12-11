package g2p

import (
	"context"
	"strings"
	"testing"

	"github.com/benoit-pereira-da-silva/textual/pkg/textual"
	"github.com/temporal-IPA/tipa/pkg/phono"
)

// renderPhoneticOrRaw reconstructs a simple "output string" by walking
// the original text rune by rune and:
//
//   - if a fragment starts at this rune position, appending its
//     Phonetized value and skipping the covered span;
//   - otherwise, appending the original rune.
//
// This helper assumes that fragments are non‑overlapping and only used
// for simple tests; it is **not** a general renderer for all pipelines,
// but it is sufficient to validate AllowPartialMatch behaviour and
// basic scan properties.
func renderPhoneticOrRaw(res textual.Result) string {
	runes := []rune(res.Text)
	if len(runes) == 0 {
		return ""
	}

	type span struct {
		s textual.UTF8String
		l int
	}

	posToSpan := make(map[int]span, len(res.Fragments))
	for _, f := range res.Fragments {
		// For identical Pos we keep the first (highest‑confidence) variant.
		if _, exists := posToSpan[f.Pos]; exists {
			continue
		}
		posToSpan[f.Pos] = span{
			s: f.Transformed,
			l: f.Len,
		}
	}

	var b strings.Builder
	for pos := 0; pos < len(runes); pos++ {
		if sp, ok := posToSpan[pos]; ok {
			b.WriteString(string(sp.s))
			// Skip underlying runes covered by this fragment.
			if sp.l > 0 {
				pos += sp.l - 1
			}
			continue
		}
		b.WriteRune(runes[pos])
	}
	return b.String()
}

// runProcessorOnSingleInput runs a textual.Processor on a single input
// Result and returns the single output Result emitted by the processor.
// Tests rely on the 1:1 semantics of Determinist and simple chains.
func runProcessorOnSingleInput(t *testing.T, p textual.Processor, input textual.Result) textual.Result {
	t.Helper()

	ctx := context.Background()
	in := make(chan textual.Result, 1)
	in <- input
	close(in)

	outCh := p.Apply(ctx, in)

	var results []textual.Result
	for res := range outCh {
		results = append(results, res)
	}

	if len(results) == 0 {
		t.Fatalf("processor produced no Result")
	}
	if len(results) > 1 {
		t.Fatalf("processor produced %d Results, want 1", len(results))
	}
	return results[0]
}

// runDeterministOnText is a convenience helper specialised for
// Determinist that starts from a plain UTF‑8 string.
func runDeterministOnText(t *testing.T, d *Determinist, text textual.UTF8String) textual.Result {
	return runProcessorOnSingleInput(t, d, textual.Input(text))
}

// TestScanProgressiveThroughUnknownChunkWithPartialMatch verifies that
// the scanner continues to make progress past unknown substrings when
// partial matching is allowed. In particular it must still be able to
// find "Benoit" after the unknown "Gros" in the text "Le GrosBenoit".
func TestScanProgressiveThroughUnknownChunkWithPartialMatch(t *testing.T) {
	langDict := phono.Dictionary{
		"benoit": {"bənwa"},
		"le":     {"lə"},
	}

	d := NewDeterministWithOptions(langDict, DeterministOptions{
		DiacriticInsensitive: true,
		AllowPartialMatch:    true,
	})
	res := runDeterministOnText(t, d, "Le GrosBenoit")

	if got, want := len(res.Fragments), 2; got != want {
		t.Fatalf("expected %d fragments, got %d", want, got)
	}

	// Fragments should cover "Le" and "Benoit" in order.
	frag0 := res.Fragments[0]
	if frag0.Transformed != "lə" || frag0.Pos != 0 || frag0.Len != 2 {
		t.Errorf("unexpected first fragment: %+v", frag0)
	}

	frag1 := res.Fragments[1]
	if frag1.Transformed != "bənwa" || frag1.Pos != 7 || frag1.Len != 6 {
		t.Errorf("unexpected second fragment: %+v", frag1)
	}
	rawTexts := res.RawTexts()
	if got, want := len(rawTexts), 1; got != want {
		t.Fatalf("expected %d raw text span, got %d", want, got)
	}

	raw := rawTexts[0]
	// The raw block contains the space before "Gros" and the unknown word itself.
	if raw.Text != " Gros" || raw.Pos != 2 || raw.Len != 5 {
		t.Errorf("unexpected raw text: %+v", raw)
	}

	rendered := renderPhoneticOrRaw(res)
	if rendered != "lə Grosbənwa" {
		t.Fatalf("expected rendered text %q, got %q", "lə Grosbənwa", rendered)
	}
}

// TestScanProgressiveThroughUnknownChunkWithoutPartialMatch verifies
// that when AllowPartialMatch is disabled, internal matches inside
// tokens like "GrosBenoit" are not taken, while full‑token matches
// such as "Le" are still allowed.
func TestScanProgressiveThroughUnknownChunkWithoutPartialMatch(t *testing.T) {
	langDict := phono.Dictionary{
		"benoit": {"bənwa"},
		"le":     {"lə"},
	}

	d := NewDeterministWithOptions(langDict, DeterministOptions{
		DiacriticInsensitive: true,
		AllowPartialMatch:    false,
	})
	res := runDeterministOnText(t, d, "Le GrosBenoit")

	if got, want := len(res.Fragments), 1; got != want {
		t.Fatalf("expected %d fragment, got %d (fragments=%+v)", want, got, res.Fragments)
	}

	frag0 := res.Fragments[0]
	if frag0.Transformed != "lə" || frag0.Pos != 0 || frag0.Len != 2 {
		t.Errorf("unexpected fragment with AllowPartialMatch=false: %+v", frag0)
	}

	rawTexts := res.RawTexts()
	if got, want := len(rawTexts), 1; got != want {
		t.Fatalf("expected %d raw text span, got %d (raw=%+v)", want, got, rawTexts)
	}

	raw := rawTexts[0]
	// With partial matching disabled, "GrosBenoit" remains entirely raw.
	if raw.Text != " GrosBenoit" || raw.Pos != 2 || raw.Len != len([]rune(" GrosBenoit")) {
		t.Errorf("unexpected raw text with AllowPartialMatch=false: %+v", raw)
	}

	// No fragment should overlap the "Benoit" portion.
	for _, f := range res.Fragments {
		if f.Pos >= 7 && f.Pos < 13 {
			t.Fatalf("unexpected fragment inside 'GrosBenoit' when AllowPartialMatch=false: %+v", f)
		}
	}

	rendered := renderPhoneticOrRaw(res)
	if rendered != "lə GrosBenoit" {
		t.Fatalf("expected rendered text %q, got %q", "lə GrosBenoit", rendered)
	}
}

// TestScanTolerantDiacritics ensures that tolerant mode can match an
// input that lacks diacritics against a dictionary entry that has
// them (e.g. "garcon" -> "garçon").
func TestScanTolerantDiacritics(t *testing.T) {
	langDict := phono.Dictionary{
		"garçon": {"garsɔ̃"},
	}

	// Strict mode (default options) should not match "garcon" when only
	// "garçon" exists in the dictionary.
	dStrict := NewDeterminist(langDict)
	strict := runDeterministOnText(t, dStrict, "garcon")

	if len(strict.Fragments) != 0 {
		t.Fatalf("expected no fragments in strict mode, got %d", len(strict.Fragments))
	}
	rawTexts := strict.RawTexts()
	if got, want := len(rawTexts), 1; got != want {
		t.Fatalf("expected %d raw text span in strict mode, got %d", want, got)
	}

	// Explicitly enable diacritic‑insensitive matching.
	opts := DeterministOptions{
		DiacriticInsensitive: true,
		AllowPartialMatch:    true,
	}
	dTolerant := NewDeterministWithOptions(langDict, opts)

	// Tolerant mode should match and produce a single fragment.
	tolerant := runDeterministOnText(t, dTolerant, "garcon")
	rawTexts = tolerant.RawTexts()
	if got, want := len(tolerant.Fragments), 1; got != want {
		t.Fatalf("expected %d fragment in tolerant mode, got %d", want, got)
	}

	frag := tolerant.Fragments[0]
	if frag.Transformed != "garsɔ̃" || frag.Pos != 0 || frag.Len != 6 {
		t.Errorf("unexpected tolerant fragment: %+v", frag)
	}
	if len(rawTexts) != 0 {
		t.Errorf("expected no raw text in tolerant mode, got: %+v", rawTexts)
	}
}

// TestAllowPartialMatchControlsSingleGrapheme verifies that the
// AllowPartialMatch option generalises the previous
// SingleGraphemeOnlyAsWord behaviour:
//
//   - with AllowPartialMatch = true  : the inner "a" of "bar" can be
//     matched;
//   - with AllowPartialMatch = false : only the isolated "a" token is
//     matched.
func TestAllowPartialMatchControlsSingleGrapheme(t *testing.T) {
	langDict := phono.Dictionary{
		"a": {"A"},
	}

	// Baseline behaviour: with partial matching allowed, the inner "a"
	// of "bar" can be matched using the single‑rune entry "a".
	baseOpts := DeterministOptions{
		DiacriticInsensitive: false,
		AllowPartialMatch:    true,
	}
	dBase := NewDeterministWithOptions(langDict, baseOpts)
	base := runDeterministOnText(t, dBase, "bar a")

	foundInsideBar := false
	for _, f := range base.Fragments {
		if f.Pos >= 0 && f.Pos < 3 { // span of "bar"
			foundInsideBar = true
			break
		}
	}
	if !foundInsideBar {
		t.Fatalf("baseline: expected a fragment inside 'bar', got %#v", base.Fragments)
	}

	// With partial matching disabled, the "a" inside "bar" must no
	// longer be segmented out; only the isolated "a" token is allowed.
	opts := DeterministOptions{
		DiacriticInsensitive: false,
		AllowPartialMatch:    false,
	}

	dStrict := NewDeterministWithOptions(langDict, opts)
	res := runDeterministOnText(t, dStrict, "bar a")

	if got, want := len(res.Fragments), 1; got != want {
		t.Fatalf("with AllowPartialMatch=false: expected %d fragment, got %d (fragments=%#v)", want, got, res.Fragments)
	}

	rawTexts := res.RawTexts()
	frag := res.Fragments[0]
	if frag.Transformed != "A" || frag.Pos != 4 || frag.Len != 1 {
		t.Errorf("unexpected fragment with AllowPartialMatch=false: %+v", frag)
	}

	// "bar " should now remain entirely raw at the beginning.
	if len(rawTexts) == 0 || rawTexts[0].Text != "bar " {
		t.Errorf("expected leading raw text 'bar ', got: %+v", rawTexts)
	}
}

// TestAllowPartialMatchIsolatedWord ensures that an isolated
// one‑letter word is still recognized when AllowPartialMatch is
// disabled.
func TestAllowPartialMatchIsolatedWord(t *testing.T) {
	langDict := phono.Dictionary{
		"a": {"A"},
	}

	d := NewDeterministWithOptions(langDict, DeterministOptions{
		DiacriticInsensitive: false,
		AllowPartialMatch:    false,
	})

	res := runDeterministOnText(t, d, "a")
	rawTexts := res.RawTexts()
	if got, want := len(res.Fragments), 1; got != want {
		t.Fatalf("expected %d fragment, got %d", want, got)
	}
	if len(rawTexts) != 0 {
		t.Fatalf("expected no raw text, got %+v", rawTexts)
	}

	frag := res.Fragments[0]
	if frag.Transformed != "A" || frag.Pos != 0 || frag.Len != 1 {
		t.Errorf("unexpected fragment: %+v", frag)
	}
}

// TestAllowPartialMatchFullToken validates the full "abcdE" scenario:
//
//	dict: a→1, b→2, c→3, d→4
//
//	- with AllowPartialMatch = false : result text is "abcdE"
//	  (no decomposition inside the longer token);
//	- with AllowPartialMatch = true  : result text is "1234E"
//	  (full decomposition into single graphemes + trailing raw 'E').
func TestAllowPartialMatchFullToken(t *testing.T) {
	langDict := phono.Dictionary{
		"a": {"1"},
		"b": {"2"},
		"c": {"3"},
		"d": {"4"},
	}

	text := "abcdE"

	// Case 1: AllowPartialMatch = false
	optsStrict := DeterministOptions{
		DiacriticInsensitive: false,
		AllowPartialMatch:    false,
	}
	dStrict := NewDeterministWithOptions(langDict, optsStrict)
	resStrict := runDeterministOnText(t, dStrict, textual.UTF8String(text))

	rawTexts := resStrict.RawTexts()
	if got, want := len(resStrict.Fragments), 0; got != want {
		t.Fatalf("AllowPartialMatch=false: expected %d fragments, got %d (%+v)", want, got, resStrict.Fragments)
	}
	if got, want := len(rawTexts), 1; got != want {
		t.Fatalf("AllowPartialMatch=false: expected %d raw span, got %d (%+v)", want, got, rawTexts)
	}
	raw := rawTexts[0]
	if raw.Text != "abcdE" || raw.Pos != 0 || raw.Len != len([]rune(text)) {
		t.Errorf("AllowPartialMatch=false: unexpected raw span: %+v", raw)
	}

	renderedStrict := renderPhoneticOrRaw(resStrict)
	if renderedStrict != "abcdE" {
		t.Fatalf("AllowPartialMatch=false: expected rendered text %q, got %q", "abcdE", renderedStrict)
	}

	// Case 2: AllowPartialMatch = true
	optsDecompose := DeterministOptions{
		DiacriticInsensitive: false,
		AllowPartialMatch:    true,
	}
	dDecompose := NewDeterministWithOptions(langDict, optsDecompose)
	resDecompose := runDeterministOnText(t, dDecompose, textual.UTF8String(text))

	rawTexts = resDecompose.RawTexts()
	if got, want := len(resDecompose.Fragments), 4; got != want {
		t.Fatalf("AllowPartialMatch=true: expected %d fragments, got %d (%+v)", want, got, resDecompose.Fragments)
	}

	// Fragments should correspond to 1,2,3,4 on a,b,c,d (positions 0..3).
	wantIPA := []string{"1", "2", "3", "4"}
	for i, w := range wantIPA {
		f := resDecompose.Fragments[i]
		if string(f.Transformed) != w || f.Pos != i || f.Len != 1 {
			t.Errorf("AllowPartialMatch=true: unexpected fragment[%d]: %+v (want Phonetized=%q, Pos=%d, Len=1)", i, f, w, i)
		}
	}
	rawTexts = resDecompose.RawTexts()
	if got, want := len(rawTexts), 1; got != want {
		t.Fatalf("AllowPartialMatch=true: expected %d raw span, got %d (%+v)", want, got, rawTexts)
	}
	raw = rawTexts[0]
	if raw.Text != "E" || raw.Pos != 4 || raw.Len != 1 {
		t.Errorf("AllowPartialMatch=true: unexpected raw span: %+v (want Text=%q, Pos=%d, Len=%d)", raw, "E", 4, 1)
	}

	renderedDecompose := renderPhoneticOrRaw(resDecompose)
	if renderedDecompose != "1234E" {
		t.Fatalf("AllowPartialMatch=true: expected rendered text %q, got %q", "1234E", renderedDecompose)
	}
}

// TestDeterministDoesNotDecomposeUnknownSingleWord ensures that when the
// dictionary only contains internal substrings of an orthographic word,
// the scanner can be configured (AllowPartialMatch=false) to leave that
// full token as raw text instead of composing it from sub‑entries
// (e.g. "Font" + "ena" inside "Fontenay").
func TestDeterministDoesNotDecomposeUnknownSingleWord(t *testing.T) {
	langDict := phono.Dictionary{
		"Font": {"F"},
		"ena":  {"E"},
	}

	d := NewDeterministWithOptions(langDict, DeterministOptions{
		DiacriticInsensitive: false,
		AllowPartialMatch:    false,
	})
	text := "Fontenay"

	res := runDeterministOnText(t, d, textual.UTF8String(text))

	rawTexts := res.RawTexts()
	// Desired behaviour: no internal breakdown of "Fontenay" into "Font" + "ena".
	if got, want := len(res.Fragments), 0; got != want {
		t.Fatalf("expected %d fragments for %q, got %d (%+v)", want, text, got, res.Fragments)
	}

	if got, want := len(rawTexts), 1; got != want {
		t.Fatalf("expected %d raw span for %q, got %d (%+v)", want, text, got, rawTexts)
	}

	raw := rawTexts[0]
	if string(raw.Text) != text || raw.Pos != 0 || raw.Len != len([]rune(text)) {
		t.Errorf("unexpected raw span for %q: %+v (want Text=%q, Pos=0, Len=%d)", text, raw, text, len([]rune(text)))
	}
}

// TestDeterministCanDecomposeUnknownSingleWordWhenAllowed ensures that
// the same "Fontenay" example can be decomposed when partial matching
// is explicitly enabled.
func TestDeterministCanDecomposeUnknownSingleWordWhenAllowed(t *testing.T) {
	langDict := phono.Dictionary{
		"Font": {"F"},
		"ena":  {"E"},
	}

	d := NewDeterministWithOptions(langDict, DeterministOptions{
		DiacriticInsensitive: false,
		AllowPartialMatch:    true,
	})
	text := "Fontenay"

	res := runDeterministOnText(t, d, textual.UTF8String(text))
	rawTexts := res.RawTexts()
	if got, want := len(res.Fragments), 2; got != want {
		t.Fatalf("expected %d fragments for %q, got %d (%+v)", want, text, got, res.Fragments)
	}

	frag0 := res.Fragments[0]
	if frag0.Transformed != "F" || frag0.Pos != 0 || frag0.Len != 4 {
		t.Errorf("unexpected first fragment for %q: %+v (want Phonetized=F, Pos=0, Len=4)", text, frag0)
	}

	frag1 := res.Fragments[1]
	if frag1.Transformed != "E" || frag1.Pos != 4 || frag1.Len != 3 {
		t.Errorf("unexpected second fragment for %q: %+v (want Phonetized=E, Pos=4, Len=3)", text, frag1)
	}

	if got, want := len(rawTexts), 1; got != want {
		t.Fatalf("expected %d raw span for %q, got %d (%+v)", want, text, got, rawTexts)
	}
	raw := rawTexts[0]
	if raw.Text != "y" || raw.Pos != 7 || raw.Len != 1 {
		t.Errorf("unexpected raw span for %q: %+v (want Text=%q, Pos=7, Len=1)", text, raw, "y")
	}
}

// TestDeterministStillSupportsMultilingualSequences verifies that even
// when single words like "Fontenay" must not be decomposed into internal
// sub‑keys in strict mode, the scanner can still segment scripts where
// sequences without spaces are the natural tokens (e.g. Japanese or
// Chinese) when partial matching is allowed.
func TestDeterministStillSupportsMultilingualSequences(t *testing.T) {
	langDict := phono.Dictionary{
		"東京": {"T1"},
		"大学": {"T2"},
	}

	d := NewDeterminist(langDict)
	text := "東京大学"

	// Use the default options (AllowPartialMatch=true).
	res := runDeterministOnText(t, d, textual.UTF8String(text))
	rawTexts := res.RawTexts()
	if got, want := len(res.Fragments), 2; got != want {
		t.Fatalf("expected %d fragments for %q, got %d (%+v)", want, text, got, res.Fragments)
	}

	frag0 := res.Fragments[0]
	if frag0.Transformed != "T1" || frag0.Pos != 0 || frag0.Len != 2 {
		t.Errorf("unexpected first fragment for %q: %+v (want Phonetized=T1, Pos=0, Len=2)", text, frag0)
	}

	frag1 := res.Fragments[1]
	if frag1.Transformed != "T2" || frag1.Pos != 2 || frag1.Len != 2 {
		t.Errorf("unexpected second fragment for %q: %+v (want Phonetized=T2, Pos=2, Len=2)", text, frag1)
	}

	if len(rawTexts) != 0 {
		t.Errorf("expected no raw text for %q, got %+v", text, rawTexts)
	}
}

// TestDeterministCustomDelimiters verifies that the SetDelimiters API
// correctly influences what is considered an "expression boundary" when
// AllowPartialMatch=false.
func TestDeterministCustomDelimiters(t *testing.T) {
	langDict := phono.Dictionary{
		"foo": {"F"},
		"bar": {"B"},
	}

	// Use the same options for all passes; only delimiters change.
	d := NewDeterministWithOptions(langDict, DeterministOptions{
		DiacriticInsensitive: false,
		AllowPartialMatch:    false,
	})
	text := "foo,bar"

	// Default delimiters: comma acts as a delimiter (punctuation), so
	// both "foo" and "bar" can be matched as separate expressions when
	// AllowPartialMatch=false.
	resDefault := runDeterministOnText(t, d, textual.UTF8String(text))

	if got, want := len(resDefault.Fragments), 2; got != want {
		t.Fatalf("default delimiters: expected %d fragments, got %d (%+v)", want, got, resDefault.Fragments)
	}

	frag0 := resDefault.Fragments[0]
	if frag0.Transformed != "F" || frag0.Pos != 0 || frag0.Len != 3 {
		t.Errorf("default delimiters: unexpected first fragment: %+v", frag0)
	}
	frag1 := resDefault.Fragments[1]
	if frag1.Transformed != "B" || frag1.Pos != 4 || frag1.Len != 3 {
		t.Errorf("default delimiters: unexpected second fragment: %+v", frag1)
	}

	// Custom delimiters: only space is a delimiter, comma is no longer
	// a boundary. "foo,bar" becomes a single expression; with
	// AllowPartialMatch=false there should be no match.
	d.SetDelimiters([]rune{' '})

	resCustom := runDeterministOnText(t, d, textual.UTF8String(text))
	rawTexts := resCustom.RawTexts()

	if got, want := len(resCustom.Fragments), 0; got != want {
		t.Fatalf("custom delimiters: expected %d fragments, got %d (%+v)", want, got, resCustom.Fragments)
	}
	if got, want := len(rawTexts), 1; got != want {
		t.Fatalf("custom delimiters: expected %d raw span, got %d (%+v)", want, got, rawTexts)
	}
	raw := rawTexts[0]
	if string(raw.Text) != text || raw.Pos != 0 || raw.Len != len([]rune(text)) {
		t.Errorf("custom delimiters: unexpected raw span: %+v", raw)
	}
}

// TestDeterministApplyChain verifies that Determinist implements
// Processor semantics correctly and can be chained across multiple
// dictionaries.
func TestDeterministApplyChain(t *testing.T) {
	// Three simple dictionaries for successive passes.
	dict1 := phono.Dictionary{
		"foo": {"fu"},
	}
	dict2 := phono.Dictionary{
		"bar": {"ba"},
	}
	dict3 := phono.Dictionary{
		"baz": {"bz"},
	}

	d1 := NewDeterminist(dict1) // strict
	d2 := NewDeterminist(dict2) // strict
	d3 := NewDeterminist(dict3) // strict

	text := "foo bar baz"

	// Wire the three Determinist processors into a Chain to exercise the
	// textual.Processor implementation.
	chain := textual.NewChain(d1, d2, d3)

	base := textual.Input(textual.UTF8String(text))
	res3 := runProcessorOnSingleInput(t, chain, base)

	if got, want := len(res3.Fragments), 3; got != want {
		t.Fatalf("after chain: expected %d fragments, got %d (%+v)", want, got, res3.Fragments)
	}

	wantIPA := []string{"fu", "ba", "bz"}
	wantPos := []int{0, 4, 8}
	wantLen := []int{3, 3, 3}

	for i, f := range res3.Fragments {
		if string(f.Transformed) != wantIPA[i] || f.Pos != wantPos[i] || f.Len != wantLen[i] {
			t.Errorf("unexpected fragment[%d]: %+v (want IPA=%q, Pos=%d, Len=%d)", i, f, wantIPA[i], wantPos[i], wantLen[i])
		}
	}
}

// TestDeterministStreamApply ensures that the textual.Processor
// implementation emits the same result as the internal single‑Result
// implementation used by applyWithOptions.
func TestDeterministStreamApply(t *testing.T) {
	dict := phono.Dictionary{
		"foo": {"fu"},
	}
	d := NewDeterminist(dict)

	text := "foo"
	base := textual.Result{
		Text: textual.UTF8String(text),
	}

	want := d.applyWithOptions(base, d.Options())

	ctx := context.Background()
	in := make(chan textual.Result, 1)
	in <- base
	close(in)

	ch := d.Apply(ctx, in)

	got, ok := <-ch
	if !ok {
		t.Fatalf("expected a result from Determinist.Apply, got closed channel")
	}

	if len(got.Fragments) != len(want.Fragments) || len(got.RawTexts()) != len(want.RawTexts()) {
		t.Fatalf("Apply result differs from applyWithOptions: got=%+v want=%+v", got, want)
	}

	// Channel must be closed after the single Result.
	if _, ok := <-ch; ok {
		t.Fatalf("expected output channel to be closed after single result")
	}
}
