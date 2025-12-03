package g2p

import (
	"testing"

	"github.com/temporal-IPA/tipa/pkg/phono"
)

// TestScanProgressiveThroughUnknownChunk verifies that the scanner
// continues to make progress past unknown substrings. In particular it
// must still be able to find "Benoit" after the unknown "Gros" in the
// text "Le GrosBenoit".
// @bpds note that the frag0.Len == 3 because the original scan is "le " with a space.
func TestScanProgressiveThroughUnknownChunk(t *testing.T) {
	langDict := phono.Dictionary{
		"benoit": {"bənwa"},
		"le":     {"lə"},
	}

	d := NewDeterminist(langDict, nil)
	res := d.Scan("Le GrosBenoit", false)

	if got, want := len(res.Fragments), 2; got != want {
		t.Fatalf("expected %d fragments, got %d", want, got)
	}

	// Fragments should cover "Le" and "Benoit" in order.
	frag0 := res.Fragments[0]
	if frag0.IPA != "lə" || frag0.Pos != 0 || frag0.Len != 3 {
		t.Errorf("unexpected first fragment: %+v", frag0)
	}

	frag1 := res.Fragments[1]
	if frag1.IPA != "bənwa" || frag1.Pos != 7 || frag1.Len != 6 {
		t.Errorf("unexpected second fragment: %+v", frag1)
	}

	if got, want := len(res.RawTexts), 1; got != want {
		t.Fatalf("expected %d raw text span, got %d", want, got)
	}

	raw := res.RawTexts[0]
	// The raw block contains the space before "Gros" and the unknown word itself.
	if raw.Text != "Gros" || raw.Pos != 3 || raw.Len != 4 {
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

	d := NewDeterminist(langDict, nil)

	// Strict mode should not match "garcon" when only "garçon" exists.
	strict := d.Scan("garcon", false)
	if len(strict.Fragments) != 0 {
		t.Fatalf("expected no fragments in strict mode, got %d", len(strict.Fragments))
	}
	if got, want := len(strict.RawTexts), 1; got != want {
		t.Fatalf("expected %d raw text span in strict mode, got %d", want, got)
	}

	// Tolerant mode should match and produce a single fragment.
	tolerant := d.Scan("garcon", true)
	if got, want := len(tolerant.Fragments), 1; got != want {
		t.Fatalf("expected %d fragment in tolerant mode, got %d", want, got)
	}

	frag := tolerant.Fragments[0]
	if frag.IPA != "garsɔ̃" || frag.Pos != 0 || frag.Len != 6 {
		t.Errorf("unexpected tolerant fragment: %+v", frag)
	}
	if len(tolerant.RawTexts) != 0 {
		t.Errorf("expected no raw text in tolerant mode, got: %+v", tolerant.RawTexts)
	}
}

// TestScanUsesFinalDictionary verifies the two‑stage scanning:
// unmatched spans from the main dictionary are rescanned using the
// final dictionary, and all resulting fragments are returned in
// positional order.
func TestScanUsesFinalDictionary(t *testing.T) {
	langDict := phono.Dictionary{
		"foo": {"fu"},
		"baz": {"bz"},
	}
	finalDict := phono.Dictionary{
		"bar": {"ba"},
	}

	d := NewDeterminist(langDict, finalDict)
	res := d.Scan("foo bar baz", false)

	if got, want := len(res.Fragments), 3; got != want {
		t.Fatalf("expected %d fragments, got %d", want, got)
	}

	// Fragments must be ordered by position: "foo" (0), "bar" (4), "baz" (8).
	wantIPA := []string{"fu", "ba", "bz"}
	wantPos := []int{0, 4, 8}
	wantLen := []int{3, 3, 3}

	for i, f := range res.Fragments {
		if f.IPA != wantIPA[i] || f.Pos != wantPos[i] || f.Len != wantLen[i] {
			t.Errorf("unexpected fragment[%d]: %+v (want IPA=%q, Pos=%d, Len=%d)", i, f, wantIPA[i], wantPos[i], wantLen[i])
		}
	}

	// The spaces around "bar" should remain as raw text ranges.
	if got, want := len(res.RawTexts), 2; got != want {
		t.Fatalf("expected %d raw text spans, got %d: %+v", want, got, res.RawTexts)
	}

	// Positions should be the single spaces before and after "bar".
	if res.RawTexts[0].Text != " " || res.RawTexts[0].Pos != 3 || res.RawTexts[0].Len != 1 {
		t.Errorf("unexpected first raw span: %+v", res.RawTexts[0])
	}
	if res.RawTexts[1].Text != " " || res.RawTexts[1].Pos != 7 || res.RawTexts[1].Len != 1 {
		t.Errorf("unexpected second raw span: %+v", res.RawTexts[1])
	}
}
