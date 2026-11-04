// Package security holds the path jail and command allowlist used by later
// file-manager and stack operations. Helpers reject escapes and shell
// metacharacters. They do not start a shell.
package security

import "errors"

var (
	// ErrEscape means the path is outside the jail, including symlink escapes.
	ErrEscape = errors.New("path escapes jail")
	// ErrJail means the jail root itself cannot be used.
	ErrJail = errors.New("jail root is not usable")
	// ErrCommand means the argv failed the allowlist or token rules.
	ErrCommand = errors.New("command rejected")
)
