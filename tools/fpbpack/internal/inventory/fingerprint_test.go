package inventory

import (
	"strings"
	"testing"
)

func TestCurseForgeFingerprint(t *testing.T) {
	const expected uint32 = 197930586

	got, err := computeCurseForgeFingerprint(strings.NewReader("foo"), 3)
	if err != nil {
		t.Fatal(err)
	}
	if got != expected {
		t.Fatalf("fingerprint(foo) = %d, want %d", got, expected)
	}

	const spaced = "f o\no\t"
	length := uint32(0)
	for i := 0; i < len(spaced); i++ {
		if !isCurseForgeWhitespace(spaced[i]) {
			length++
		}
	}
	got, err = computeCurseForgeFingerprint(strings.NewReader(spaced), length)
	if err != nil {
		t.Fatal(err)
	}
	if got != expected {
		t.Fatalf("fingerprint(whitespace-normalized foo) = %d, want %d", got, expected)
	}
}

func TestNormalizedLengthWriter(t *testing.T) {
	writer := &normalizedLengthWriter{}
	input := []byte("a b\tc\r\nd")
	n, err := writer.Write(input)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(input) {
		t.Fatalf("Write returned %d, want %d", n, len(input))
	}
	if writer.n != 4 {
		t.Fatalf("normalized length = %d, want 4", writer.n)
	}
}
