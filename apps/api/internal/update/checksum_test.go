package update

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestVerifySHA256AcceptsMatchingDigest(t *testing.T) {
	data := []byte("altas-panel-binary")
	sum := sha256.Sum256(data)
	if err := VerifySHA256(data, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	prefixed := "sha256:" + strings.ToUpper(hex.EncodeToString(sum[:]))
	if err := VerifySHA256(data, prefixed); err != nil {
		t.Fatal(err)
	}
}

func TestVerifySHA256RefusesMismatchAndBadDigest(t *testing.T) {
	data := []byte("altas-panel-binary")
	sum := sha256.Sum256(data)
	sum[0] ^= 0xff
	err := VerifySHA256(data, hex.EncodeToString(sum[:]))
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("mismatch err = %v", err)
	}

	for _, digest := range []string{"", "abcd", "zz", strings.Repeat("a", 63), strings.Repeat("g", 64)} {
		err := VerifySHA256(data, digest)
		if !errors.Is(err, ErrChecksumFormat) {
			t.Fatalf("digest %q err = %v", digest, err)
		}
	}
}

func TestParseChecksumsReadsCoreutilsLines(t *testing.T) {
	text := "" +
		"# release\n" +
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  altas-panel-linux-amd64\n" +
		"abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd *SHA256SUMS\n"
	sums, err := ParseChecksums(text)
	if err != nil {
		t.Fatal(err)
	}
	if sums["altas-panel-linux-amd64"] == "" || sums["SHA256SUMS"] == "" {
		t.Fatalf("sums = %#v", sums)
	}
	if _, err := DigestFor(sums, "missing"); !errors.Is(err, ErrChecksumFormat) {
		t.Fatalf("missing digest err = %v", err)
	}
}

func TestParseChecksumsRejectsUnsafeNamesAndConflicts(t *testing.T) {
	good := strings.Repeat("ab", 32)
	cases := []string{
		good + "  ../altas\n",
		good + "  /tmp/altas\n",
		good + "  altas panel\n",
		good + "  altas\n" + strings.Repeat("cd", 32) + "  altas\n",
		"",
		"# only a comment\n",
	}
	for _, text := range cases {
		if _, err := ParseChecksums(text); !errors.Is(err, ErrChecksumFormat) {
			t.Fatalf("text %q err = %v", text, err)
		}
	}
	if bytes.Contains([]byte(good), []byte(" ")) {
		t.Fatal("fixture hex contained a space")
	}
}
