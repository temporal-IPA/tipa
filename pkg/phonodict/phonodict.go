// Package phonodict provides helpers to preload, merge and represent
// pronunciation dictionaries. It supports multiple input formats via
// pluggable "Preloader" implementations and exposes a functional API
// for line-based textual preloaders.
package phonodict

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

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
	Entries        map[string][]string
	SeenWordPron   map[string]struct{}
	PreloadedWords map[string]struct{}
}

// NewRepresentation creates an empty Representation with reasonable
// initial capacities.
func NewRepresentation() *Representation {
	return &Representation{
		Entries:        make(map[string][]string, 1<<16),
		SeenWordPron:   make(map[string]struct{}, 1<<18),
		PreloadedWords: make(map[string]struct{}),
	}
}

// PreloadKind identifies the "type" of preloader used.
// It is mostly informational but can be useful for debugging or
// for selecting a particular preloader in user code.
type PreloadKind string

const (
	// PreloadKindGOB identifies a gob-encoded map[string][]string.
	PreloadKindGOB PreloadKind = "ipa_gob"

	// PreloadKindText identifies the "native" wikipa text format:
	//   <word>\t<IPA1> | <IPA2> | ...
	PreloadKindText PreloadKind = "ipa_text"

	// PreloadKindIPADictTxt identifies the external text format with
	// slashed IPA, e.g.:
	//   a\t/a/
	PreloadKindIPADictTxt PreloadKind = "ipa_dict_txt"
)

// OnEntryFunc is called by a Preloader for each dictionary entry
// (word, pronunciations).
type OnEntryFunc func(word string, prons []string) error

// Preloader parses a dictionary source (file or bytes) and emits
// (word, pronunciations) entries through the provided callback.
type Preloader interface {
	// Kind returns a short identifier for the preloader.
	Kind() PreloadKind

	// Sniff inspects a prefix of the input (sniff) and decides whether
	// this preloader is appropriate for the source.
	//
	// - path: optional file path or virtual name (may be empty).
	// - sniff: initial bytes of the source (up to a few KB).
	// - isEOF: true if sniff contains the full source.
	Sniff(path string, sniff []byte, isEOF bool) bool

	// Load parses the entire source from r and calls emit for each
	// entry found. The path argument is purely informational.
	Load(path string, r io.Reader, emit OnEntryFunc) error
}

// LineParser is a per-line parser for text-based formats.
// It receives a single logical line (with surrounding whitespace
// already trimmed) and should return the word and its pronunciations.
// If the line should be ignored, it can return word == "" or len(prons) == 0.
type LineParser func(line string) (word string, prons []string, err error)

// NewLinePreloader constructs a Preloader that reads a text source
// line by line and delegates actual parsing to the provided LineParser.
// This makes it easy to support additional textual formats (e.g. Lexique,
// Flexique, custom tab-separated dictionaries).
func NewLinePreloader(
	kind PreloadKind,
	sniff func(path string, sniff []byte, isEOF bool) bool,
	parser LineParser,
) Preloader {
	return &linePreloader{
		kind:      kind,
		sniffFunc: sniff,
		parseLine: parser,
	}
}

// linePreloader is a generic implementation for textual formats where
// each entry fits on a single line.
type linePreloader struct {
	kind      PreloadKind
	sniffFunc func(path string, sniff []byte, isEOF bool) bool
	parseLine LineParser
}

func (p *linePreloader) Kind() PreloadKind { return p.kind }

func (p *linePreloader) Sniff(path string, sniff []byte, isEOF bool) bool {
	if p.sniffFunc == nil {
		return false
	}
	return p.sniffFunc(path, sniff, isEOF)
}

func (p *linePreloader) Load(path string, r io.Reader, emit OnEntryFunc) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		word, prons, err := p.parseLine(line)
		if err != nil {
			return fmt.Errorf("%s (%s): parse line %q: %w", path, p.kind, line, err)
		}
		if word == "" || len(prons) == 0 {
			continue
		}
		if err := emit(word, prons); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// gobPreloader handles gob-encoded map[string][]string dictionaries.
type gobPreloader struct{}

func (g *gobPreloader) Kind() PreloadKind { return PreloadKindGOB }

func (g *gobPreloader) Sniff(path string, sniff []byte, isEOF bool) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".gob") {
		return true
	}
	if len(sniff) == 0 {
		return false
	}
	// If it's not valid UTF-8, it's very likely a binary (gob) payload.
	if !utf8.Valid(sniff) {
		return true
	}
	// Heuristic: presence of NUL bytes strongly suggests a binary format.
	for _, b := range sniff {
		if b == 0 {
			return true
		}
	}
	return false
}

