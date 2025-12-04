package g2p

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// FrenchLiaisonProcessor applies simple French liaison heuristics
// on top of a grapheme‑to‑phoneme Result. It never modifies the
// original text, only the Phonetized fields of the fragments.
type FrenchLiaisonProcessor struct {
	// allowLooseLiaison controls whether we are allowed to use the
	// loose "final consonant" heuristic for arbitrary words, beyond
	// the small hand‑coded lexical sets.
	//
	// When false (default via NewFrenchLiaisonProcessor), liaison is
	// only created after a few determiners, pronouns, and adjectives.
	// When true (via NewFrenchLiaisonProcessorWithFallback), liaison
	// may also be added based purely on word‑final S/X/Z/N/T/D/P for
	// non‑proper names.
	allowLooseLiaison bool

	determinersZ map[string]struct{}
	determinersN map[string]struct{}
	adjectivesT  map[string]struct{}
	pronounsZ    map[string]struct{}
	pronounsN    map[string]struct{}
	pronounsT    map[string]struct{}
	adverbsZ     map[string]struct{} // e.g. très
	adverbsP     map[string]struct{} // e.g. trop

	forbidBefore map[string]struct{} // words that block liaison after them (e.g. "et")
	hAspire      map[string]struct{} // words with h aspiré (no liaison before)
}

// NewFrenchLiaisonProcessor returns a processor configured with a
// conservative rule set: liaison is only applied after a small list of
// determiners, pronouns, and common adjectives.
func NewFrenchLiaisonProcessor() *FrenchLiaisonProcessor {
	return newFrenchLiaisonProcessor(false)
}

// NewFrenchLiaisonProcessorWithFallback returns a processor that also
// allows optional liaison based only on final consonants (s/x/z/n/t/d/p)
// for words that are not obvious proper names.
func NewFrenchLiaisonProcessorWithFallback() *FrenchLiaisonProcessor {
	return newFrenchLiaisonProcessor(true)
}

// newFrenchLiaisonProcessor initializes the lexical sets and global
// control flags for liaison processing.
func newFrenchLiaisonProcessor(allowLoose bool) *FrenchLiaisonProcessor {
	p := &FrenchLiaisonProcessor{
		allowLooseLiaison: allowLoose,
	}

	// Minimal liaison givers – not exhaustive, but useful in practice.
	p.determinersZ = makeNormalizedSet([]string{
		"les", "des", "mes", "tes", "ses",
		"nos", "vos", "leurs",
		"aux", "ces", "quelques", "toutes", "tous",
	})
	p.determinersN = makeNormalizedSet([]string{
		"un", "une", "aucun", "plein",
		"mon", "ton", "son",
	})
	p.adjectivesT = makeNormalizedSet([]string{
		"grand", "petit", "tout",
	})
	p.pronounsZ = makeNormalizedSet([]string{
		"nous", "vous", "elles", "ils",
	})
	p.pronounsN = makeNormalizedSet([]string{
		"en", "on",
	})
	// Todo c'est n'importe quoi chat gpt pro !!!!!
	p.pronounsT = makeNormalizedSet([]string{
		"est", "sont", "ait", "était", "étaient",
	})
	// Optional extras: très + ADJ (z), trop + ADJ (p).
	p.adverbsZ = makeNormalizedSet([]string{
		"très", "tres",
	})
	p.adverbsP = makeNormalizedSet([]string{
		"trop",
	})
	// Words that never take liaison after them.
	p.forbidBefore = makeNormalizedSet([]string{
		"et",
	})
	// Small list of h aspiré words – enough to avoid the worst errors.
	p.hAspire = makeNormalizedSet([]string{
		"haricot", "honte", "héros", "heros", "huitre",
	})

	return p
}

// Apply takes a g2p Result and returns a new Result in which liaison
// consonants have been inserted into the Phonetized fields of fragments
// when appropriate.
//
// The original text and fragment positions are preserved. Only the
// Phonetized strings are modified.
func (p *FrenchLiaisonProcessor) Apply(res Result) Result {
	if len(res.Text) == 0 || len(res.Fragments) == 0 {
		return res
	}

	// Copy fragments so that we do not mutate the original Result.
	out := res
	out.Fragments = make([]Fragment, len(res.Fragments))
	copy(out.Fragments, res.Fragments)

	runes := []rune(res.Text)
	tokens := tokenizeFrenchWords(res.Text)
	if len(tokens) < 2 {
		return out
	}

	// Attach fragments to tokens when a fragment span exactly covers the
	// token span in rune positions. This keeps the logic robust even if
	// some dictionary entries span multiple orthographic tokens.
	attachFragmentsToTokens(tokens, out.Fragments)

	// Walk through successive (word) tokens and decide whether to insert
	// a liaison consonant between them.
	for i := 0; i < len(tokens)-1; i++ {
		left := &tokens[i]
		right := &tokens[i+1]

		// We only handle boundaries where both sides have a pronunciation.
		if left.fragIndex < 0 || right.fragIndex < 0 {
			continue
		}

		// Do not cross strong punctuation or line breaks.
		if hasStrongBoundary(runes, left, right) {
			continue
		}

		// Never create liaison after words like "et".
		if _, forbidden := p.forbidBefore[left.norm]; forbidden {
			continue
		}

		// The right word must start with a vowel or h muet.
		if !p.startsWithVowelOrHMuet(right.text) {
			continue
		}

		// Decide which liaison consonant to use, if any.
		liaisonPhone := p.liaisonPhoneFor(left)
		if liaisonPhone == "" {
			continue
		}

		// Insert the liaison consonant at the end of the left fragment.
		frag := &out.Fragments[left.fragIndex]
		frag.Phonetized = appendLiaisonPhone(frag.Phonetized, liaisonPhone)
	}

	return out
}

