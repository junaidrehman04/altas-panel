package security

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenJailRejectsUnusableRoots(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []string{"", "/", file, filepath.Join(t.TempDir(), "missing")}
	for _, root := range cases {
		if _, err := OpenJail(root); !errors.Is(err, ErrJail) {
			t.Fatalf("OpenJail(%q) err = %v, want ErrJail", root, err)
		}
	}
}

func TestResolveRejectsTraversal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	jail, err := OpenJail(root)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(root)
	outside := filepath.Join(parent, "outside")
	attacks := []string{
		"..",
		"../etc/passwd",
		"foo/../../etc/passwd",
		"foo/../bar",
		"/etc/passwd",
		filepath.Join(parent, "outside"),
		outside,
		"foo\x00bar",
		"foo\nbar",
		"",
		"%2e%2e/%2e%2e/etc/passwd",
		"%2E%2E/secret",
		"..%2f..",
		"sub/%2e%2e/secret",
	}
	for _, name := range attacks {
		got, err := jail.Resolve(name)
		if err == nil {
			t.Fatalf("Resolve(%q) = %s, want escape", name, got)
		}
		if !errors.Is(err, ErrEscape) {
			t.Fatalf("Resolve(%q) err = %v, want ErrEscape", name, err)
		}
	}
}

func TestResolveAllowsPathsInsideJail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "proj"), 0o755); err != nil {
		t.Fatal(err)
	}
	jail, err := OpenJail(root)
	if err != nil {
		t.Fatal(err)
	}

	got, err := jail.Resolve(".")
	if err != nil || got != jail.Root() {
		t.Fatalf("Resolve(.) = %s, %v", got, err)
	}
	got, err = jail.Resolve("proj")
	if err != nil || got != filepath.Join(jail.Root(), "proj") {
		t.Fatalf("Resolve(proj) = %s, %v", got, err)
	}
	got, err = jail.Resolve("projects/app/compose.yaml")
	want := filepath.Join(jail.Root(), "projects", "app", "compose.yaml")
	if err != nil || got != want {
		t.Fatalf("Resolve(new file) = %s, %v; want %s", got, err, want)
	}
	got, err = jail.Resolve(filepath.Join(jail.Root(), "proj"))
	if err != nil || got != filepath.Join(jail.Root(), "proj") {
		t.Fatalf("Resolve(absolute inside) = %s, %v", got, err)
	}
}

func TestSiblingPrefixDoesNotEscape(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "jail")
	evil := filepath.Join(parent, "jail-evil")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(evil, "file"), 0o755); err != nil {
		t.Fatal(err)
	}
	jail, err := OpenJail(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := jail.Resolve(filepath.Join(evil, "file")); err == nil {
		t.Fatalf("sibling prefix escaped to %s", got)
	}
}

func TestSymlinkEscapeRejected(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "sub")); err != nil {
		t.Fatal(err)
	}
	jail, err := OpenJail(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"escape", "sub", "sub/secret"} {
		if got, err := jail.Resolve(name); err == nil {
			t.Fatalf("Resolve(%s) escaped to %s", name, got)
		}
		if _, err := jail.Open(name); err == nil {
			t.Fatalf("Open(%s) followed a symlink out of the jail", name)
		}
	}
}

func TestSymlinkInsideAndOpenStayInJail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	real := filepath.Join(root, "data.txt")
	if err := os.WriteFile(real, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	jail, err := OpenJail(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jail.Resolve("alias")
	want, errEval := filepath.EvalSymlinks(real)
	if errEval != nil {
		t.Fatal(errEval)
	}
	if err != nil || got != want {
		t.Fatalf("Resolve(alias) = %s, %v; want %s", got, err, want)
	}
	f, err := jail.Open("alias")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	buf := make([]byte, 8)
	n, err := f.Read(buf)
	if err != nil || string(buf[:n]) != "ok" {
		t.Fatalf("read %q, %v", buf[:n], err)
	}
}
