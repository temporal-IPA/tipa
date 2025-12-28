package g2p

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/benoit-pereira-da-silva/textual/pkg/carrier"
)

// FrenchLiaison applies simple French liaison heuristics on top of a
// grapheme‑to‑phoneme textual.Parcel.
//
// It implements textual.Processor so that it can be used directly in
// textual.Chain, Router, IOReaderProcessor, Transformation, etc.
type FrenchLiaison[S carrier.Parcel] struct {
	// allowLooseLiaison enables the optional heuristic that tries to
	// guess liaison consonants for a broader set of words, beyond the
	// curated determiner/pronoun/verb/adverb sets.
	allowLooseLiaison bool

	determinersZ map[string]struct{}
	determinersN map[string]struct{}
	adjectivesT  map[string]struct{}
	pronounsZ    map[string]struct{}
	pronounsN    map[string]struct{}
	verbsT       map[string]struct{}
	adverbsZ     map[string]struct{}
	adverbsP     map[string]struct{}

	forbidBefore map[string]struct{}
	hAspire      map[string]struct{}
}

// NewFrenchLiaison constructs a conservative FrenchLiaison processor
// that only inserts liaison consonants for a curated set of grammatical
// contexts (determiners, pronouns, verbs, etc.).
func NewFrenchLiaison[S carrier.Parcel]() *FrenchLiaison[S] {
	return newFrenchLiaison[S](false)
}

// NewFrenchLiaisonWithFallback constructs a FrenchLiaison processor that
// also enables a loose heuristic to guess liaison consonants in a wider
// range of contexts when they are not explicitly listed.
func NewFrenchLiaisonWithFallback[S carrier.Parcel]() *FrenchLiaison[S] {
	return newFrenchLiaison[S](true)
}

// newFrenchLiaison initialises the internal lexicons used by the liaison
// heuristics. The allowLoose flag controls whether the broader heuristic
// is enabled.
func newFrenchLiaison[S carrier.Parcel](allowLoose bool) *FrenchLiaison[S] {
	p := &FrenchLiaison[S]{
		allowLooseLiaison: allowLoose,
	}

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
	p.verbsT = makeNormalizedSet([]string{
		"est", "sont", "ait", "était", "étaient",
	})
	p.adverbsZ = makeNormalizedSet([]string{
		"très", "tres",
	})
	p.adverbsP = makeNormalizedSet([]string{
		"trop",
	})
	p.forbidBefore = makeNormalizedSet([]string{
		"et",
	})
	p.hAspire = makeNormalizedSet([]string{
		"haricot", "honte", "héros", "heros", "huitre",
	})

	return p
}

// Apply implements the textual.Processor interface.
//
// For each incoming Parcel, FrenchLiaison analyses the orthographic word
// sequence and inserts liaison consonants into Fragment.Transformed when
// appropriate. The original text and fragment coordinates are preserved.
func (p *FrenchLiaison[S]) Apply(ctx context.Context, in <-chan carrier.Parcel) <-chan carrier.Parcel {
	if ctx == nil {
		ctx = context.Background()
	}

	out := make(chan carrier.Parcel)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				// Stop emitting results but drain upstream to avoid
				// blocking senders.
				for range in {
				}
				return
			case res, ok := <-in:
				if !ok {
					// Upstream closed: no more input.
					return
				}

				processed := p.processResult(res)

				select {
				case <-ctx.Done():
					// Context canceled while sending.
					return
				case out <- processed:
				}
			}
		}
	}()

	return out
}

// --- tokens and helpers ---

type orthToken struct {
	text      string
	norm      string
	runeStart int
	runeLen   int
	fragIndex int
}

// processResult takes a g2p Parcel and returns a new Parcel with liaison
// consonants inserted into Fragment.Transformed when appropriate.
func (p *FrenchLiaison[S]) processResult(res carrier.Parcel) carrier.Parcel {
	if len(res.Text) == 0 || len(res.Fragments) == 0 {
		return res
	}

	out := res
	out.Fragments = make([]carrier.Fragment, len(res.Fragments))
	copy(out.Fragments, res.Fragments)

	runes := []rune(string(res.Text))
	tokens := tokenizeFrenchWords(string(res.Text))
	if len(tokens) < 2 {
		return out
	}

	attachFragmentsToTokens(tokens, out.Fragments)

	for i := 0; i < len(tokens)-1; i++ {
		left := &tokens[i]
		right := &tokens[i+1]

		if left.fragIndex < 0 || right.fragIndex < 0 {
			continue
		}
		if hasStrongBoundary(runes, left, right) {
			continue
		}
		if _, forbidden := p.forbidBefore[left.norm]; forbidden {
			continue
		}
		if !p.startsWithVowelOrHMuet(left.text, right.text) {
			continue
		}
		liaisonPhone := p.liaisonPhoneFor(left)
		if liaisonPhone == "" {
			continue
		}
		p.insertLiaisonConsonant(
			&out.Fragments[left.fragIndex],
			&out.Fragments[right.fragIndex],
			liaisonPhone,
		)
	}

	return out
}

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

func attachFragmentsToTokens(tokens []orthToken, fragments []carrier.Fragment) {
	if len(tokens) == 0 || len(fragments) == 0 {
		return
	}

	iFrag := 0
	for i := range tokens {
		tok := &tokens[i]

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

// startsWithVowelOrHMuet: right word must start with vowel or non‑aspirated h.
func (p *FrenchLiaison[S]) startsWithVowelOrHMuet(left, right string) bool {
	norm := tolerantNormalize(right)
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

func (p *FrenchLiaison[S]) liaisonPhoneFor(tok *orthToken) string {
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

func (p *FrenchLiaison[S]) isLiaisonGiver(norm string) bool {
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
	if _, ok := p.verbsT[norm]; ok {
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

func (p *FrenchLiaison[S]) guessLiaisonPhone(word string) string {
	lower := tolerantNormalize(word)

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
	if _, ok := p.verbsT[lower]; ok {
		return "t"
	}

	if _, ok := p.adverbsP[lower]; ok {
		return "p"
	}

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

func (p *FrenchLiaison[S]) insertLiaisonConsonant(leftFrag, rightFrag *carrier.Fragment, phone string) {
	phone = strings.TrimSpace(phone)
	if phone == "" || leftFrag == nil {
		return
	}

	if rightFrag != nil {
		base := strings.TrimSpace(string(rightFrag.Transformed))
		if base != "" {
			rightFrag.Transformed = carrier.UTF8String(phone + base)
			return
		}
	}
	leftFrag.Transformed = carrier.UTF8String(appendLiaisonPhone(string(leftFrag.Transformed), phone))
}

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

func isProbablyProperName(word string) bool {
	for _, r := range word {
		if !unicode.IsLetter(r) {
			continue
		}
		return unicode.IsUpper(r)
	}
	return false
}
