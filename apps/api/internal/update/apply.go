package update

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrNotBinary = errors.New("not a binary")
	ErrNoBackup  = errors.New("no backup")
	ErrPath      = errors.New("binary path")
)

// Install replaces path with data and keeps the previous bytes at path.bak.
// data must already be a binary image. The function writes the file and
// does not execute it.
func Install(path string, data []byte) error {
	if !IsInstallableBinary(data) {
		return ErrNotBinary
	}
	path, err := cleanBinaryPath(path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		if err := copyAtomically(path, path+".bak"); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeAtomically(path, data)
}

// RestoreBackup copies path.bak back onto path. The backup file is kept.
func RestoreBackup(path string) error {
	path, err := cleanBinaryPath(path)
	if err != nil {
		return err
	}
	bak := path + ".bak"
	info, err := os.Lstat(bak)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return ErrNoBackup
	}
	data, err := os.ReadFile(bak)
	if err != nil {
		return err
	}
	if !IsInstallableBinary(data) {
		return ErrNotBinary
	}
	return writeAtomically(path, data)
}

// HasBackup reports whether a previous binary is available to restore.
func HasBackup(path string) bool {
	path, err := cleanBinaryPath(path)
	if err != nil {
		return false
	}
	info, err := os.Lstat(path + ".bak")
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func cleanBinaryPath(path string) (string, error) {
	if path == "" || strings.ContainsRune(path, 0) || !filepath.IsAbs(path) {
		return "", ErrPath
	}
	if filepath.Clean(path) != path {
		return "", ErrPath
	}
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
			return "", ErrPath
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 {
		return "", ErrPath
	}
	return path, nil
}

func writeAtomically(path string, data []byte) error {
	next := path + ".next"
	file, err := os.OpenFile(next, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(next)
		if writeErr != nil {
			return writeErr
		}
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	}
	if err := os.Rename(next, path); err != nil {
		_ = os.Remove(next)
		return err
	}
	return nil
}

func copyAtomically(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ErrPath
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		if copyErr != nil {
			return copyErr
		}
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
