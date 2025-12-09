package g2p

import (
	"sort"
	"strings"

	"github.com/temporal-IPA/tipa/pkg/phono"
)

type Fragment struct {
	Phonetized phono.Phonetized `json:"phonetized"` // The phonetized text may be IPA, SAMPA, ... or pseudo phonetics
	Pos        int              `json:"pos"`        // The first char position in the original text.
	Len        int              `json:"len"`        // The len of the expression in the original text
	Confidence float64          `json:"confidence"` // The confidence of the result.
	Variant    int              `json:"variant"`    // A variant number, when the dictionary offers multiple candidates.
}

type RawText struct {
	Text string `json:"text"` // The remaining raw text.
	Pos  int    `json:"pos"`  // Its position in the original text
	Len  int    `json:"len"`  // Its length
}

type Result struct {
	Text      string     `json:"text"` // The original text.
	Fragments []Fragment `json:"fragments"`
	RawTexts  []RawText  `json:"raw_texts"`
}

// Render merges the phonetized fragments and the raw text segments back into
// a single output string. The reconstruction follows the original positional
// indices (Pos) to ensure the correct ordering.
//
// Rules for reconstruction:
//   - Both fragments and raw texts reference absolute positions in the original string.
//   - We collect all segments into a common list annotated with their start Pos.
//   - Segments are sorted by Pos to restore the original sequence.
//   - Fragment output uses Fragment.Phonetized.String().
//   - RawText output uses RawText.Text.
//   - No modification or transformation is done on the text content itself.
func (r Result) Render() string {
	// A small struct to unify fragments and raw texts during reconstruction.
	type segment struct {
		pos  int
		text string
	}

	segments := make([]segment, 0, len(r.Fragments)+len(r.RawTexts))

	lastFrag := Fragment{
		Pos: -1,
	}
	// Convert all fragments to reconstruction segments.
	for _, f := range r.Fragments {
		if f.Pos != lastFrag.Pos {
			// Phonetized.String() returns the human-readable representation of the phonetic form.
			segments = append(segments, segment{
				pos:  f.Pos,
				text: f.Phonetized,
			})
			lastFrag.Pos = f.Pos
		}

	}

	// Convert all raw texts to reconstruction segments.
	for _, raw := range r.RawTexts {
		segments = append(segments, segment{
			pos:  raw.Pos,
			text: raw.Text,
		})
	}

	// Sort by position to ensure correct ordering.
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].pos < segments[j].pos
	})

	// Merge the ordered segments into the final output string.
	// The segments are assumed to cover the whole relevant reconstructed output.
	var out strings.Builder
	for _, seg := range segments {
		out.WriteString(seg.text)
	}
	return out.String()
}
