package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrSignature means the release did not carry a valid Ed25519 signature
// for the configured public key. The binary is not installed.
var ErrSignature = errors.New("signature verification failed")

// ParsePublicKey reads a 32-byte Ed25519 public key encoded as hex.
// An empty string means signature checks are not configured.
func ParsePublicKey(text string) (ed25519.PublicKey, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	raw, err := hex.DecodeString(text)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, ErrSignature
	}
	return ed25519.PublicKey(raw), nil
}

// VerifyEd25519 checks an Ed25519 signature over the raw release bytes.
// encoded is the signature file body: hex or standard base64.
func VerifyEd25519(pub ed25519.PublicKey, data, encoded []byte) error {
	if len(pub) != ed25519.PublicKeySize {
		return ErrSignature
	}
	sig, err := decodeSignature(encoded)
	if err != nil {
		return ErrSignature
	}
	if !ed25519.Verify(pub, data, sig) {
		return ErrSignature
	}
	return nil
}

func decodeSignature(encoded []byte) ([]byte, error) {
	text := strings.TrimSpace(string(encoded))
	if text == "" {
		return nil, errors.New("empty signature")
	}
	if len(text) == ed25519.SignatureSize*2 {
		raw, err := hex.DecodeString(text)
		if err == nil && len(raw) == ed25519.SignatureSize {
			return raw, nil
		}
	}
	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(text)
	}
	if err != nil || len(raw) != ed25519.SignatureSize {
		return nil, errors.New("signature encoding")
	}
	return raw, nil
}