func (g *gobPreloader) Load(path string, r io.Reader, emit OnEntryFunc) error {
	dec := gob.NewDecoder(r)
	dict := make(map[string][]string)
	if err := dec.Decode(&dict); err != nil {
		return fmt.Errorf("decode gob %s: %w", path, err)
	}
	for w, prons := range dict {
		if len(prons) == 0 {
			continue
		}
		if err := emit(w, prons); err != nil {
			return err
		}
	}
	return nil
}

const sniffLen = 4 * 1024 // a few kilobytes, like http.DetectContentType

var (
	builtinPreloaders []Preloader
	defaultPreloader  Preloader
)

// RegisterPreloader allows external code to add additional preloaders
// (for example Lexique, Flexique, etc.). Preloaders are consulted in
// registration order during sniffing.
func RegisterPreloader(p Preloader) {
	if p == nil {
		return
	}
	builtinPreloaders = append(builtinPreloaders, p)
}

// selectPreloader chooses the first preloader whose Sniff method returns true.
// If none match, it falls back to defaultPreloader (the "native"
// text format).
func selectPreloader(path string, sniff []byte, isEOF bool) Preloader {
	for _, p := range builtinPreloaders {
		if p.Sniff(path, sniff, isEOF) {
			return p
		}
	}
	return defaultPreloader
}

// PreloadPaths preloads and merges dictionaries from a sequence of file paths.
//
// The same MergeMode semantics used by the scanner are applied between
// the preloaded dictionaries themselves, and the order of paths is respected.
//
// It returns the combined internal representation: entries, seenWordPron
// and preloadedWords (the set of words that originate from any preloaded
// dictionary). These maps can be passed directly to the wikipa scanner.
func PreloadPaths(mode MergeMode, paths ...string) (entries map[string][]string, seenWordPron map[string]struct{}, preloadedWords map[string]struct{}, err error) {
	rep := NewRepresentation()

	for _, p := range paths {
		path := strings.TrimSpace(p)
		if path == "" {
			continue
		}
		if err := preloadFromFile(rep, path, mode); err != nil {
			return nil, nil, nil, err
		}
	}

	return rep.Entries, rep.SeenWordPron, rep.PreloadedWords, nil
}

// PreloadBlobs preloads and merges dictionaries from in‑memory byte slices.
//
// Each blob is treated like a separate source, and MergeMode is applied
// between these sources in the same way as for files.
func PreloadBlobs(mode MergeMode, blobs ...[]byte) (entries map[string][]string, seenWordPron map[string]struct{}, preloadedWords map[string]struct{}, err error) {
	rep := NewRepresentation()

	for i, blob := range blobs {
		if len(blob) == 0 {
			continue
		}
		virtualPath := fmt.Sprintf("blob#%d", i)

		sniff := blob
		isEOF := true
		if len(sniff) > sniffLen {
			sniff = sniff[:sniffLen]
			isEOF = false
		}

		pl := selectPreloader(virtualPath, sniff, isEOF)
		if pl == nil {
			return nil, nil, nil, fmt.Errorf("no preloader matched for %s", virtualPath)
		}

		if err := runPreloader(pl, virtualPath, mode, bytes.NewReader(blob), rep); err != nil {
			return nil, nil, nil, err
		}
	}

	return rep.Entries, rep.SeenWordPron, rep.PreloadedWords, nil
}

// preloadFromFile opens a file, sniffs its format and runs the matching preloader.
func preloadFromFile(rep *Representation, path string, mode MergeMode) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	buf := make([]byte, sniffLen)
	n, readErr := io.ReadFull(f, buf)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		return fmt.Errorf("sniff %s: %w", path, readErr)
	}
	buf = buf[:n]
	isEOF := readErr == io.EOF || readErr == io.ErrUnexpectedEOF || n == 0

	pl := selectPreloader(path, buf, isEOF)
	if pl == nil {
		return fmt.Errorf("no preloader matched for %s", path)
	}

	reader := io.MultiReader(bytes.NewReader(buf), f)
	return runPreloader(pl, path, mode, reader, rep)
}

