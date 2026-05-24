package cursor_test

import (
	"strings"
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/pkg/cursor"
)

func newCodec(t *testing.T) *cursor.Codec {
	t.Helper()
	c, err := cursor.New([]byte("test-secret-must-be-at-least-32-bytes-long!"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestCodec_RoundTrip(t *testing.T) {
	c := newCodec(t)
	want := cursor.Payload{
		ID: "01HX9YZK3F4M5N6P7Q8R9STUVW",
		TS: time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC),
	}

	encoded, err := c.Encode(want)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if encoded == "" {
		t.Fatal("Encode produced empty string")
	}

	got, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID mismatch: got %q want %q", got.ID, want.ID)
	}
	if !got.TS.Equal(want.TS) {
		t.Errorf("TS mismatch: got %v want %v", got.TS, want.TS)
	}
}

func TestCodec_DecodeRejectsTamperedPayload(t *testing.T) {
	c := newCodec(t)
	original, err := c.Encode(cursor.Payload{ID: "abc", TS: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	// Flip a character in the middle of the body, not the signature suffix.
	tampered := flipMiddleChar(original)
	if tampered == original {
		t.Fatal("test setup: tampered cursor identical to original")
	}

	if _, err := c.Decode(tampered); err == nil {
		t.Fatal("expected tampered cursor to fail signature verification")
	}
}

func TestCodec_DecodeRejectsDifferentSecret(t *testing.T) {
	a, _ := cursor.New([]byte("secret-A-must-be-at-least-32-bytes-long-aaa"))
	b, _ := cursor.New([]byte("secret-B-must-be-at-least-32-bytes-long-bbb"))

	encoded, err := a.Encode(cursor.Payload{ID: "abc", TS: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Decode(encoded); err == nil {
		t.Fatal("decoding with wrong secret should fail")
	}
}

func TestCodec_DecodeRejectsGarbage(t *testing.T) {
	c := newCodec(t)
	cases := []string{
		"",
		"not-base64-!!!",
		"YWJjZGVm", // valid base64, invalid structure
		strings.Repeat("a", 4096),
	}
	for _, in := range cases {
		if _, err := c.Decode(in); err == nil {
			t.Errorf("Decode(%q) expected error, got nil", in)
		}
	}
}

func TestNew_RejectsShortSecret(t *testing.T) {
	if _, err := cursor.New([]byte("too-short")); err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestCodec_OutputIsURLSafe(t *testing.T) {
	c := newCodec(t)
	out, err := c.Encode(cursor.Payload{ID: "id-with/special+chars=", TS: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range out {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			t.Errorf("non-URL-safe char %q in cursor %q", r, out)
		}
	}
}

func flipMiddleChar(s string) string {
	if len(s) < 4 {
		return s
	}
	b := []byte(s)
	i := len(b) / 2
	// pick a different char in URL-safe base64 alphabet
	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}
	return string(b)
}
