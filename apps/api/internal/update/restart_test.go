package update

import (
	"context"
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