// runPreloader executes a preloader, applying MergeMode semantics and
// global de‑duplication (word, pron) across all sources.
func runPreloader(pl Preloader, path string, mode MergeMode, r io.Reader, rep *Representation) error {
	if pl == nil {
		return fmt.Errorf("nil preloader for %s", path)
	}

	datasetWords := make(map[string]struct{})
	replaced := make(map[string]struct{}) // used only in MergeModeReplace

	emit := func(word string, prons []string) error {
		word = strings.TrimSpace(word)
		if word == "" || len(prons) == 0 {
			return nil
		}

		datasetWords[word] = struct{}{}
		baseKey := word + "\x00"

		// In MergeModeNoOverride, words that already exist in the
		// preloaded dictionary are left untouched: ignore new prons.
		if mode == MergeModeNoOverride {
			if _, pre := rep.PreloadedWords[word]; pre {
				return nil
			}
		}

		// In MergeModeReplace, the first time we see a word that comes
		// from the existing preloaded dictionary, discard its current
		// pronunciations and start fresh.
		if mode == MergeModeReplace {
			if _, pre := rep.PreloadedWords[word]; pre {
				if _, already := replaced[word]; !already {
					for _, old := range rep.Entries[word] {
						delete(rep.SeenWordPron, baseKey+old)
					}
					rep.Entries[word] = nil
					replaced[word] = struct{}{}
				}
			}
		}

		for _, p := range prons {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			key := baseKey + p
			if _, ok := rep.SeenWordPron[key]; ok {
				continue
			}
			rep.SeenWordPron[key] = struct{}{}

			switch mode {
			case MergeModePrepend:
				rep.Entries[word] = append([]string{p}, rep.Entries[word]...)
			default:
				// Append mode (including no-override & replace).
				rep.Entries[word] = append(rep.Entries[word], p)
			}
		}

		return nil
	}

	if err := pl.Load(path, r, emit); err != nil {
		return fmt.Errorf("preload %s (%s): %w", path, pl.Kind(), err)
	}

	// After consuming the full dataset, record words that originate
	// from this source as "preloaded" for future merges.
	for w := range datasetWords {
		rep.PreloadedWords[w] = struct{}{}
	}

	return nil
}

// --- Built-in text formats --------------------------------------------------

// sniffIPADictTxt detects the ipa_dict_txt format, e.g.:
//
//	a\t/a/
//	à aucun moment\t/aokœ̃mɔmɑ̃/
//
// i.e. a tab, then an IPA string surrounded by slashes.
func sniffIPADictTxt(path string, sniff []byte, isEOF bool) bool {
	if len(sniff) == 0 {
		return false
	}
	if !utf8.Valid(sniff) {
		return false
	}
	if !bytes.Contains(sniff, []byte("\t")) {
		return false
	}

	idx := bytes.IndexByte(sniff, '\t')
	if idx == -1 || idx+1 >= len(sniff) {
		return false
	}
	rest := sniff[idx+1:]
	rest = bytes.TrimLeft(rest, " ")

	if len(rest) == 0 || rest[0] != '/' {
		return false
	}

	// Ensure we have at least a closing slash.
	if bytes.IndexByte(rest[1:], '/') == -1 {
		return false
	}

	return true
}

// sniffIPAText detects the native wikipa text format:
//
//	<word>\t<IPA1> | <IPA2> | ...
//
// It is deliberately permissive: as long as we see a tab and it does
// not look like "tab + /", we treat it as ipa_text.
func sniffIPAText(path string, sniff []byte, isEOF bool) bool {
	if len(sniff) == 0 {
		return false
	}
	if !utf8.Valid(sniff) {
		return false
	}
	if !bytes.Contains(sniff, []byte("\t")) {
		return false
	}
	// ipa_dict_txt has TAB followed by a slash; we avoid matching that here.
	if bytes.Contains(sniff, []byte("\t/")) || bytes.Contains(sniff, []byte("\t /")) {
		return false
	}
	// Prefer this format when we see the " | " separator.
	if bytes.Contains(sniff, []byte(" | ")) {
		return true
	}
	// Fallback: generic word<TAB>pronunciation lines.
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

func init() {
	// Register built-in preloaders in order of specificity:
	//   1) gob
	//   2) ipa_dict_txt
	//   3) ipa_text (fallback for generic tab-separated text dictionaries)
	textPreloader := NewLinePreloader(
		PreloadKindText,
		sniffIPAText,
		parseIPATextLine,
	)

	dictTxtPreloader := NewLinePreloader(
		PreloadKindIPADictTxt,
		sniffIPADictTxt,
		parseIPADictTxtLine,
	)

	builtinPreloaders = []Preloader{
		&gobPreloader{},
		dictTxtPreloader,
		textPreloader,
	}

	defaultPreloader = textPreloader
}
