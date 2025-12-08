package phono

import (
	"strings"
	"testing"
)

func TestParseSlashedTxtLine_MultiplePhones_WithSeparators(t *testing.T) {
	line := "expr\t/p1/;/p2/ # comment"
	expr, phones, err := parseSlashedTxtLine(line)
	if err != nil {
		t.Fatalf("parseSlashedTxtLine returned error: %v", err)
	}
	if expr != "expr" {
		t.Fatalf("unexpected expression: got %q, want %q", expr, "expr")
	}
	if len(phones) != 2 || phones[0] != "p1" || phones[1] != "p2" {
		t.Fatalf("unexpected phones: %#v, want [p1 p2]", phones)
	}

	// Variant with comma and missing trailing slash on the last form.
	line = "expr /p1/ , /p2"
	expr, phones, err = parseSlashedTxtLine(line)
	if err != nil {
		t.Fatalf("parseSlashedTxtLine returned error: %v", err)
	}
	if expr != "expr" {
		t.Fatalf("unexpected expression: got %q, want %q", expr, "expr")
	}
	if len(phones) != 2 || phones[0] != "p1" || phones[1] != "p2" {
		t.Fatalf("unexpected phones (comma variant): %#v, want [p1 p2]", phones)
	}
}

func TestParseSlashedTxtLine_ExpressionTrimmed(t *testing.T) {
	line := "   benoit pereira da silva   \t  /bɛnwɑ/ "
	expr, phones, err := parseSlashedTxtLine(line)
	if err != nil {
		t.Fatalf("parseSlashedTxtLine returned error: %v", err)
	}
	if expr != "benoit pereira da silva" {
		t.Fatalf("unexpected expression: got %q, want %q", expr, "benoit pereira da silva")
	}
	if len(phones) != 1 || phones[0] != "bɛnwɑ" {
		t.Fatalf("unexpected phones: %#v, want [bɛnwɑ]", phones)
	}
}

func TestLineLoader_RemovesInlineComments(t *testing.T) {
	content := `
# global comment
hello   /hɛlo/    # inline comment
world	/wɔʁld/## another comment
`

	loader := NewLineLoader(
		KindSlashedTxt,
		sniffSlashedTxt,
		parseSlashedTxtLine,
	)

	dict, err := loader.LoadAll(strings.NewReader(content))
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if len(dict) != 2 {
		t.Fatalf("expected 2 expressions, got %d (%#v)", len(dict), dict)
	}
	if got := dict["hello"]; len(got) != 1 || got[0] != "hɛlo" {
		t.Fatalf("unexpected entry for 'hello': %#v", got)
	}
	if got := dict["world"]; len(got) != 1 || got[0] != "wɔʁld" {
		t.Fatalf("unexpected entry for 'world': %#v", got)
	}
}

func TestSniffSlashedTxt_SkipsComments(t *testing.T) {
	data := []byte(`# comment line
expr	/p1/
`)
	if !sniffSlashedTxt(data, true) {
		t.Fatalf("sniffSlashedTxt should detect slashed format when comments precede data")
	}
}

func TestSniffSlashedText_SkipsComments(t *testing.T) {
	data := []byte(`# comment line
expr	phon1 | phon2
`)
	if !sniffTextIpa(data, true) {
		t.Fatalf("sniffTextIpa should detect native text format when comments precede data")
	}
}
