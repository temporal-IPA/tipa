package g2p

import (
	"context"
	"strings"
	"testing"

	"github.com/temporal-IPA/tipa/pkg/phono"
)

// renderPhoneticOrRaw reconstructs a simple "output string" by walking
// the original text rune by rune and:
//
//   - if a fragment starts at this rune position, appending its
//     Phonetized value;
//   - otherwise, appending the original rune.
//
// This helper assumes that fragments are non‑overlapping and only used
// for simple tests; it is **not** a general renderer for all pipelines,
// but it is sufficient to validate SingleGraphemeOnlyAsWord behaviour.
func renderPhoneticOrRaw(res Result) string {
	runes := []rune(res.Text)
	if len(runes) == 0 {
		return ""
	}

	posToPron := make(map[int]string, len(res.Fragments))
	for _, f := range res.Fragments {
		// For identical Pos we keep the first (highest‑confidence) variant.
		if _, exists := posToPron[f.Pos]; exists {
			continue
		}
		posToPron[f.Pos] = f.Phonetized
	}

	var b strings.Builder
	for pos, r := range runes {
		if s, ok := posToPron[pos]; ok {
			b.WriteString(s)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TestScanProgressiveThroughUnknownChunk verifies that the scanner
// continues to make progress past unknown substrings. In particular it
// must still be able to find "Benoit" after the unknown "Gros" in the
// text "Le GrosBenoit".
func TestScanProgressiveThroughUnknownChunk(t *testing.T) {
	langDict := phono.Dictionary{
		"benoit": {"bənwa"},
		"le":     {"lə"},
	}

	d := NewDeterminist(langDict)
	res := d.Scan("Le GrosBenoit")

	if got, want := len(res.Fragments), 2; got != want {
		t.Fatalf("expected %d fragments, got %d", want, got)
	}

	// Fragments should cover "Le" and "Benoit" in order.
	frag0 := res.Fragments[0]
	if frag0.Phonetized != "lə" || frag0.Pos != 0 || frag0.Len != 2 {
		t.Errorf("unexpected first fragment: %+v", frag0)
	}

	frag1 := res.Fragments[1]
	if frag1.Phonetized != "bənwa" || frag1.Pos != 7 || frag1.Len != 6 {
		t.Errorf("unexpected second fragment: %+v", frag1)
	}

	if got, want := len(res.RawTexts), 1; got != want {
		t.Fatalf("expected %d raw text span, got %d", want, got)
	}

	raw := res.RawTexts[0]
	// The raw block contains the space before "Gros" and the unknown word itself.
	if raw.Text != " Gros" || raw.Pos != 2 || raw.Len != 5 {
		t.Errorf("unexpected raw text: %+v", raw)
	}
}

// TestScanTolerantDiacritics ensures that tolerant mode can match an
// input that lacks diacritics against a dictionary entry that has
// them (e.g. "garcon" -> "garçon").
func TestScanTolerantDiacritics(t *testing.T) {
	langDict := phono.Dictionary{
		"garçon": {"garsɔ̃"},
	}

	d := NewDeterminist(langDict)

	// Strict mode (default options) should not match "garcon" when only
	// "garçon" exists in the dictionary.
	strict := d.Scan("garcon")
	if len(strict.Fragments) != 0 {
		t.Fatalf("expected no fragments in strict mode, got %d", len(strict.Fragments))
	}
	if got, want := len(strict.RawTexts), 1; got != want {
		t.Fatalf("expected %d raw text span in strict mode, got %d", want, got)
	}

	// Explicitly enable diacritic‑insensitive matching.
	opts := DeterministOptions{
		DiacriticInsensitive:     true,
		SingleGraphemeOnlyAsWord: false,
	}

	// Tolerant mode should match and produce a single fragment.
	tolerant := d.ScanWithOptions("garcon", opts)
	if got, want := len(tolerant.Fragments), 1; got != want {
		t.Fatalf("expected %d fragment in tolerant mode, got %d", want, got)
	}

	frag := tolerant.Fragments[0]
	if frag.Phonetized != "garsɔ̃" || frag.Pos != 0 || frag.Len != 6 {
		t.Errorf("unexpected tolerant fragment: %+v", frag)
	}
	if len(tolerant.RawTexts) != 0 {
		t.Errorf("expected no raw text in tolerant mode, got: %+v", tolerant.RawTexts)
	}
}

// TestScanSingleGraphemeOnlyAsWord verifies that when the
// SingleGraphemeOnlyAsWord option is enabled, single‑rune dictionary
// entries are only used for isolated one‑letter words and not inside
// longer tokens.
func TestScanSingleGraphemeOnlyAsWord(t *testing.T) {
	langDict := phono.Dictionary{
		"a": {"A"},
	}

	d := NewDeterminist(langDict)

	// Baseline behaviour: without the option, the inner "a" of "bar"
	// can be matched using the single‑rune entry "a".
	baseOpts := DeterministOptions{
		DiacriticInsensitive:     false,
		SingleGraphemeOnlyAsWord: false,
	}
	base := d.ScanWithOptions("bar a", baseOpts)

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

	// With the option enabled, single‑rune entries are only allowed for
	// isolated one‑letter words ("a" here). The "a" inside "bar" must no
	// longer be segmented out.
	opts := DeterministOptions{
		DiacriticInsensitive:     false,
		SingleGraphemeOnlyAsWord: true,
	}

	res := d.ScanWithOptions("bar a", opts)

	if got, want := len(res.Fragments), 1; got != want {
		t.Fatalf("with option: expected %d fragment, got %d (fragments=%#v)", want, got, res.Fragments)
	}

	frag := res.Fragments[0]
	if frag.Phonetized != "A" || frag.Pos != 4 || frag.Len != 1 {
		t.Errorf("unexpected fragment with option: %+v", frag)
	}

	// "bar " should now remain entirely raw at the beginning.
	if len(res.RawTexts) == 0 || res.RawTexts[0].Text != "bar " {
		t.Errorf("expected leading raw text 'bar ', got: %+v", res.RawTexts)
	}
}

// TestSingleGraphemeOnlyAsWordIsolated ensures that an isolated
// one‑letter word is still recognized when SingleGraphemeOnlyAsWord
// is enabled.
func TestSingleGraphemeOnlyAsWordIsolated(t *testing.T) {
	langDict := phono.Dictionary{
		"a": {"A"},
	}

	d := NewDeterminist(langDict)

	opts := DeterministOptions{
		DiacriticInsensitive:     false,
		SingleGraphemeOnlyAsWord: true,
	}

	res := d.ScanWithOptions("a", opts)

	if got, want := len(res.Fragments), 1; got != want {
		t.Fatalf("expected %d fragment, got %d", want, got)
	}
	if len(res.RawTexts) != 0 {
		t.Fatalf("expected no raw text, got %+v", res.RawTexts)
	}

	frag := res.Fragments[0]
	if frag.Phonetized != "A" || frag.Pos != 0 || frag.Len != 1 {
		t.Errorf("unexpected fragment: %+v", frag)
	}
}

// TestSingleGraphemeOnlyAsWordDisabledFullToken validates the full
// "abcdE" scenario:
//
//	dict: a→1, b→2, c→3, d→4
//
//	- with SingleGraphemeOnlyAsWord = true  : result text is "abcdE"
//	  (no decomposition inside the longer token);
//	- with SingleGraphemeOnlyAsWord = false : result text is "1234E"
//	  (full decomposition into single graphemes + trailing raw 'E').
func TestSingleGraphemeOnlyAsWordDisabledFullToken(t *testing.T) {
	langDict := phono.Dictionary{
		"a": {"1"},
		"b": {"2"},
		"c": {"3"},
		"d": {"4"},
	}

	d := NewDeterminist(langDict)
	text := "abcdE"

	// Case 1: SingleGraphemeOnlyAsWord = true
	optsIsolated := DeterministOptions{
		DiacriticInsensitive:     false,
		SingleGraphemeOnlyAsWord: true,
	}
	resIsolated := d.ScanWithOptions(text, optsIsolated)

	if got, want := len(resIsolated.Fragments), 0; got != want {
		t.Fatalf("isolated=true: expected %d fragments, got %d (%+v)", want, got, resIsolated.Fragments)
	}
	if got, want := len(resIsolated.RawTexts), 1; got != want {
		t.Fatalf("isolated=true: expected %d raw span, got %d (%+v)", want, got, resIsolated.RawTexts)
	}
	raw := resIsolated.RawTexts[0]
	if raw.Text != "abcdE" || raw.Pos != 0 || raw.Len != len([]rune(text)) {
		t.Errorf("isolated=true: unexpected raw span: %+v", raw)
	}

	renderedIsolated := renderPhoneticOrRaw(resIsolated)
	if renderedIsolated != "abcdE" {
		t.Fatalf("isolated=true: expected rendered text %q, got %q", "abcdE", renderedIsolated)
	}

	// Case 2: SingleGraphemeOnlyAsWord = false
	optsDecompose := DeterministOptions{
		DiacriticInsensitive:     false,
		SingleGraphemeOnlyAsWord: false,
	}
	resDecompose := d.ScanWithOptions(text, optsDecompose)

	if got, want := len(resDecompose.Fragments), 4; got != want {
		t.Fatalf("isolated=false: expected %d fragments, got %d (%+v)", want, got, resDecompose.Fragments)
	}

	// Fragments should correspond to 1,2,3,4 on a,b,c,d (positions 0..3).
	wantIPA := []string{"1", "2", "3", "4"}
	for i, w := range wantIPA {
		f := resDecompose.Fragments[i]
		if f.Phonetized != w || f.Pos != i || f.Len != 1 {
			t.Errorf("isolated=false: unexpected fragment[%d]: %+v (want Phonetized=%q, Pos=%d, Len=1)", i, f, w, i)
		}
	}

	if got, want := len(resDecompose.RawTexts), 1; got != want {
		t.Fatalf("isolated=false: expected %d raw span, got %d (%+v)", want, got, resDecompose.RawTexts)
	}
	raw = resDecompose.RawTexts[0]
	if raw.Text != "E" || raw.Pos != 4 || raw.Len != 1 {
		t.Errorf("isolated=false: unexpected raw span: %+v (want Text=%q, Pos=%d, Len=%d)", raw, "E", 4, 1)
	}

	renderedDecompose := renderPhoneticOrRaw(resDecompose)
	if renderedDecompose != "1234E" {
		t.Fatalf("isolated=false: expected rendered text %q, got %q", "1234E", renderedDecompose)
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

	// First pass: only "foo" is recognized.
	res1 := d1.Scan(text)
	if got, want := len(res1.Fragments), 1; got != want {
		t.Fatalf("after pass1: expected %d fragment, got %d (%+v)", want, got, res1.Fragments)
	}

	// Second pass: process remaining raw text with dict2.
	res2 := d2.Apply(res1)

	// Third pass: process remaining raw text with dict3.
	res3 := d3.Apply(res2)

	if got, want := len(res3.Fragments), 3; got != want {
		t.Fatalf("after chain: expected %d fragments, got %d (%+v)", want, got, res3.Fragments)
	}

	wantIPA := []string{"fu", "ba", "bz"}
	wantPos := []int{0, 4, 8}
	wantLen := []int{3, 3, 3}

	for i, f := range res3.Fragments {
		if f.Phonetized != wantIPA[i] || f.Pos != wantPos[i] || f.Len != wantLen[i] {
			t.Errorf("unexpected fragment[%d]: %+v (want IPA=%q, Pos=%d, Len=%d)", i, f, wantIPA[i], wantPos[i], wantLen[i])
		}
	}
}

// TestDeterministStreamApply ensures that the CancellableProcessor
// implementation emits the same result as Apply.
func TestDeterministStreamApply(t *testing.T) {
	dict := phono.Dictionary{
		"foo": {"fu"},
	}
	d := NewDeterminist(dict)

	text := "foo"
	base := Result{
		Text: text,
		RawTexts: []RawText{{
			Text: text,
			Pos:  0,
			Len:  len([]rune(text)),
		}},
	}

	want := d.Apply(base)

	ctx := context.Background()
	ch := d.StreamApply(ctx, base)

	got, ok := <-ch
	if !ok {
		t.Fatalf("expected a result from StreamApply, got closed channel")
	}

	if len(got.Fragments) != len(want.Fragments) || len(got.RawTexts) != len(want.RawTexts) {
		t.Fatalf("StreamApply result differs from Apply: got=%+v want=%+v", got, want)
	}
}
