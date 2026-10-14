package auth

import (
	"errors"
	"strings"
	"testing"
)

// One fob is one subject however the UID was written down, and the subject
// says nothing about the UID without the key (docs/adr/0025).
func TestFobSubject(t *testing.T) {
	f := &Fobs{key: []byte("0123456789abcdef0123456789abcdef")}

	want, err := f.subject("04:A3:1B:22:5C:80:01")
	if err != nil {
		t.Fatal(err)
	}
	for _, uid := range []string{"04a31b225c8001", "04 A3 1B 22 5C 80 01", "04-a3-1b-22-5c-80-01"} {
		got, err := f.subject(uid)
		if err != nil {
			t.Fatalf("subject(%q): %v", uid, err)
		}
		if got != want {
			t.Errorf("subject(%q) = %s, want %s like the other spellings", uid, got, want)
		}
	}
	if strings.Contains(want, "04a31b") {
		t.Errorf("subject %s contains the UID", want)
	}

	other := &Fobs{key: []byte("another key, another application!")}
	if got, _ := other.subject("04a31b225c8001"); got == want {
		t.Error("two keys produced the same subject; the key is what makes the hash local")
	}
}

func TestFobSubjectRefusesWhatNoReaderSends(t *testing.T) {
	f := &Fobs{key: []byte("0123456789abcdef0123456789abcdef")}

	for _, uid := range []string{"", "04", "04:A3:1B", "04:A3:1B:22:5C", "zz:zz:zz:zz", "04a31b2"} {
		if _, err := f.subject(uid); !errors.Is(err, ErrBadUID) {
			t.Errorf("subject(%q) = %v, want ErrBadUID", uid, err)
		}
	}
}
