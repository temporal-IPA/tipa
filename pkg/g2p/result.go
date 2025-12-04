package g2p

import "github.com/temporal-IPA/tipa/pkg/phono"

type Fragment struct {
	Phonetized phono.Phonetized `json:"phonetized"` // The phonetized text may be IPA, SAMPA, ... or pseudo phonetics
	Pos        int              `json:"pos"`        // The first char position in the original text.
	Len        int              `json:"len"`        // The len of the expression in the original text
	Confidence float64          `json:"confidence"` // The confidence of the result.
}

type RawText struct {
	Text string `json:"text"`
	Pos  int    `json:"pos"`
	Len  int    `json:"len"`
}

type Result struct {
	Fragments []Fragment `json:"fragments"`
	RawTexts  []RawText  `json:"raw_texts"`
}
