package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrRestart means the panel process was not restarted.
var ErrRestart = errors.New("restart")

// Restarter restarts the installed panel after a binary replace.
type Restarter interface {
	Restart(ctx context.Context) error
}

type noopRestart struct{}

func (noopRestart) Restart(context.Context) error { return nil }

// Systemd restarts one unit through the systemctl binary.
// The unit name is a single token. The command is not a shell.
// Helper, when set, is a copy of the binary that was running before the
// replace. Restart starts that copy outside this process so systemctl can
// stop the service without deadlocking on itself.
type Systemd struct {
	Unit     string
	Binary   string
	Helper   string
	lookPath func(string) (string, error)
	run      func(context.Context, string, ...string) error
}

// Restart runs the out-of-process helper when one was staged.
// Tests inject run and observe `systemctl restart <unit>` directly.
func (s Systemd) Restart(ctx context.Context) error {
	unit := s.unit()
	if !ValidUnit(unit) || !allowedRestartUnit(unit) {
		return ErrRestart
	}
	if s.run != nil {
		look := s.lookPath
		if look == nil {
			look = exec.LookPath
		}
		bin, err := look("systemctl")
		if err != nil || !filepath.IsAbs(bin) {
			return ErrRestart
		}
		return s.run(ctx, bin, "restart", unit)
	}
	if s.Helper != "" {
		return s.spawnHelper(unit)
	}
	return s.restartSync(ctx, unit)
}

func (s Systemd) unit() string {
	if s.Unit == "" {
		return "altas-panel"
	}
	return s.Unit
}

func (s Systemd) spawnHelper(unit string) error {
	path := s.Binary
	if path == "" {
		var err error
		path, err = os.Executable()
		if err != nil {
			return ErrRestart
		}
	}
	cleaned, err := cleanBinaryPath(path)
	if err != nil || !allowedBinaryBase(cleaned) {
		return ErrRestart
	}
	helper := s.Helper
	info, err := os.Lstat(helper)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return ErrRestart
	}
	// Detach from the request context. A disconnected client must not cancel
	// the helper that systemctl will use to stop this process.
	spawnCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if runPath, err := exec.LookPath("systemd-run"); err == nil && filepath.IsAbs(runPath) {
		name := fmt.Sprintf("altas-panel-restart-%d", os.Getpid())
		cmd := exec.CommandContext(spawnCtx, runPath, "--collect", "--quiet", "--unit="+name, helper, "apply-restart", unit, cleaned)
		cmd.Stdin = nil
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return ErrRestart
		}
		return nil
	}
	cmd := exec.Command(helper, "apply-restart", unit, cleaned)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return ErrRestart
	}
	return nil
}

func (s Systemd) restartSync(ctx context.Context, unit string) error {
	return runSystemctl(ctx, "restart", unit)
}

// ApplyRestart is the detached helper. It restarts the unit, and if that
// start does not stay active it copies binary.bak back and restarts again.
// args are the unit and the absolute binary path.
func ApplyRestart(args []string) int {
	if len(args) != 2 || !allowedRestartUnit(args[0]) {
		return 2
	}
	unit := args[0]
	path, err := cleanBinaryPath(args[1])
	if err != nil || !allowedBinaryBase(path) {
		return 2
	}
	time.Sleep(restartDelay())
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := runSystemctl(ctx, "restart", unit); err != nil || !unitHealthy(ctx, unit) {
		if err := RestoreBackup(path); err != nil {
			return 1
		}
		_ = runSystemctl(ctx, "restart", unit)
		return 1
	}
	return 0
}

func restartDelay() time.Duration {
	raw := strings.TrimSpace(os.Getenv("ALTAS_RESTART_DELAY"))
	if raw == "" {
		return time.Second
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 || d > 10*time.Second {
		return time.Second
	}
	return d
}

func unitHealthy(ctx context.Context, unit string) bool {
	if err := runSystemctl(ctx, "is-active", "--quiet", unit); err != nil {
		return false
	}
	return true
}

func runSystemctl(ctx context.Context, args ...string) error {
	bin, err := exec.LookPath("systemctl")
	if err != nil || !filepath.IsAbs(bin) {
		return ErrRestart
	}
	if len(args) == 0 || !ValidUnit(lastUnit(args)) {
		return ErrRestart
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return ErrRestart
	}
	return nil
}

func lastUnit(args []string) string {
	for i := len(args) - 1; i >= 0; i-- {
		if !strings.HasPrefix(args[i], "-") {
			return args[i]
		}
	}
	return ""
}

// allowedRestartUnit limits the helper to the panel unit so a local user
// cannot aim this binary at an unrelated service.
func allowedRestartUnit(unit string) bool {
	if !ValidUnit(unit) {
		return false
	}
	base := strings.TrimSuffix(unit, ".service")
	return base == "altas-panel"
}

func allowedBinaryBase(path string) bool {
	base := filepath.Base(path)
	return base == "altas-panel" || base == "altas-panel.exe"
}

// ValidUnit accepts a systemd unit name and rejects shell metacharacters.
func ValidUnit(unit string) bool {
	if unit == "" || len(unit) > 80 || strings.Contains(unit, "..") {
		return false
	}
	for i, r := range unit {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.' || r == ':' || r == '@':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// stageHelper copies the running executable beside the binary that is about
// to be replaced. The copy is what ApplyRestart runs.
func stageHelper(binPath string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	info, err := os.Lstat(exe)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return "", ErrRestart
	}
	dst := binPath + ".restart"
	if err := copyAtomically(exe, dst); err != nil {
		return "", err
	}
	return dst, nil
}
