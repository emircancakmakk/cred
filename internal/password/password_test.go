package password

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	for range 100 {
		value, err := Generate()
		if err != nil {
			t.Fatal(err)
		}
		if len(value) != Length || strings.Trim(value, alphabet) != "" {
			t.Fatal("invalid password length or character")
		}
		for _, group := range []string{lower, upper, digits, symbols} {
			if !strings.ContainsAny(value, group) {
				t.Fatal("missing character group")
			}
		}
	}
}

func TestGenerateRejectsBiasedBytesAndIncompleteCandidates(t *testing.T) {
	// Discard a high byte and an all-lowercase candidate before accepting a
	// candidate containing the first character of each group.
	source := append([]byte{255}, make([]byte, Length)...)
	candidate := make([]byte, Length)
	candidate[1], candidate[2], candidate[3] = byte(len(lower)), byte(len(lower+upper)), byte(len(lower+upper+digits))
	source = append(source, candidate...)
	value, err := generate(bytes.NewReader(source))
	if err != nil || value != "aA0!"+strings.Repeat("a", Length-4) {
		t.Fatal("rejection sampling failed")
	}
}

func TestGenerateSourceFailure(t *testing.T) {
	for _, source := range []io.Reader{bytes.NewReader(nil), bytes.NewReader([]byte{0, 1, 2})} {
		value, err := generate(source)
		if err == nil || value != "" {
			t.Fatal("source failure returned a partial password")
		}
	}
}
