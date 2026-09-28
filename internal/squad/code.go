// Package squad is MAYAK's squad room: players who share a squad code see
// each other's last position on the squad map. The members meet in a room
// of the "mayak-relay" Worker (relay/worker/index.js) over a WebSocket.
//
// The code never leaves the PC. The room is a hash of it (RoomID) and every
// message is sealed with a key derived from it (AES-256-GCM), so the relay
// forwards strings it cannot read.
package squad

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// alphabet is Crockford's base 32: no I, L, O or U, which read as other
// characters.
const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// CodeLength is the number of characters in a squad code (40 bits).
const CodeLength = 8

var ErrInvalidCode = errors.New("invalid squad code")

// NewCode returns a new random squad code (without the dash).
func NewCode() string {
	b := make([]byte, CodeLength)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// Normalize turns a code as typed ("abcd-1234", "ABCD 1234", with O for 0
// or I/L for 1) into its canonical form, or fails when it is not a code.
func Normalize(s string) (string, error) {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch r {
		case '-', ' ', '\t':
			continue
		case 'O':
			r = '0'
		case 'I', 'L':
			r = '1'
		}
		if !strings.ContainsRune(alphabet, r) {
			return "", ErrInvalidCode
		}
		b.WriteRune(r)
	}
	if b.Len() != CodeLength {
		return "", ErrInvalidCode
	}
	return b.String(), nil
}

// Format shows a canonical code in two groups ("ABCD-1234").
func Format(code string) string {
	if len(code) != CodeLength {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// RoomID is the relay's name for the code's room: a hash, so the relay
// does not learn the code the key comes from.
func RoomID(code string) string {
	sum := sha256.Sum256([]byte("mayak-squad-room\x00" + code))
	return hex.EncodeToString(sum[:])
}
