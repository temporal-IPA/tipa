package phono

// MergeMode controls how multiple sources (preloaded dictionaries,
// dumps, etc.) are combined when the same expression appears in more
// than one source.
type MergeMode int

const (
	// MergeModeAppend appends new pronunciations after existing ones.
	MergeModeAppend MergeMode = iota

	// MergeModePrepend prepends new pronunciations before existing ones.
	MergeModePrepend

	// MergeModeNoOverride does not change entries for expressions that
	// already exist in the preloaded dictionary. New pronunciations are
	// only added for expressions that are not present yet.
	MergeModeNoOverride

	// MergeModeReplace replaces entries for expressions that already
	// exist in the preloaded dictionary. As soon as an expression appears
	// in a new source, its existing pronunciations are discarded and the
	// new ones are kept.
	MergeModeReplace
)

// Kind identifies the "type" of preloader used.
// It is mostly informational but can be useful for debugging or
// for selecting a particular preloader in user code.
type Kind string

const (
	// KindGOB identifies a gob‑encoded Dictionary (map[Expression][]string),
	// used to serialize dictionaries natively in Go.
	KindGOB Kind = "ipa_gob"

	// KindPipedTxt identifies the "native" tab‑separated text format:
	//   <expression>\t<phon1> | <phon2> | ...
	// The phonetic strings can be any encoding.
	KindPipedTxt Kind = "piped_txt"

	// KindSlashedTxt identifies the external text format with
	// slashed phonetic strings, e.g.:
	//   expression\t/phones/
	//   expression   /phones1/;/phones2/
	KindSlashedTxt Kind = "slashed_txt"
)

// sniffLen defines the size of the block used to sniff the type.
const sniffLen = 4 * 1024 // a few kilobytes, like http.DetectContentType
