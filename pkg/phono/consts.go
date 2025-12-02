package phono

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

// Kind identifies the "type" of preloader used.
// It is mostly informational but can be useful for debugging or
// for selecting a particular preloader in user code.
type Kind string

const (
	// KindGOB identifies a gob-encoded map[string][]string.
	// used to serialize natively in golang.
	KindGOB Kind = "ipa_gob"

	// KindTxtIpa identifies the "native" ipadict text format:
	//   <word>\t<IPA1> | <IPA2> | ...
	KindTxtIpa Kind = "txt_ipa"

	// KindTxtSlashedIpa identifies the external text format with
	// slashed IPA, e.g.:
	//   a\t/a/
	// used for example by https://github.com/open-dict-data/ipa-dict
	KindTxtSlashedIpa Kind = "txt_slashed_ipa"
)

// sniffLen defines the size of the block used to sniff the type.
const sniffLen = 4 * 1024 // a few kilobytes, like http.DetectContentType
