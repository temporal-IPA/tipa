package psr

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/temporal-IPA/tipa/pkg/conversion"
)

// Rule implements a simple conversion mechanism.
// that relies on presets.
type Rule struct {
	Prefixes     map[string]string `json:"prefixes"`
	Suffixes     map[string]string `json:"suffixes"`
	Replacements map[string]string `json:"replacements"`
}

// Load loads from a file path.
func Load(path string) (conversion.Rule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return load(b)
}

// LoadBlob loads the rule from bytes.
func LoadBlob(blob []byte) (conversion.Rule, error) {
	return load(blob)
}

func load(b []byte) (rule conversion.Rule, err error) {
	r := &Rule{}
	err = json.Unmarshal(b, &r)
	return rule, err
}

// isIPAKey reports whether a key encodes an IPA sequence.
//
// By convention IPA sequences are surrounded with '/' on both
// sides in the JSON rule definition (for example "/u/" or
// "/fyzi/"). A single slash or a pair of slashes without any
// content inside is not considered a valid IPA key.
func isIPAKey(k string) bool {
	return len(k) > 2 && strings.HasPrefix(k, "/") && strings.HasSuffix(k, "/")
}

// applyRuleSet applies the three-step process to a string:
//
//  1. Prefix substitutions.
//  2. Suffix substitutions.
//  3. In-string replacements.
//
// The same algorithm is used both for IPA rules and for
// textual rules; the caller chooses which maps to pass.
func applyRuleSet(s string, prefixes, suffixes, replacements map[string]string) string {
	// Step 1: prefix handling.
	for k, v := range prefixes {
		if strings.HasPrefix(s, k) {
			s = v + s[len(k):]
		}
	}

	// Step 2: suffix handling.
	for k, v := range suffixes {
		if strings.HasSuffix(s, k) {
			s = s[:len(s)-len(k)] + v
		}
	}

	// Step 3: generic replacements.
	for k, v := range replacements {
		s = strings.Replace(s, k, v, -1)
	}

	return s
}

// Convert implement transform.Rule.
//
// It supports both textual and IPA entries.
//
// When for example we have IPA to txt
// "/u/":"ou", "/ø/" : "eu"
// When an entry (it can be multi char) is defined in IPA it is between slashes.
// It is more expressive so it primes on any thing else e.g: "/fyzi/":"fuzzi" instead of "fusil".
// IPA always primes.
//
// Concretely, the algorithm is:
//
//  1. Extract all IPA rules (keys between leading and trailing '/').
//  2. Apply the three steps (prefix, suffix, replacements) using only
//     those IPA rules.
//  3. Apply the three steps again using only textual (non-IPA) rules.
func (r Rule) Convert(s string) string {
	// Split prefixes into IPA and textual sets.
	ipaPrefixes := make(map[string]string)
	textPrefixes := make(map[string]string)
	for k, v := range r.Prefixes {
		if isIPAKey(k) {
			k = strings.Replace(k, "/", "", -1)
			ipaPrefixes[k] = v
		} else {
			textPrefixes[k] = v
		}
	}

	// Split suffixes into IPA and textual sets.
	ipaSuffixes := make(map[string]string)
	textSuffixes := make(map[string]string)
	for k, v := range r.Suffixes {
		if isIPAKey(k) {
			k = strings.Replace(k, "/", "", -1)
			ipaSuffixes[k] = v
		} else {
			textSuffixes[k] = v
		}
	}

	// Split replacements into IPA and textual sets.
	ipaReplacements := make(map[string]string)
	textReplacements := make(map[string]string)
	for k, v := range r.Replacements {
		if isIPAKey(k) {
			k = strings.Replace(k, "/", "", -1)
			ipaReplacements[k] = v
		} else {
			textReplacements[k] = v
		}
	}

	// First pass: IPA rules (always prime).
	s = applyRuleSet(s, ipaPrefixes, ipaSuffixes, ipaReplacements)

	// Second pass: textual rules.
	s = applyRuleSet(s, textPrefixes, textSuffixes, textReplacements)

	return s
}
