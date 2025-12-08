package phono

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

func init() {
	// Built-in loaders, ordered from most specific to most generic.
	textSlashed := NewLineLoader(
		KindSlashedTxt,
		sniffSlashedTxt,
		parseSlashedTxtLine,
	)
	textNative := NewLineLoader(
		KindPipedTxt,
		sniffTextIpa,
		parseTextIpaLine,
	)
	gobPL := &GobLoader{}

	builtinLoaders = []Loader{
		textSlashed,
		textNative,
		gobPL,
	}

	// Fallback to the native text preloader when sniffing is inconclusive.
	defaultLoader = textNative
}

// OnEntryFunc is called by a Loader for each dictionary entry
// (expression, phonetic forms).
type OnEntryFunc func(expression string, phones []string) error

// Loader parses a dictionary source (file or bytes) and emits
// (expression, phonetic forms) entries through the provided callback.
type Loader interface {
	// Kind returns a short identifier for the preloader.
	Kind() Kind

	// Sniff inspects a prefix of the input (sniff) and decides whether
	// this preloader is appropriate for the source.
	//
	// - sniff: initial bytes of the source (up to a few KB).
	// - isEOF: true if sniff contains the full source.
	Sniff(sniff []byte, isEOF bool) bool

	// Load parses the entire source from r and calls emit for each entry found.
	Load(r io.Reader, emit OnEntryFunc) error

	// LoadAll loads the entire dictionary into memory.
	// May be more efficient for pure loaders like GOB.
	LoadAll(r io.Reader) (Dictionary, error)
}

var (
	builtinLoaders []Loader
	defaultLoader  Loader
)

// RegisterLoader allows external code to add additional Loaders
// (for example Lexique, Flexique, etc.). Loaders are consulted in
// registration order during sniffing.
func RegisterLoader(p Loader) {
	if p == nil {
		return
	}
	builtinLoaders = append(builtinLoaders, p)
}

// selectLoader chooses the first preloader whose Sniff method returns true.
// If none match, it falls back to defaultLoader (the native text format).
func selectLoader(sniff []byte, isEOF bool) Loader {
	for _, p := range builtinLoaders {
		if p.Sniff(sniff, isEOF) {
			return p
		}
	}
	return defaultLoader
}

// LoadPaths preloads and merges dictionaries from a sequence of file paths.
//
// The same MergeMode semantics used by the scanner are applied between
// the preloaded dictionaries themselves, and the order of paths is respected.
func LoadPaths(fs fs.FS, mode MergeMode, paths ...string) (Dictionary, error) {
	rep := NewRepresentation()
	if err := LoadInto(fs, rep, mode, paths...); err != nil {
		return nil, err
	}
	return rep.Entries, nil
}

// LoadBlobs preloads and merges dictionaries from in‑memory byte slices.
//
// Each blob is treated like a separate source, and MergeMode is applied
// between these sources in the same way as for files.
func LoadBlobs(mode MergeMode, blobs ...[]byte) (Dictionary, error) {
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
			return nil, err
		}
	}

	return rep.Entries, nil
}

// LoadInto preloads and merges dictionaries from a sequence of file paths
// into an existing Representation.
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
// global de‑duplication (expression, phones) across all sources.
func runLoader(pl Loader, mode MergeMode, r io.Reader, rep *Representation) error {
	if pl == nil {
		return fmt.Errorf("nil preloader for ")
	}
	datasetExpressions := make(map[string]struct{})
	replaced := make(map[string]struct{}) // used only in MergeModeReplace

	emit := func(expression string, phones []string) error {
		expression = strings.TrimSpace(expression)
		if expression == "" || len(phones) == 0 {
			return nil
		}

		datasetExpressions[expression] = struct{}{}
		baseKey := expression + "\x00"

		// In MergeModeNoOverride, expressions that already exist in the
		// preloaded dictionary are left untouched: ignore new phones.
		if mode == MergeModeNoOverride {
			if _, pre := rep.PreloadedWords[expression]; pre {
				return nil
			}
		}

		// In MergeModeReplace, the first time we see an expression that
		// comes from the existing preloaded dictionary, discard its
		// current pronunciations and start fresh.
		if mode == MergeModeReplace {
			if _, pre := rep.PreloadedWords[expression]; pre {
				if _, already := replaced[expression]; !already {
					for _, old := range rep.Entries[expression] {
						delete(rep.SeenWordPron, baseKey+old)
					}
					rep.Entries[expression] = nil
					replaced[expression] = struct{}{}
				}
			}
		}

		for _, p := range phones {
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
				rep.Entries[expression] = append([]string{p}, rep.Entries[expression]...)
			default:
				// Append mode (including no-override & replace).
				rep.Entries[expression] = append(rep.Entries[expression], p)
			}
		}

		return nil
	}

	if err := pl.Load(r, emit); err != nil {
		return fmt.Errorf("preload (%s): %w", pl.Kind(), err)
	}

	// After consuming the full dataset, record expressions that originate
	// from this source as "preloaded" for future merges.
	for expr := range datasetExpressions {
		rep.PreloadedWords[expr] = struct{}{}
	}

	return nil
}
