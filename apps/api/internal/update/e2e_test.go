package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestInstalledProcessAutoUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("set a full test run to build and restart a panel")
	}
	if runtime.GOOS != "linux" {
		t.Skip("installed auto-update is exercised on linux")
	}
	root := moduleRoot(t)
	built := t.TempDir()
	oldBin := filepath.Join(built, "v030")
	newBin := filepath.Join(built, "v031")
	buildServer(t, root, oldBin, "v0.3.0")
	buildServer(t, root, newBin, "v0.3.1")

	t.Run("bad checksum", func(t *testing.T) {
		work := installCopy(t, oldBin)
		payload := bytes.Clone(mustRead(t, newBin))
		payload[len(payload)-1] ^= 0xff
		sum := sha256.Sum256(mustRead(t, newBin))
		srv := githubRelease(t, "v0.3.1", payload, hex.EncodeToString(sum[:]))
		proc := startPanel(t, work, srv.URL, false)
		defer proc.stop()
		client := proc.client(t)
		registerAdmin(t, client, proc.base, "bad@example.com")
		enableAuto(t, client, proc.base)
		status, body := postJSON(t, client, proc.base+"/api/settings/updates/check", map[string]any{})
		if status != http.StatusConflict || !bytes.Contains(body, []byte("checksum")) {
			t.Fatalf("status=%d body=%s", status, body)
		}
		if !bytes.Equal(mustRead(t, work), mustRead(t, oldBin)) {
			t.Fatal("bad checksum replaced the binary")
		}
		if _, err := os.Stat(work + ".bak"); err == nil {
			t.Fatal("bad checksum wrote a backup")
		}
		if got := versionOf(t, proc.base); got != "v0.3.0" {
			t.Fatalf("version = %s", got)
		}
	})

	t.Run("applies and restarts", func(t *testing.T) {
		work := installCopy(t, oldBin)
		next := mustRead(t, newBin)
		sum := sha256.Sum256(next)
		srv := githubRelease(t, "v0.3.1", next, hex.EncodeToString(sum[:]))
		proc := startPanel(t, work, srv.URL, false)
		defer proc.stop()
		client := proc.client(t)
		registerAdmin(t, client, proc.base, "good@example.com")
		enableAuto(t, client, proc.base)
		postJSON(t, client, proc.base+"/api/settings/updates/check", map[string]any{})
		deadline := time.Now().Add(25 * time.Second)
		var got string
		for time.Now().Before(deadline) {
			got = versionOf(t, proc.base)
			if got == "v0.3.1" {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if got != "v0.3.1" {
			t.Fatalf("version after restart = %s\n%s", got, proc.logs())
		}
		if !bytes.Equal(mustRead(t, work), next) {
			t.Fatal("installed bytes are not the new release")
		}
		if _, err := os.Stat(work + ".bak"); err != nil {
			t.Fatal("backup missing after a successful swap")
		}
	})

	t.Run("failed start rolls back", func(t *testing.T) {
		work := installCopy(t, oldBin)
		bad := buildExit(t)
		sum := sha256.Sum256(bad)
		srv := githubRelease(t, "v0.3.2", bad, hex.EncodeToString(sum[:]))
		proc := startPanel(t, work, srv.URL, true)
		defer proc.stop()
		client := proc.client(t)
		registerAdmin(t, client, proc.base, "rollback@example.com")
		enableAuto(t, client, proc.base)
		postJSON(t, client, proc.base+"/api/settings/updates/check", map[string]any{})
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if bytes.Equal(mustRead(t, work), mustRead(t, oldBin)) && fileExists(work+".bak") {
				if versionOf(t, proc.base) == "v0.3.0" {
					return
				}
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatalf("rollback did not restore the running binary\n%s", proc.logs())
	})
}

type panelProc struct {
	cmd       *exec.Cmd
	base      string
	pidFile   string
	helperPID string
	serverLog string
	helperLog string
	sysLog    string
}

func (p *panelProc) stop() {
	if p == nil {
		return
	}
	killFile(p.pidFile)
	killFile(p.helperPID)
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_, _ = p.cmd.Process.Wait()
	}
}

func (p *panelProc) logs() string {
	return "server:\n" + readTrim(p.serverLog) + "\nhelper:\n" + readTrim(p.helperLog) + "\nsystemd:\n" + readTrim(p.sysLog)
}

func (p *panelProc) client(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar, Timeout: 20 * time.Second}
}