// ApplyFrenchLiaison is a convenience helper that uses the conservative
// processor configuration (no loose fallback).
func ApplyFrenchLiaison(res Result) Result {
	return NewFrenchLiaisonProcessor().Apply(res)
}

// ApplyFrenchLiaisonWithFallback is a helper that enables the looser
// "final consonant" heuristic for liaison givers.
func ApplyFrenchLiaisonWithFallback(res Result) Result {
	return NewFrenchLiaisonProcessorWithFallback().Apply(res)
}

// orthToken represents a single orthographic word (sequence of letters
// and possibly apostrophes) in the original text, along with its rune
// offsets and the index of the fragment that covers it, if any.
type orthToken struct {
	text      string
	norm      string
	runeStart int
	runeLen   int
	fragIndex int
}

// tokenizeFrenchWords extracts a flat list of word‑like tokens from
// the text. Tokens are sequences of letters and apostrophes; everything
// else (spaces, punctuation, hyphens, etc.) acts as a separator.
//
// Token positions are recorded in rune indices so they can be aligned
// with Fragment.Pos / Fragment.Len from the g2p pipeline.
func tokenizeFrenchWords(text string) []orthToken {
	runes := []rune(text)
	n := len(runes)
	tokens := make([]orthToken, 0, n/2)

	inWord := false
	wordStart := 0

	for i, r := range runes {
		if isWordRune(r) {
			if !inWord {
				inWord = true
				wordStart = i
			}
			continue
		}
		if inWord {
			tokens = append(tokens, newOrthToken(runes, wordStart, i))
			inWord = false
		}
	}

	if inWord {
		tokens = append(tokens, newOrthToken(runes, wordStart, n))
	}

	return tokens
}

// newOrthToken builds an orthToken from a rune slice [start:end].
func newOrthToken(runes []rune, start, end int) orthToken {
	txt := string(runes[start:end])
	return orthToken{
		text:      txt,
		norm:      tolerantNormalize(txt),
		runeStart: start,
		runeLen:   end - start,
		fragIndex: -1,
	}
}

// isWordRune reports whether a rune belongs to a "word" token for the
// purposes of liaison. Letters and apostrophes are treated as word
// characters; hyphens and other punctuation are separators.
func isWordRune(r rune) bool {
	if unicode.IsLetter(r) {
		return true
	}
	switch r {
	case '\'', '’':
		return true
	default:
		return false
	}
}

// attachFragmentsToTokens links orthTokens to fragments when a fragment
// span exactly matches the token span in rune offsets. Tokens that are
// covered by multi‑word dictionary entries will not get a fragment.
func attachFragmentsToTokens(tokens []orthToken, fragments []Fragment) {
	if len(tokens) == 0 || len(fragments) == 0 {
		return
	}

	iFrag := 0
	for i := range tokens {
		tok := &tokens[i]

		// Skip fragments that end before the token starts.
		for iFrag < len(fragments) && fragments[iFrag].Pos+fragments[iFrag].Len <= tok.runeStart {
			iFrag++
		}
		if iFrag >= len(fragments) {
			return
		}

		frag := &fragments[iFrag]
		if frag.Pos == tok.runeStart && frag.Len == tok.runeLen {
			tok.fragIndex = iFrag
		}
	}
}

// hasStrongBoundary reports whether the text between left and right
// tokens contains strong punctuation (. ? ! ; :) or line breaks. If so,
// we do not create liaison across that boundary.
func hasStrongBoundary(runes []rune, left, right *orthToken) bool {
	start := left.runeStart + left.runeLen
	end := right.runeStart
	if start >= end {
		return false
	}

	for _, r := range runes[start:end] {
		switch r {
		case '.', '?', '!', ';', ':':
			return true
		case '\n', '\r':
			return true
		}
	}

	return false
}

