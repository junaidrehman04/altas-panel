package update

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdRestartUsesArgvNotAShell(t *testing.T) {
	var got []string
	systemd := Systemd{
		Unit: "altas-panel",
		lookPath: func(name string) (string, error) {
			if name != "systemctl" {
				t.Fatalf("lookup %s", name)
			}
			return "/bin/systemctl", nil
		},
		run: func(_ context.Context, name string, args ...string) error {
			got = append([]string{name}, args...)
			return nil
		},
	}
	if err := systemd.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, " ") != "/bin/systemctl restart altas-panel" {
		t.Fatalf("argv = %#v", got)
	}

	systemd.Unit = "altas-panel;reboot"
	if err := systemd.Restart(context.Background()); err == nil {
		t.Fatal("metacharacter in unit was accepted")
	}
}

func TestApplyRestartRestoresBackupWhenStartFails(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "altas-panel")
	old := fakeELF("previous")
	next := fakeELF("failed-start")
	if err := os.WriteFile(bin, next, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin+".bak", old, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "systemctl")
	body := []byte("#!/bin/sh\nexit 1\n")
	if err := os.WriteFile(script, body, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("ALTAS_RESTART_DELAY", "0s")
	if code := ApplyRestart([]string{"altas-panel", bin}); code == 0 {
		t.Fatal("failed start was treated as success")
	}
	got, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, old) {
		t.Fatal("backup was not restored after a failed start")
	}
}

func TestApplyRestartKeepsBinaryWhenStartSucceeds(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "altas-panel")
	next := fakeELF("running")
	if err := os.WriteFile(bin, next, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin+".bak", fakeELF("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "systemctl")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("ALTAS_RESTART_DELAY", "0s")
	if code := ApplyRestart([]string{"altas-panel", bin}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	got, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, next) {
		t.Fatal("successful start restored the backup")
	}
}

func TestApplyRestartRejectsOtherUnits(t *testing.T) {
	if code := ApplyRestart([]string{"ssh.service", "/tmp/altas-panel"}); code != 2 {
		t.Fatalf("exit %d", code)
	}
}
