package conversion

import "fmt"

type Rule interface {

	// Convert a given string to another according to a rule.
	// Note that errors are encoded as string with StringWithError
	Convert(s string) string

	// Load loads from a file path.
	Load(path string) (Rule, error)

	// LoadBlob loads the rule from bytes.
	LoadBlob(blob []byte) (Rule, error)
}

func StringWithError(s string, err error) string {
	return fmt.Sprintf("ERROR:\"%s\" s:\"%s\"", err.Error(), s)
}
