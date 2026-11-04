package security

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	maxArgv     = 32
	maxTokenLen = 256
	maxNameLen  = 64
)

// Shells, interpreters, and wrappers that turn an argument into code are
// never allowlisted. Stack operations have to name a real binary such as docker.
var deniedCommands = map[string]struct{}{
	"sh": {}, "bash": {}, "dash": {}, "ash": {}, "zsh": {}, "ksh": {}, "csh": {}, "tcsh": {}, "fish": {},
	"busybox": {}, "env": {}, "sudo": {}, "su": {}, "nohup": {}, "xargs": {}, "nice": {}, "ionice": {},
	"perl": {}, "python": {}, "python2": {}, "python3": {}, "ruby": {}, "php": {}, "node": {}, "deno": {}, "bun": {},
	"nc": {}, "ncat": {}, "netcat": {}, "socat": {},
	"awk": {}, "sed": {}, "find": {},
}

// Allowlist checks argv before it is passed to os/exec.
// Arguments are separate tokens. They are never joined into a shell string.
type Allowlist struct {
	allowed map[string]struct{}
}

// NewAllowlist records the only command names that Validate will accept.
// Names are basenames, not paths. Shells and interpreters are refused.
func NewAllowlist(commands ...string) (*Allowlist, error) {
	allowed := make(map[string]struct{}, len(commands))
	for _, name := range commands {
		if !validCommandName(name) {
			return nil, ErrCommand
		}
		if _, denied := deniedCommands[name]; denied {
			return nil, ErrCommand
		}
		allowed[name] = struct{}{}
	}
	return &Allowlist{allowed: allowed}, nil
}

// Validate checks argv without executing it.
// argv[0] must be an allowlisted basename. Every element must be a plain
// token: no shell metacharacters, no NUL, and no ".." path segment.
func (a *Allowlist) Validate(argv []string) error {
	if a == nil || len(argv) == 0 || len(argv) > maxArgv {
		return ErrCommand
	}
	if _, ok := a.allowed[argv[0]]; !ok || !validCommandName(argv[0]) {
		return ErrCommand
	}
	if _, denied := deniedCommands[argv[0]]; denied {
		return ErrCommand
	}
	for _, arg := range argv[1:] {
		if !validToken(arg) {
			return ErrCommand
		}
	}
	return nil
}

// Command returns an os/exec command for a validated argv.
// The binary is looked up by basename on PATH. A relative lookup result is
// refused. The command is not run through a shell.
func (a *Allowlist) Command(ctx context.Context, argv ...string) (*exec.Cmd, error) {
	if err := a.Validate(argv); err != nil {
		return nil, err
	}
	looked, err := exec.LookPath(argv[0])
	if err != nil || !filepath.IsAbs(looked) {
		return nil, ErrCommand
	}
	// exec.CommandContext uses argv directly. It does not invoke sh -c.
	cmd := exec.CommandContext(ctx, looked, argv[1:]...)
	cmd.Args[0] = looked
	return cmd, nil
}

func validCommandName(name string) bool {
	if name == "" || len(name) > maxNameLen {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '+' || r == '-':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func validToken(token string) bool {
	if token == "" || len(token) > maxTokenLen {
		return false
	}
	for _, r := range token {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '+' || r == '-' || r == ':' || r == '@' || r == ',' || r == '/' || r == '%' || r == '=':
		default:
			return false
		}
	}
	for _, part := range strings.Split(token, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}
