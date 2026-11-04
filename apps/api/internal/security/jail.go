package security

import (
	"os"
	"path/filepath"
	"strings"
)

// Jail is a directory that file operations must not leave.
// The root is stored as an absolute path with symlinks resolved.
type Jail struct {
	root string
}

// OpenJail prepares a jail rooted at an existing directory.
// The filesystem root is refused so a misconfigured path cannot unlock the host.
func OpenJail(root string) (*Jail, error) {
	if root == "" || strings.ContainsRune(root, 0) || strings.ContainsAny(root, "\r\n") {
		return nil, ErrJail
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, ErrJail
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, ErrJail
	}
	resolved = filepath.Clean(resolved)
	if resolved == string(os.PathSeparator) {
		return nil, ErrJail
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return nil, ErrJail
	}
	return &Jail{root: resolved}, nil
}

// Root returns the resolved jail directory.
func (j *Jail) Root() string {
	return j.root
}

// Resolve returns the absolute path of name inside the jail.
// name may be relative to the jail root. An absolute name is accepted only
// when it is already inside the root. ".." segments, NUL, newlines, and
// percent-encoding are rejected before the path is cleaned. Symlinks are
// resolved a component at a time and must stay under the root. A missing
// tail is allowed so a caller can create a file that does not exist yet;
// the lexical remainder still has to stay inside the jail.
func (j *Jail) Resolve(name string) (string, error) {
	if j == nil || j.root == "" {
		return "", ErrJail
	}
	if !cleanInput(name) || hasDotDot(name) {
		return "", ErrEscape
	}

	var full string
	if filepath.IsAbs(name) {
		full = filepath.Clean(name)
	} else {
		full = filepath.Clean(filepath.Join(j.root, name))
	}
	if !within(j.root, full) {
		return "", ErrEscape
	}

	rel, err := filepath.Rel(j.root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", ErrEscape
	}
	if rel == "." {
		return j.root, nil
	}

	current := j.root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." || part == ".." {
			return "", ErrEscape
		}
		next := filepath.Join(current, part)
		info, statErr := os.Lstat(next)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				current = next
				continue
			}
			return "", ErrEscape
		}
		if info.Mode()&os.ModeSymlink != 0 {
			dest, evalErr := filepath.EvalSymlinks(next)
			if evalErr != nil || !within(j.root, dest) {
				return "", ErrEscape
			}
			current = dest
			continue
		}
		if !within(j.root, next) {
			return "", ErrEscape
		}
		current = next
	}
	if !within(j.root, current) {
		return "", ErrEscape
	}
	return current, nil
}

// Open resolves name and opens it for reading. On Linux the opened file
// descriptor is checked again so a symlink swap after Resolve still fails.
func (j *Jail) Open(name string) (*os.File, error) {
	path, err := j.Resolve(name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if err := confirmInside(f, j.root); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func cleanInput(name string) bool {
	if name == "" || strings.ContainsRune(name, 0) || strings.ContainsAny(name, "\r\n") {
		return false
	}
	// Callers pass decoded paths. Percent-encoding is rejected so a later
	// decode cannot turn a stored name into ".." after this check.
	if strings.ContainsRune(name, '%') {
		return false
	}
	return true
}

func hasDotDot(name string) bool {
	normalized := strings.ReplaceAll(name, "\\", "/")
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func within(root, path string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(path, root+sep)
}
