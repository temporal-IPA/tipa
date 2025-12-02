// Package phonodict provides helpers to preload, merge and represent
// pronunciation dictionaries. It supports multiple input formats via
// pluggable "Loader" implementations and exposes a functional API
// for line-based textual preloaders.
package phono

import (
	"bufio"
	"bytes"
	"strings"
)

type Expression = string

type IPA = string

type Dictionary map[Expression][]IPA

// MergeMode controls how multiple sources (preloaded dictionaries,
// dumps, etc.) are combined when the same headword appears in more
// than one source.
type MergeMode int

const (
	// MergeModeAppend appends new pronunciations after existing ones.
	MergeModeAppend MergeMode = iota

	// MergeModePrepend prepends new pronunciations before existing ones.
	MergeModePrepend

	// MergeModeNoOverride does not change entries for words that already
	// exist in the preloaded dictionary. New pronunciations are only
	// added for words that are not present yet.
	MergeModeNoOverride

	// MergeModeReplace replaces entries for words that already exist in
	// the preloaded dictionary. As soon as a word appears in a new source,
	// its existing pronunciations are discarded and the new ones are kept.
	MergeModeReplace
)

// Representation holds the internal dictionary representation used
// by the scanner and the preloaders.
type Representation struct {
	Entries        Dictionary
	SeenWordPron   map[string]struct{}
	PreloadedWords map[string]struct{}
}

// NewRepresentation creates an empty Representation with reasonable
// initial capacities.
func NewRepresentation() *Representation {
	return &Representation{
		Entries:        make(Dictionary, 1<<16),
		SeenWordPron:   make(map[string]struct{}, 1<<18),
		PreloadedWords: make(map[string]struct{}),
	}
}

// Kind identifies the "type" of preloader used.
// It is mostly informational but can be useful for debugging or
// for selecting a particular preloader in user code.
type Kind string

const (
	// KindGOB identifies a gob-encoded map[string][]string.
	KindGOB Kind = "ipa_gob"

	// KindTxtTipa identifies the "native" ipadict text format:
	//   <word>\t<IPA1> | <IPA2> | ...
	KindTxtTipa Kind = "txt_tipa"

	// KindTxtSlashedTipa identifies the external text format with
	// slashed IPA, e.g.:
	//   a\t/a/
	KindTxtSlashedTipa Kind = "txt_slashed_tipa"
)

const sniffLen = 4 * 1024 // a few kilobytes, like http.DetectContentType

// --- Built-in text formats --------------------------------------------------

// sniffTxtSlashedTipa detects the ipa_dict_txt format, e.g.:
//
//	a\t/a/
//	à aucun moment\t/aokœ̃mɔmɑ̃/
//
// i.e. a tab, then an IPA string surrounded by slashes.
func sniffTxtSlashedTipa(sniff []byte, isEOF bool) bool {
	if len(sniff) == 0 {
		return false
	}
	reader := bytes.NewReader(sniff)
	scanner := bufio.NewScanner(reader)
	i := 10 // scan 10 lines.
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "\t") {
			return false
		}
		s := strings.Split(line, "\t")
		if len(s) != 2 {
			return false
		}
		if !strings.HasSuffix(s[1], "/") && !strings.HasSuffix(s[1], "/") {
			return false
		}
		i--
		if i == 0 {
			break
		}
	}
	return true
}

// sniffTextTipa detects the native ipadict text format:
//
//	<word>\t<IPA1> | <IPA2> | ...
func sniffTextTipa(sniff []byte, isEOF bool) bool {
	if len(sniff) == 0 {
		return false
	}
	reader := bytes.NewReader(sniff)
	scanner := bufio.NewScanner(reader)
	i := 10 // scan 10 lines.
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "\t") {
			return false
		}
		s := strings.Split(line, "\t")
		if len(s) != 2 {
			return false
		}
		if strings.HasSuffix(s[1], "/") {
			return false
		}
		i--
		if i == 0 {
			break
		}
	}
	return true
}

// parseIPATextLine parses a single line of the native text format:
//
//	<word>\t<IPA1> | <IPA2> | ...
func parseIPATextLine(line string) (string, []string, error) {
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return "", nil, nil
	}
	word := strings.TrimSpace(parts[0])
	rawProns := strings.TrimSpace(parts[1])
	if word == "" || rawProns == "" {
		return "", nil, nil
	}

	chunks := strings.Split(rawProns, "|")
	prons := make([]string, 0, len(chunks))
	for _, c := range chunks {
		p := strings.TrimSpace(c)
		if p == "" {
			continue
		}
		prons = append(prons, p)
	}
	if len(prons) == 0 {
		return "", nil, nil
	}
	return word, prons, nil
}

// parseIPADictTxtLine parses a single line of the ipa_dict_txt format:
//
//	<word>\t/<IPA>/
//	<word>\t/<IPA1>/ /<IPA2>/
//
// All IPA values are converted to the internal representation without
// surrounding slashes.
func parseIPADictTxtLine(line string) (string, []string, error) {
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return "", nil, nil
	}
	word := strings.TrimSpace(parts[0])
	raw := strings.TrimSpace(parts[1])
	if word == "" || raw == "" {
		return "", nil, nil
	}

	var prons []string
	rest := raw

	// Extract segments between /.../; this supports multiple IPA
	// forms on the same line, such as "/ipa1/ /ipa2/".
	for {
		start := strings.Index(rest, "/")
		if start == -1 {
			break
		}
		end := strings.Index(rest[start+1:], "/")
		if end == -1 {
			break
		}
		end += start + 1
		pron := strings.TrimSpace(rest[start+1 : end])
		if pron != "" {
			prons = append(prons, pron)
		}
		if end+1 >= len(rest) {
			break
		}
		rest = rest[end+1:]
	}

	if len(prons) == 0 {
		// Fallback: treat the whole RHS as a single pronunciation,
		// removing a single pair of surrounding slashes if present.
		trimmed := strings.Trim(raw, " ")
		if strings.HasPrefix(trimmed, "/") && strings.HasSuffix(trimmed, "/") && len(trimmed) > 2 {
			trimmed = trimmed[1 : len(trimmed)-1]
		}
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			prons = []string{trimmed}
		}
	}

	if len(prons) == 0 {
		return "", nil, nil
	}
	return word, prons, nil
}
