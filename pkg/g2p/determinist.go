package g2p

import "github.com/temporal-IPA/tipa/pkg/phono"

type Determinist struct {
	dictionary phono.Dictionary
	normalized map[string][]string
	maxKeyLen  int
	tolerant   bool
}

func NewDeterminist(dictionary phono.Dictionary, tolerant bool) *Determinist {
	normalized := dictionary.NormalizedKeys()
	maxKeyLen := 0
	for _, k := range normalized {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	}
	g := &Determinist{
		dictionary: dictionary,
		normalized: normalized,
		maxKeyLen:  maxKeyLen,
		tolerant:   tolerant,
	}
	return g
}

// Scan is a brute force scanner that tries to find the biggest available key for each part of the text.
func (g Determinist) Scan(text string) ([]Fragment, []RawText) {
	fragments := make([]Fragment, 0)
	rawTexts := make([]RawText, 0)
	// TODO implement here
	return fragments, rawTexts
}
