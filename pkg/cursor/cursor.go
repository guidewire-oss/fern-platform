// Package cursor implements opaque, HMAC-signed pagination cursors.
//
// A cursor encodes a (ID, timestamp) pair that the repository uses for
// stable keyset pagination. The on-wire form is URL-safe base64
// without padding so it can travel in query strings without encoding.
package cursor

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	minSecretLen = 32
	sigLen       = 16 // truncated HMAC-SHA256
	tsLen        = 8  // unix nanoseconds, big-endian
	// body layout: [tsLen][id...][sigLen]
	minBodyLen = tsLen + 1 + sigLen
)

var (
	// ErrInvalid is returned when a cursor cannot be parsed or fails
	// signature verification. The two cases are conflated on purpose:
	// callers should not be able to distinguish "malformed" from
	// "tampered" because doing so leaks information.
	ErrInvalid = errors.New("cursor: invalid")
)

// Payload is the data carried by a cursor.
type Payload struct {
	ID string
	TS time.Time
}

// Codec encodes and decodes cursors with a shared secret.
type Codec struct {
	secret []byte
}

// New returns a Codec keyed by secret. The secret must be at least
// 32 bytes; in production wire it to a long-lived random value
// stored alongside other deployment secrets.
func New(secret []byte) (*Codec, error) {
	if len(secret) < minSecretLen {
		return nil, fmt.Errorf("cursor: secret must be at least %d bytes", minSecretLen)
	}
	dup := make([]byte, len(secret))
	copy(dup, secret)
	return &Codec{secret: dup}, nil
}

// Encode produces a URL-safe, signed cursor string.
func (c *Codec) Encode(p Payload) (string, error) {
	if p.ID == "" {
		return "", fmt.Errorf("cursor: empty id")
	}
	body := make([]byte, 0, tsLen+len(p.ID)+sigLen)
	var tsBuf [tsLen]byte
	binary.BigEndian.PutUint64(tsBuf[:], uint64(p.TS.UnixNano()))
	body = append(body, tsBuf[:]...)
	body = append(body, []byte(p.ID)...)
	sig := c.sign(body)
	body = append(body, sig...)
	return base64.RawURLEncoding.EncodeToString(body), nil
}

// Decode validates the signature and returns the embedded payload.
// All failure modes return ErrInvalid.
func (c *Codec) Decode(s string) (Payload, error) {
	if s == "" {
		return Payload{}, ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Payload{}, ErrInvalid
	}
	if len(raw) < minBodyLen {
		return Payload{}, ErrInvalid
	}
	signed := raw[:len(raw)-sigLen]
	got := raw[len(raw)-sigLen:]
	want := c.sign(signed)
	if !hmac.Equal(got, want) {
		return Payload{}, ErrInvalid
	}
	tsNanos := int64(binary.BigEndian.Uint64(signed[:tsLen]))
	id := string(signed[tsLen:])
	return Payload{ID: id, TS: time.Unix(0, tsNanos).UTC()}, nil
}

func (c *Codec) sign(b []byte) []byte {
	m := hmac.New(sha256.New, c.secret)
	m.Write(b)
	full := m.Sum(nil)
	return full[:sigLen]
}
