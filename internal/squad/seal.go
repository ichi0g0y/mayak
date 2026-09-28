package squad

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

// sealer seals and opens the room's messages with the code's key.
type sealer struct{ aead cipher.AEAD }

func newSealer(code string) (sealer, error) {
	key, err := hkdf.Key(sha256.New, []byte(code), nil, "mayak-squad-key v1", 32)
	if err != nil {
		return sealer{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return sealer{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return sealer{}, err
	}
	return sealer{aead}, nil
}

// seal returns base64url(nonce || ciphertext).
func (s sealer) seal(plain []byte) string {
	nonce := make([]byte, s.aead.NonceSize(), s.aead.NonceSize()+len(plain)+s.aead.Overhead())
	_, _ = rand.Read(nonce)
	return base64.RawURLEncoding.EncodeToString(s.aead.Seal(nonce, nonce, plain, nil))
}

var errSealed = errors.New("message does not open with this squad's key")

func (s sealer) open(text string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil || len(raw) < s.aead.NonceSize() {
		return nil, errSealed
	}
	plain, err := s.aead.Open(nil, raw[:s.aead.NonceSize()], raw[s.aead.NonceSize():], nil)
	if err != nil {
		return nil, errSealed
	}
	return plain, nil
}