func startPanel(t *testing.T, bin, releaseURL string, failStart bool) *panelProc {
	t.Helper()
	port := freePort(t)
	dir := filepath.Dir(bin)
	pidFile := filepath.Join(dir, "panel.pid")
	helperPID := filepath.Join(dir, "helper.pid")
	serverLog := filepath.Join(dir, "server.log")
	helperLog := filepath.Join(dir, "helper.log")
	sysLog := filepath.Join(dir, "systemctl.log")
	fake := filepath.Join(dir, "fakebin")
	if err := os.MkdirAll(fake, 0o755); err != nil {
		t.Fatal(err)
	}
	fail := "0"
	if failStart {
		fail = "1"
	}
	writeExe(t, filepath.Join(fake, "systemd-run"), fakeSystemdRun)
	writeExe(t, filepath.Join(fake, "systemctl"), fakeSystemctl)
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"ALTAS_ENV=development",
		"ALTAS_ADDR=127.0.0.1:"+port,
		"ALTAS_DB="+filepath.Join(dir, "altas.db"),
		"ALTAS_UPDATE_API_BASE="+releaseURL,
		"ALTAS_UPDATE_BINARY="+bin,
		"ALTAS_UPDATE_UNIT=altas-panel",
		"ALTAS_UPDATE_RESTART=systemd",
		"ALTAS_RESTART_DELAY=200ms",
		"PATH="+fake+string(os.PathListSeparator)+os.Getenv("PATH"),
		"E2E_FAIL_START="+fail,
		"E2E_BIN="+bin,
		"E2E_PIDFILE="+pidFile,
		"E2E_HELPER_PID="+helperPID,
		"E2E_HELPER_LOG="+helperLog,
		"E2E_SERVER_LOG="+serverLog,
		"E2E_SYSLOG="+sysLog,
		"E2E_HEALTH=http://127.0.0.1:"+port+"/api/health",
	)
	logFile, err := os.Create(serverLog)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644); err != nil {
		t.Fatal(err)
	}
	proc := &panelProc{
		cmd: cmd, base: "http://127.0.0.1:" + port, pidFile: pidFile, helperPID: helperPID,
		serverLog: serverLog, helperLog: helperLog, sysLog: sysLog,
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if versionOf(t, proc.base) == "v0.3.0" {
			return proc
		}
		time.Sleep(100 * time.Millisecond)
	}
	proc.stop()
	t.Fatalf("panel did not start\n%s", proc.logs())
	return nil
}

func registerAdmin(t *testing.T, client *http.Client, base, email string) {
	t.Helper()
	status, body := postJSON(t, client, base+"/api/auth/register", map[string]string{
		"email": email, "password": "correct-horse",
	})
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("register %d %s", status, body)
	}
}

