package security

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllowlistAcceptsPlainDockerArgs(t *testing.T) {
	t.Parallel()
	list, err := NewAllowlist("docker", "docker-compose")
	if err != nil {
		t.Fatal(err)
	}
	if err := list.Validate([]string{"docker", "ps"}); err != nil {
		t.Fatal(err)
	}
	if err := list.Validate([]string{"docker", "compose", "ls", "--format", "json"}); err != nil {
		t.Fatal(err)
	}
	if err := list.Validate([]string{"docker", "ps", "--filter", "name=web"}); err != nil {
		t.Fatal(err)
	}
}

func TestAllowlistRejectsShellsAndInjection(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"sh", "bash", "dash", "busybox", "env", "python3", "perl", "node", "sudo"} {
		if _, err := NewAllowlist(name); !errors.Is(err, ErrCommand) {
			t.Fatalf("NewAllowlist(%s) err = %v, want ErrCommand", name, err)
		}
	}
	if _, err := NewAllowlist("/bin/docker"); !errors.Is(err, ErrCommand) {
		t.Fatal("path command name was accepted")
	}
	if _, err := NewAllowlist(""); !errors.Is(err, ErrCommand) {
		t.Fatal("empty command name was accepted")
	}

	list, err := NewAllowlist("docker", "true")
	if err != nil {
		t.Fatal(err)
	}
	attacks := [][]string{
		{"docker", "ps;rm -rf /"},
		{"docker", "ps", "&&", "id"},
		{"docker", "$(id)"},
		{"docker", "`id`"},
		{"docker", "a|b"},
		{"docker", "a&b"},
		{"docker", "a>b"},
		{"docker", "a<b"},
		{"docker", "a$b"},
		{"docker", "a\nb"},
		{"docker", "a\rb"},
		{"docker", "a\x00b"},
		{"docker", "a'b"},
		{"docker", `a"b`},
		{"docker", "a\\b"},
		{"docker", "a!b"},
		{"docker", "a*b"},
		{"docker", "a?b"},
		{"docker", "{{.ID}}"},
		{"sh", "-c", "id"},
		{"/bin/sh"},
		{"../../bin/sh"},
		{"docker/../sh"},
		{"bash"},
		{"docker", "foo/../../etc/passwd"},
		{"docker", ""},
		{},
		{"docker", "name with spaces"},
		{"Docker"},
		{"docker", strings.Repeat("a", maxTokenLen+1)},
		append([]string{"docker"}, repeat("ok", maxArgv)...),
	}
	for _, argv := range attacks {
		if err := list.Validate(argv); !errors.Is(err, ErrCommand) {
			t.Fatalf("Validate(%q) err = %v, want ErrCommand", argv, err)
		}
		if _, err := list.Command(context.Background(), argv...); !errors.Is(err, ErrCommand) {
			t.Fatalf("Command(%q) err = %v, want ErrCommand", argv, err)
		}
	}
}

func TestCommandDoesNotUseAShell(t *testing.T) {
	t.Parallel()
	list, err := NewAllowlist("true")
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := list.Command(context.Background(), "true")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(cmd.Path) || filepath.Base(cmd.Path) != "true" {
		t.Fatalf("path = %s", cmd.Path)
	}
	if base := filepath.Base(cmd.Path); base == "sh" || base == "bash" {
		t.Fatalf("command resolved to a shell: %s", cmd.Path)
	}
	marker := filepath.Join(t.TempDir(), "should-not-exist")
	// A shell would interpret the semicolon if Command joined argv. Validate
	// must fail first, and the marker file must not appear.
	if _, err := list.Command(context.Background(), "true", "ok;touch", marker); !errors.Is(err, ErrCommand) {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("injection created %s", marker)
	}
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}

func TestMissingBinaryIsRejected(t *testing.T) {
	t.Parallel()
	list, err := NewAllowlist("definitely-not-a-binary-altas")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := list.Command(context.Background(), "definitely-not-a-binary-altas"); !errors.Is(err, ErrCommand) {
		t.Fatalf("missing binary err = %v", err)
	}
}

func repeat(value string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = value
	}
	return out
}
