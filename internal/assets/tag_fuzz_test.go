package assets

import (
	"strings"
	"testing"
	"unicode"
)

// FuzzValidTag: the explicit-tag validator never panics and accepts exactly the
// tags that are 1..64 bytes of visible characters.
func FuzzValidTag(f *testing.F) {
	f.Add("AST-000001")
	f.Add("")
	f.Add("a b")
	f.Add(strings.Repeat("x", 64))
	f.Add(strings.Repeat("x", 65))
	f.Add("tab\there")
	f.Add("\x7f")
	f.Fuzz(func(t *testing.T, tag string) {
		got := ValidTag(tag)
		want := tag != "" && len(tag) <= 64
		if want {
			for _, r := range tag {
				if r <= ' ' || r == 0x7f || (unicode.IsControl(r) && r < 0x80) {
					want = false
					break
				}
			}
		}
		if got != want {
			t.Fatalf("ValidTag(%q)=%v want %v", tag, got, want)
		}
	})
}

// FuzzAutoTag: generated tags are always valid and well-formed.
func FuzzAutoTag(f *testing.F) {
	f.Add(uint8(0))
	f.Fuzz(func(t *testing.T, _ uint8) {
		tag := AutoTag()
		if !ValidTag(tag) || !strings.HasPrefix(tag, "AST-") || len(tag) != 10 {
			t.Fatalf("bad tag %q", tag)
		}
		for _, r := range tag[4:] {
			if !strings.ContainsRune("0123456789ABCDEF", r) {
				t.Fatalf("non-hex %q", tag)
			}
		}
	})
}
