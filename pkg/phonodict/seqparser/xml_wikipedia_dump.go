// Package seqparser provides sequential parsers for IPA-bearing sources
// such as Wiktionary / Wikipedia XML dumps. Parsers operate in a
// streaming fashion and emit entries into a shared phonodict.Representation.
package seqparser

import (
	"bufio"
	"compress/bzip2"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/temporal-IPA/tipa/pkg/ipa"
	"github.com/temporal-IPA/tipa/pkg/phonodict"
	"golang.org/x/net/html"
)

// --- Regexes used by the XML dump parser -----------------------------------

// headwordRegex extracts the headword from lines like:
//
//	'''fauteuil''' {{pron|fo.tœj|fr}} {{m}}
var headwordRegex = regexp.MustCompile(`'''([^']+)'''`)

// pronTemplateRegex extracts full pron/API templates like:
//
//	{{pron|pʁɔ̃|fr}}, {{pron|pʁɔ̃|pʁã|fr}}, {{API|…|fr}}
var pronTemplateRegex = regexp.MustCompile(`\{\{(?:pron|API)\|([^}]*)\}\}`)

// htmlTagRegexp strips HTML-ish tags like <small>…</small>, <sup>6</sup>, <span...>, etc.
var htmlTagRegexp = regexp.MustCompile(`<[^>]+>`)

// interwikiPrefixRegex strips prefixes like :fr:foo, :en:bar, :it:JeanJean.
var interwikiPrefixRegex = regexp.MustCompile(`^:([a-z]{2,3}):(.+)$`)

// The replace / replacer pair is used to normalize a headword by removing
// various wiki markup characters that should not appear in dictionary keys.
var replace = []string{
	" ", "",
	"\t", "",
	"[", "",
	"]", "",
	"{", "",
	"}", "",
	"+", "",
	"(", "",
	")", "",
	"|", "",
}

var replacer = strings.NewReplacer(replace...)

// --- Generic helpers --------------------------------------------------------

// openLocalPossiblyCompressed opens a local file and wraps it in a bzip2
// decompressor when the path ends with ".bz2". The returned ReadCloser always
// closes the underlying file.
func openLocalPossiblyCompressed(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".bz2") {
		return struct {
			io.Reader
			io.Closer
		}{
			Reader: bzip2.NewReader(f),
			Closer: f,
		}, nil
	}

	return f, nil
}

// isHTTPURL returns true if src looks like an HTTP or HTTPS URL.
func isHTTPURL(src string) bool {
	return strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")
}

// hasBZ2SuffixURL reports whether a URL string should be treated as a .bz2
// resource, ignoring query or fragment parts.
func hasBZ2SuffixURL(raw string) bool {
	lower := strings.ToLower(raw)
	if idx := strings.IndexAny(lower, "?#"); idx >= 0 {
		lower = lower[:idx]
	}
	return strings.HasSuffix(lower, ".bz2")
}

// openHTTPPossiblyCompressed performs an HTTP GET and returns a streaming
// reader, wrapping the response body in a bzip2 decompressor when the URL
// indicates a .bz2 payload.
//
// No temporary files are created: the caller reads directly from the HTTP
// response stream.
func openHTTPPossiblyCompressed(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url) // #nosec G107 - URL is user-provided by design.
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("HTTP GET %s: unexpected status %s", url, resp.Status)
	}

	if hasBZ2SuffixURL(url) {
		return struct {
			io.Reader
			io.Closer
		}{
			Reader: bzip2.NewReader(resp.Body),
			Closer: resp.Body,
		}, nil
	}

	return resp.Body, nil
}

// openSource opens either a local file or an HTTP/HTTPS URL and wraps it in a
// bzip2 decompressor when appropriate. The returned ReadCloser must be closed
// by the caller.
func openSource(pathOrURL string) (io.ReadCloser, error) {
	if isHTTPURL(pathOrURL) {
		return openHTTPPossiblyCompressed(pathOrURL)
	}
	return openLocalPossiblyCompressed(pathOrURL)
}

