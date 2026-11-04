package update

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	ErrChecksumMismatch = errors.New("checksum mismatch")
	ErrChecksumFormat   = errors.New("checksum format")
)

// VerifySHA256 compares data to a hex digest from the release checksum file.
// A mismatch or a malformed digest is refused. The bytes are not executed.
func VerifySHA256(data []byte, expected string) error {
	want, err := normalizeDigest(expected)
	if err != nil {
		return ErrChecksumFormat
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return ErrChecksumMismatch
	}
	return nil
}

func normalizeDigest(expected string) (string, error) {
	text := strings.ToLower(strings.TrimSpace(expected))
	text = strings.TrimPrefix(text, "sha256:")
	if len(text) != sha256.Size*2 {
		return "", ErrChecksumFormat
	}
	if _, err := hex.DecodeString(text); err != nil {
		return "", ErrChecksumFormat
	}
	return text, nil
}

// ParseChecksums reads coreutils lines (`digest  name` or `digest *name`).
// Names that could escape the release asset list are rejected.
func ParseChecksums(text string) (map[string]string, error) {
	sums := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			return nil, ErrChecksumFormat
		}
		name := strings.TrimPrefix(parts[1], "*")
		if !safeAssetName(name) {
			return nil, ErrChecksumFormat
		}
		digest, err := normalizeDigest(parts[0])
		if err != nil {
			return nil, ErrChecksumFormat
		}
		if prev, ok := sums[name]; ok && prev != digest {
			return nil, ErrChecksumFormat
		}
		sums[name] = digest
	}
	if len(sums) == 0 {
		return nil, ErrChecksumFormat
	}
	return sums, nil
}

// DigestFor returns the checksum recorded for an asset name.
func DigestFor(sums map[string]string, name string) (string, error) {
	digest, ok := sums[name]
	if !ok || digest == "" {
		return "", ErrChecksumFormat
	}
	return digest, nil
}

func safeAssetName(name string) bool {
	if name == "" || len(name) > 128 || strings.Contains(name, "..") {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-' || r == '+':
		default:
			return false
		}
	}
	return true
}
