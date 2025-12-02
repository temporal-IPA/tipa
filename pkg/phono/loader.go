package phono

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

func init() {
	// Built-in preloaders, ordered from most specific to most generic.
	textSlashed := NewLineLoader(
		KindTxtSlashedTipa,
		sniffTxtSlashedTipa,
		parseIPADictTxtLine,
	)
	textNative := NewLineLoader(
		KindTxtTipa,
		sniffTextTipa,
		parseIPATextLine,
	)
	gobPL := &GobLoader{}

	builtinPreloaders = []Loader{
		textSlashed,
		textNative,
		gobPL,
	}

	// Fallback to the native ipa text preloader when sniffing is inconclusive.
	defaultPreloader = textNative
}

// OnEntryFunc is called by a Loader for each dictionary entry
// (word, pronunciations).
type OnEntryFunc func(word string, prons []string) error

// Loader parses a dictionary source (file or bytes) and emits
// (word, pronunciations) entries through the provided callback.
type Loader interface {
	// Kind returns a short identifier for the preloader.
	Kind() Kind

	// Sniff inspects a prefix of the input (sniff) and decides whether
	// this preloader is appropriate for the source.
	//
	// - sniff: initial bytes of the source (up to a few KB).
	// - isEOF: true if sniff contains the full source.
	Sniff(sniff []byte, isEOF bool) bool

	// Load parses the entire source from r and calls emit for each entry found
	Load(r io.Reader, emit OnEntryFunc) error

	// LoadAll loads all the dictionary
	// May be more efficient for pure loaders like GOB.
	LoadAll(r io.Reader) (Dictionary, error)
}

var (
	builtinPreloaders []Loader
	defaultPreloader  Loader
)

// RegisterPreloader allows external code to add additional Loaders
// (for example Lexique, Flexique, etc.). Preloaders are consulted in
// registration order during sniffing.
func RegisterPreloader(p Loader) {
	if p == nil {
		return
	}
	builtinPreloaders = append(builtinPreloaders, p)
}

// selectLoader chooses the first preloader whose Sniff method returns true.
// If none match, it falls back to defaultPreloader (the "native"
// text format).
func selectLoader(sniff []byte, isEOF bool) Loader {
	for _, p := range builtinPreloaders {
		if p.Sniff(sniff, isEOF) {
			return p
		}
	}
	return defaultPreloader
}

// LoadPaths preloads and merges dictionaries from a sequence of file paths.
//
// The same MergeMode semantics used by the scanner are applied between
// the preloaded dictionaries themselves, and the order of paths is respected.
//
// It returns the combined internal representation: entries, seenWordPron
// and preloadedWords (the set of words that originate from any preloaded
// dictionary). These maps can be passed directly to the wikipa scanner.
func LoadPaths(fs fs.FS, mode MergeMode, paths ...string) (entries map[string][]string, seenWordPron map[string]struct{}, preloadedWords map[string]struct{}, err error) {
	rep := NewRepresentation()
	if err := LoadInto(fs, rep, mode, paths...); err != nil {
		return nil, nil, nil, err
	}
	return rep.Entries, rep.SeenWordPron, rep.PreloadedWords, nil
}

// LoadBlobs preloads and merges dictionaries from in‑memory byte slices.
//
// Each blob is treated like a separate source, and MergeMode is applied
// between these sources in the same way as for files.
func LoadBlobs(mode MergeMode, blobs ...[]byte) (entries map[string][]string, seenWordPron map[string]struct{}, preloadedWords map[string]struct{}, err error) {
	rep := NewRepresentation()

	for _, blob := range blobs {
		if len(blob) == 0 {
			continue
		}
		sniff := blob
		isEOF := true
		if len(sniff) > sniffLen {
			sniff = sniff[:sniffLen]
			isEOF = false
		}
		pl := selectLoader(sniff, isEOF)
		if err := runLoader(pl, mode, bytes.NewReader(blob), rep); err != nil {
			return nil, nil, nil, err
		}
	}

	return rep.Entries, rep.SeenWordPron, rep.PreloadedWords, nil
}

// LoadInto preloads and merges dictionaries from a sequence of file paths
// into an existing Representation.
//
// This is useful when you want to reuse a single Representation across
// multiple sources (dumps, dictionaries, etc.) and keep consistent
// merge semantics.
func LoadInto(fs fs.FS, rep *Representation, mode MergeMode, paths ...string) error {
	if rep == nil {
		rep = NewRepresentation()
	}
	for _, p := range paths {
		path := strings.TrimSpace(p)
		if path == "" {
			continue
		}
		if err := loadFromFile(fs, rep, path, mode); err != nil {
			return err
		}
	}

	return nil
}

// loadFromFile opens a file, sniffs its format and runs the matching preloader.
func loadFromFile(fs fs.FS, rep *Representation, path string, mode MergeMode) error {
	f, err := fs.Open(path)
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

	pl := selectLoader(buf, isEOF)
	if pl == nil {
		return fmt.Errorf("no preloader matched for %s", path)
	}

	reader := io.MultiReader(bytes.NewReader(buf), f)
	return runLoader(pl, mode, reader, rep)
}

// runLoader executes a preloader, applying MergeMode semantics and
// global de‑duplication (word, pron) across all sources.
func runLoader(pl Loader, mode MergeMode, r io.Reader, rep *Representation) error {
	if pl == nil {
		return fmt.Errorf("nil preloader for ")
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

	if err := pl.Load(r, emit); err != nil {
		return fmt.Errorf("preload (%s): %w", pl.Kind(), err)
	}

	// After consuming the full dataset, record words that originate
	// from this source as "preloaded" for future merges.
	for w := range datasetWords {
		rep.PreloadedWords[w] = struct{}{}
	}

	return nil
}