// --- Extraction helpers -----------------------------------------------------

// extractPronunciationsFromLine extracts IPA pronunciations from a single line
// containing one or more {{pron|...|<lang>}} or {{API|...|<lang>}} templates.
//
// It performs a fast local de-duplication per line to reduce downstream work,
// and only keeps parameters that both:
//   - appear before the language marker, and
//   - contain at least one character from ipa.Charset.
func extractPronunciationsFromLine(line string, lang string) []string {
	matches := pronTemplateRegex.FindAllStringSubmatch(line, -1)
	if len(matches) == 0 {
		return nil
	}

	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		lang = "fr"
	}

	seen := make(map[string]struct{})
	var out []string

	for _, m := range matches {
		raw := m[1]
		if raw == "" {
			continue
		}
		parts := strings.Split(raw, "|")

		isTargetLang := false
		for _, p := range parts {
			if strings.ToLower(strings.TrimSpace(p)) == lang {
				isTargetLang = true
				break
			}
		}
		if !isTargetLang {
			continue
		}

		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.ToLower(p) == lang {
				break
			}
			if p == "" {
				continue
			}
			if !strings.ContainsAny(p, ipa.Charset) {
				continue
			}

			if _, ok := seen[p]; !ok {
				seen[p] = struct{}{}
				out = append(out, p)
			}
		}
	}

	return out
}

// normalizeHeadword cleans and filters a raw headword string.
//
// It:
//   - decodes HTML entities,
//   - strips HTML-ish tags (<small>, <sup>, malformed <spanstyle=...>, ...),
//   - strips interwiki prefixes like :fr:foo, :en:bar,
//   - applies the replacer (removing spaces, [], {}, +, (), |),
//   - trims simple trailing punctuation,
//   - rejects lines that look like bullets (#...) or contain no letters,
//   - rejects tiny slash-based artifacts like "s/s".
func normalizeHeadword(raw string) string {
	if raw == "" {
		return ""
	}

	raw = html.UnescapeString(raw)
	raw = htmlTagRegexp.ReplaceAllString(raw, "")
	raw = strings.TrimSpace(raw)

	if m := interwikiPrefixRegex.FindStringSubmatch(raw); len(m) == 3 {
		raw = strings.TrimSpace(m[2])
	}

	raw = replacer.Replace(raw)
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "#") {
		return ""
	}

	raw = strings.Trim(raw, ".,;:")

	if raw == "" {
		return ""
	}

	hasLetter := false
	letterCount := 0
	for _, r := range raw {
		if unicode.IsLetter(r) {
			hasLetter = true
			letterCount++
		}
	}
	if !hasLetter {
		return ""
	}

	if strings.Contains(raw, "/") && letterCount <= 2 {
		return ""
	}

	return raw
}

// extractHeadwordFromLine returns a normalized headword for the current line,
// falling back to the page title when no explicit ”'headword”' is present.
func extractHeadwordFromLine(line, title string) string {
	raw := title
	if m := headwordRegex.FindStringSubmatch(line); len(m) > 1 {
		raw = strings.TrimSpace(m[1])
	}
	return normalizeHeadword(raw)
}

// --- Public parser API ------------------------------------------------------

// XMLWikipediaStats contains summary statistics for a parsing run.
type XMLWikipediaStats struct {
	Lines       int
	Words       int
	UniquePairs int
	Elapsed     time.Duration
}

// XMLWikipediaDump parses Wiktionary / Wikipedia XML dumps and merges the
// extracted pronunciations into a phonodict.Representation.
type XMLWikipediaDump struct {
	// Lang is the language code used in {{pron|...}} / {{API|...}} templates.
	Lang string

	// Mode controls how pronunciations from the dump are merged with any
	// preloaded dictionaries in the Representation.
	Mode phonodict.MergeMode

	// Progress, if non-nil, is called periodically with the current
	// line count, word count and unique (word, pron) pair count.
	Progress func(lineCount int, wordCount int, uniquePairs int)
}