func enableAuto(t *testing.T, client *http.Client, base string) {
	t.Helper()
	status, body := sendJSON(t, client, http.MethodPatch, base+"/api/settings", map[string]any{"auto_update": true})
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"auto_update":true`)) && !bytes.Contains(body, []byte(`"auto_update": true`)) {
		t.Fatalf("enable %d %s", status, body)
	}
}

func versionOf(t *testing.T, base string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/version", nil)
	if err != nil {
		return ""
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ""
	}
	return payload.Version
}

func postJSON(t *testing.T, client *http.Client, url string, body any) (int, []byte) {
	t.Helper()
	return sendJSON(t, client, http.MethodPost, url, body)
}

func sendJSON(t *testing.T, client *http.Client, method, url string, body any) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return 0, []byte(err.Error())
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, data
}

func githubRelease(t *testing.T, tag string, bin []byte, digest string) *httptest.Server {
	t.Helper()
	name := "altas-panel-linux-" + runtime.GOARCH
	sums := digest + "  " + name + "\n"
	files := map[string][]byte{"10": bin, "11": []byte(sums)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/junaidrehman04/altas-panel/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": tag,
				"assets": []map[string]any{
					{"id": 10, "name": name, "size": len(bin)},
					{"id": 11, "name": "SHA256SUMS", "size": len(sums)},
				},
			})
		default:
			id := strings.TrimPrefix(r.URL.Path, "/repos/junaidrehman04/altas-panel/releases/assets/")
			body, ok := files[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func buildServer(t *testing.T, root, out, version string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath",
		"-ldflags", "-s -w -X github.com/junaidrehman04/altas/apps/api/internal/version.Version="+version,
		"-o", out, "./apps/api/cmd/server")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", version, err, output)
	}
}

func buildExit(t *testing.T) []byte {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte("package main\nimport \"os\"\nfunc main() { os.Exit(1) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "exit")
	cmd := exec.Command("go", "build", "-o", out, src)
	cmd.Env = append(os.Environ(), "GO111MODULE=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		mod := exec.Command("go", "mod", "init", "exitbin")
		mod.Dir = dir
		if out, err := mod.CombinedOutput(); err != nil {
			t.Fatalf("mod init: %v\n%s", err, out)
		}
		cmd = exec.Command("go", "build", "-o", out, ".")
		cmd.Dir = dir
		if output, err = cmd.CombinedOutput(); err != nil {
			t.Fatalf("build exit: %v\n%s", err, output)
		}
	}
	return mustRead(t, out)
}

func installCopy(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, "altas-panel")
	body := mustRead(t, src)
	if err := os.WriteFile(dst, body, 0o755); err != nil {
		t.Fatal(err)
	}
	return dst
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return fmt.Sprintf("%d", ln.Addr().(*net.TCPAddr).Port)
}

func writeExe(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

func readTrim(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	if len(body) > 4000 {
		return string(body[len(body)-4000:])
	}
	return string(body)
}

func killFile(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(raw)), "%d", &pid); err != nil || pid <= 1 {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Kill()
}

const fakeSystemdRun = `#!/bin/sh
set -eu
while [ $# -gt 0 ]; do
  case "$1" in
    --collect|--quiet) shift ;;
    --unit=*) shift ;;
    --*) echo "unexpected $1" >&2; exit 1 ;;
    *) break ;;
  esac
done
"$@" >"${E2E_HELPER_LOG:-/tmp/altas-helper.log}" 2>&1 &
echo $! >"${E2E_HELPER_PID:-/tmp/altas-helper.pid}"
exit 0
`

const fakeSystemctl = `#!/bin/sh
set -eu
echo "$*" >>"${E2E_SYSLOG:-/tmp/altas-systemctl.log}"
cmd=$1
shift || true
if [ "$cmd" = "restart" ]; then
  if [ "${E2E_FAIL_START:-0}" = "1" ]; then
    exit 1
  fi
  if [ -n "${E2E_PIDFILE:-}" ] && [ -f "$E2E_PIDFILE" ]; then
    old=$(cat "$E2E_PIDFILE" || true)
    if [ -n "$old" ]; then
      kill "$old" 2>/dev/null || true
      i=0
      while [ "$i" -lt 50 ]; do
        if ! kill -0 "$old" 2>/dev/null; then
          break
        fi
        i=$((i + 1))
        sleep 0.1
      done
    fi
  fi
  "$E2E_BIN" >>"${E2E_SERVER_LOG:-/tmp/altas-server.log}" 2>&1 &
  echo $! >"$E2E_PIDFILE"
  i=0
  while [ "$i" -lt 50 ]; do
    if curl -fsS "${E2E_HEALTH}" >/dev/null 2>&1; then
      exit 0
    fi
    i=$((i + 1))
    sleep 0.1
  done
  exit 1
fi
if [ "$cmd" = "is-active" ]; then
  if curl -fsS "${E2E_HEALTH}" >/dev/null 2>&1; then
    exit 0
  fi
  exit 1
fi
exit 1
`