// startsWithVowelOrHMuet checks whether a word starts with a vowel
// (orthographically a/e/i/o/u/y/œ with or without accents) or with a
// non‑aspirated h (h muet).
func (p *FrenchLiaisonProcessor) startsWithVowelOrHMuet(word string) bool {
	norm := tolerantNormalize(word)
	if norm == "" {
		return false
	}

	for _, r := range norm {
		if !unicode.IsLetter(r) {
			continue
		}
		if r == 'h' {
			if _, aspirated := p.hAspire[norm]; aspirated {
				return false
			}
			return true
		}

		switch r {
		case 'a', 'e', 'i', 'o', 'u', 'y', 'œ':
			return true
		default:
			return false
		}
	}

	return false
}

// liaisonPhoneFor decides which liaison consonant to insert after a
// given left‑hand token, if any. It applies the lexical rule sets
// first, and optionally falls back to the final‑consonant heuristic.
func (p *FrenchLiaisonProcessor) liaisonPhoneFor(tok *orthToken) string {
	if _, forbidden := p.forbidBefore[tok.norm]; forbidden {
		return ""
	}

	if p.isLiaisonGiver(tok.norm) {
		return p.guessLiaisonPhone(tok.text)
	}

	if p.allowLooseLiaison && !isProbablyProperName(tok.text) {
		return p.guessLiaisonPhone(tok.text)
	}

	return ""
}

// isLiaisonGiver reports whether a normalized word belongs to one of
// the small hand‑coded sets that systematically trigger liaison in
// common contexts (determiners, pronouns, some adjectives, très/trop).
func (p *FrenchLiaisonProcessor) isLiaisonGiver(norm string) bool {
	if _, ok := p.determinersZ[norm]; ok {
		return true
	}
	if _, ok := p.determinersN[norm]; ok {
		return true
	}
	if _, ok := p.adjectivesT[norm]; ok {
		return true
	}
	if _, ok := p.pronounsZ[norm]; ok {
		return true
	}
	if _, ok := p.pronounsN[norm]; ok {
		return true
	}
	if _, ok := p.pronounsT[norm]; ok {
		return true
	}
	if _, ok := p.adverbsZ[norm]; ok {
		return true
	}
	if _, ok := p.adverbsP[norm]; ok {
		return true
	}
	return false
}

// guessLiaisonPhone chooses the liaison consonant for a given word.
// It first checks the small lexical classes (Z/N/T/P), then optionally
// falls back to a simple orthographic heuristic based on the final
// consonant.
//
// The returned value is a phonetic symbol string (e.g. "z", "n", "t"),
// which can be IPA, SAMPA, or any compatible unit.
func (p *FrenchLiaisonProcessor) guessLiaisonPhone(word string) string {
	lower := tolerantNormalize(word)

	// Lexical classes first.
	if _, ok := p.determinersZ[lower]; ok {
		return "z"
	}
	if _, ok := p.pronounsZ[lower]; ok {
		return "z"
	}
	if _, ok := p.adverbsZ[lower]; ok {
		return "z"
	}

	if _, ok := p.determinersN[lower]; ok {
		return "n"
	}
	if _, ok := p.pronounsN[lower]; ok {
		return "n"
	}

	if _, ok := p.adjectivesT[lower]; ok {
		return "t"
	}
	if _, ok := p.pronounsT[lower]; ok {
		return "t"
	}

	if _, ok := p.adverbsP[lower]; ok {
		return "p"
	}

	// Fallback purely by orthography: last letter of the word.
	last := lastLetter(lower)
	switch last {
	case 's', 'x', 'z':
		return "z"
	case 'n':
		return "n"
	case 'd', 't':
		return "t"
	case 'p':
		return "p"
	default:
		return ""
	}
}

// lastLetter returns the last letter rune of s, skipping any trailing
// non‑letters (punctuation, digits, etc.). If no letter is found, it
// returns 0.
func lastLetter(s string) rune {
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
		if unicode.IsLetter(r) {
			return r
		}
	}
	return 0
}

// appendLiaisonPhone appends a liaison consonant to an existing
// phonetized string, inserting a single space if both parts are
// non‑empty.
func appendLiaisonPhone(base, phone string) string {
	base = strings.TrimSpace(base)
	phone = strings.TrimSpace(phone)

	if base == "" {
		return phone
	}
	if phone == "" {
		return base
	}
	return base + " " + phone
}

// makeNormalizedSet builds a map‑set of tolerant‑normalized forms for
// a small word list (used for lexical classes like determiners).
func makeNormalizedSet(words []string) map[string]struct{} {
	m := make(map[string]struct{}, len(words))
	for _, w := range words {
		norm := tolerantNormalize(w)
		if norm == "" {
			continue
		}
		m[norm] = struct{}{}
	}
	return m
}

// isProbablyProperName returns true if the word looks like a proper
// name (first letter encountered is uppercase). This is only used to
// avoid very dubious optional liaisons such as "Robert arrive".
func isProbablyProperName(word string) bool {
	for _, r := range word {
		if !unicode.IsLetter(r) {
			continue
		}
		return unicode.IsUpper(r)
	}
	return false
}