// NewXMLWikipediaDump constructs a parser for the given language and merge mode.
func NewXMLWikipediaDump(lang string, mode phonodict.MergeMode) *XMLWikipediaDump {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		lang = "fr"
	}
	return &XMLWikipediaDump{
		Lang: lang,
		Mode: mode,
	}
}

// ParseSource opens a local file or HTTP/HTTPS URL (optionally .bz2-compressed),
// parses the dump and merges the entries into rep. It returns summary stats.
func (p *XMLWikipediaDump) ParseSource(pathOrURL string, rep *phonodict.Representation) (XMLWikipediaStats, error) {
	if rep == nil {
		rep = phonodict.NewRepresentation()
	}

	reader, err := openSource(pathOrURL)
	if err != nil {
		return XMLWikipediaStats{}, fmt.Errorf("open %q: %w", pathOrURL, err)
	}
	defer reader.Close()

	return p.Parse(reader, rep)
}

// Parse reads a dump from reader, updating rep in place.
//
// rep may already contain entries and preloaded words; merge semantics
// follow p.Mode and are consistent with phonodict's merge logic.
func (p *XMLWikipediaDump) Parse(reader io.Reader, rep *phonodict.Representation) (XMLWikipediaStats, error) {
	if rep == nil {
		rep = phonodict.NewRepresentation()
	}

	stats := XMLWikipediaStats{}
	start := time.Now()

	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)

	var (
		title  string
		inText bool
	)

	const progressStep = 100000

	replaced := make(map[string]struct{})

	lang := strings.ToLower(strings.TrimSpace(p.Lang))
	if lang == "" {
		lang = "fr"
	}

	mode := p.Mode

	for scanner.Scan() {
		line := scanner.Text()
		stats.Lines++

		if p.Progress != nil && stats.Lines%progressStep == 0 {
			p.Progress(stats.Lines, len(rep.Entries), len(rep.SeenWordPron))
		}

		if strings.Contains(line, "<title>") && strings.Contains(line, "</title>") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "<title>") && strings.Contains(trim, "</title>") {
				startIdx := strings.Index(trim, "<title>") + len("<title>")
				endIdx := strings.Index(trim, "</title>")
				if endIdx > startIdx {
					title = trim[startIdx:endIdx]
				}
			}
		}

		if strings.Contains(line, "<text") {
			inText = true
		}
		if strings.Contains(line, "</text>") {
			inText = false
		}

		if !inText {
			continue
		}

		if !strings.Contains(line, "{{pron|") && !strings.Contains(line, "{{API|") {
			continue
		}

		word := extractHeadwordFromLine(line, title)
		if word == "" {
			continue
		}

		if mode == phonodict.MergeModeNoOverride {
			if _, pre := rep.PreloadedWords[word]; pre {
				continue
			}
		}

		prons := extractPronunciationsFromLine(line, lang)
		if len(prons) == 0 {
			continue
		}

		if mode == phonodict.MergeModeReplace {
			if _, pre := rep.PreloadedWords[word]; pre {
				if _, already := replaced[word]; !already {
					baseKey := word + "\x00"
					for _, old := range rep.Entries[word] {
						delete(rep.SeenWordPron, baseKey+old)
					}
					rep.Entries[word] = nil
					replaced[word] = struct{}{}
				}
			}
		}

		baseKey := word + "\x00"
		for _, pron := range prons {
			key := baseKey + pron
			if _, ok := rep.SeenWordPron[key]; ok {
				continue
			}
			rep.SeenWordPron[key] = struct{}{}

			switch mode {
			case phonodict.MergeModePrepend:
				rep.Entries[word] = append([]string{pron}, rep.Entries[word]...)
			default:
				rep.Entries[word] = append(rep.Entries[word], pron)
			}
		}
	}

	stats.Words = len(rep.Entries)
	stats.UniquePairs = len(rep.SeenWordPron)
	stats.Elapsed = time.Since(start)

	if err := scanner.Err(); err != nil {
		return stats, err
	}

	return stats, nil
}
