package update

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallKeepsBackupAndRollbackRestoresIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "altas-panel")
	old := fakeELF("previous")
	next := fakeELF("replacement")
	if err := os.WriteFile(path, old, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, next); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, next) {
		t.Fatalf("installed = %q", got)
	}
	if !HasBackup(path) {
		t.Fatal("backup missing")
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bak, old) {
		t.Fatalf("backup = %q", bak)
	}

	if err := RestoreBackup(path); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, old) {
		t.Fatalf("restored = %q", restored)
	}
	if !HasBackup(path) {
		t.Fatal("backup should remain so rollback stays available")
	}
}

func TestInstallRefusesScriptAndUnsafePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "altas-panel")
	if err := os.WriteFile(path, fakeELF("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := append([]byte("#!/bin/sh\n"), []byte("echo no\n")...)
	err := Install(path, script)
	if !errors.Is(err, ErrNotBinary) {
		t.Fatalf("script err = %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.HasPrefix(got, []byte{0x7f, 'E', 'L', 'F'}) {
		t.Fatal("script replaced the binary")
	}
	if HasBackup(path) {
		t.Fatal("refused install created a backup")
	}

	if err := Install("relative-altas", fakeELF("x")); !errors.Is(err, ErrPath) {
		t.Fatalf("relative err = %v", err)
	}

	link := filepath.Join(dir, "linked")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := Install(link, fakeELF("via-link")); !errors.Is(err, ErrPath) {
		t.Fatalf("symlink err = %v", err)
	}
}

func TestRestoreBackupMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "altas-panel")
	if err := os.WriteFile(path, fakeELF("only"), 0o755); err != nil {
		t.Fatal(err)
	}
	if HasBackup(path) {
		t.Fatal("unexpected backup")
	}
	if err := RestoreBackup(path); !errors.Is(err, ErrNoBackup) {
		t.Fatalf("err = %v", err)
	}
}

func fakeELF(label string) []byte {
	return append([]byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0}, []byte(label)...)
}
