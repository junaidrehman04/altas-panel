package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchLatestUsesReleaseAPINotAssetURL(t *testing.T) {
	bin := fakeELF("official")
	sum := sha256.Sum256(bin)
	digest := hex.EncodeToString(sum[:])
	sums := digest + "  altas-panel-linux-amd64\n"
	var sawEvil bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/junaidrehman04/altas-panel/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v0.3.0",
				"assets": []map[string]any{
					{"id": 10, "name": "altas-panel-linux-amd64", "size": len(bin), "browser_download_url": "https://evil.example/bin"},
					{"id": 11, "name": "SHA256SUMS", "size": len(sums), "browser_download_url": "https://evil.example/sums"},
					{"id": 12, "name": "altas-panel_0.3.0_src.tar.gz", "size": 10, "browser_download_url": "https://evil.example/src"},
				},
			})
		case "/repos/junaidrehman04/altas-panel/releases/assets/10",
			"/repos/junaidrehman04/altas-panel/releases/assets/11":
			if r.Header.Get("Accept") != "application/octet-stream" {
				http.Error(w, "accept", http.StatusNotAcceptable)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/10") {
				_, _ = w.Write(bin)
				return
			}
			_, _ = w.Write([]byte(sums))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client := &Client{
		BaseURL: srv.URL,
		Owner:   "junaidrehman04",
		Repo:    "altas-panel",
		OS:      "linux",
		Arch:    "amd64",
		HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "evil.example" || strings.Contains(req.URL.String(), "browser_download") {
				sawEvil = true
				return nil, errors.New("blocked")
			}
			return http.DefaultTransport.RoundTrip(req)
		})},
	}
	got, err := client.FetchLatest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sawEvil {
		t.Fatal("client requested a non-API asset URL")
	}
	if got.Tag != "v0.3.0" || got.Name != "altas-panel-linux-amd64" {
		t.Fatalf("release = %+v", got)
	}
	if !bytesEqual(got.Binary, bin) || string(got.Checksums) != sums {
		t.Fatal("payload mismatch")
	}
}

func TestFetchLatestRefusesOffHostRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.example/releases/latest", http.StatusFound)
	}))
	t.Cleanup(srv.Close)

	client := &Client{
		BaseURL: srv.URL,
		Owner:   "junaidrehman04",
		Repo:    "altas-panel",
		OS:      "linux",
		Arch:    "amd64",
		HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Hostname() == "evil.example" {
				t.Errorf("followed redirect to %s", req.URL)
				return nil, errors.New("blocked")
			}
			return http.DefaultTransport.RoundTrip(req)
		})},
	}
	_, err := client.FetchLatest(context.Background())
	if err == nil {
		t.Fatal("expected redirect refusal")
	}
}

func TestFetchLatestMissingRelease(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	client := &Client{
		BaseURL: srv.URL,
		Owner:   "junaidrehman04",
		Repo:    "altas-panel",
		OS:      "linux",
		Arch:    "amd64",
	}
	_, err := client.FetchLatest(context.Background())
	if !errors.Is(err, ErrNoRelease) {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchLatestRejectsRepoEscape(t *testing.T) {
	client := &Client{BaseURL: "https://api.github.com", Owner: "../admin", Repo: "altas-panel", OS: "linux", Arch: "amd64"}
	if _, err := client.FetchLatest(context.Background()); err == nil {
		t.Fatal("expected invalid owner to fail before a request")
	}
}

func TestNewerRelease(t *testing.T) {
	if !Newer("v0.3.0", "v0.2.0-dev") {
		t.Fatal("release should replace the dev build")
	}
	if Newer("v0.2.0-dev", "v0.2.0") {
		t.Fatal("prerelease must not replace the release")
	}
	if Newer("v0.2.0", "v0.2.0") {
		t.Fatal("same version is not newer")
	}
	if Newer("not-a-version", "v0.2.0") {
		t.Fatal("unparsed latest must not apply")
	}
}

func TestEd25519Signature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	data := fakeELF("signed")
	sig := ed25519.Sign(priv, data)
	if err := VerifyEd25519(pub, data, []byte(hex.EncodeToString(sig))); err != nil {
		t.Fatal(err)
	}
	other, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyEd25519(other, data, []byte(hex.EncodeToString(sig))); !errors.Is(err, ErrSignature) {
		t.Fatalf("wrong key err = %v", err)
	}
	parsed, err := ParsePublicKey(hex.EncodeToString(pub))
	if err != nil || !bytesEqual(parsed, pub) {
		t.Fatalf("parse key err=%v", err)
	}
	if _, err := ParsePublicKey("abcd"); err == nil {
		t.Fatal("short key accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func bytesEqual(a, b []byte) bool { return string(a) == string(b) }

func TestChecksumBodyNotExecuted(t *testing.T) {
	// The updater hashes bytes. It must not treat a checksum file as a program.
	script := []byte("#!/bin/sh\necho pwned\n")
	sum := sha256.Sum256(script)
	if err := VerifySHA256(script, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	if IsInstallableBinary(script) {
		t.Fatal("shell script was accepted as an installable binary")
	}
	if _, err := io.ReadAll(strings.NewReader(string(script))); err != nil {
		t.Fatal(err)
	}
}
