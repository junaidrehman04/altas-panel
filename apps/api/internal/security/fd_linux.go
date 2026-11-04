//go:build linux

package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func confirmInside(f *os.File, root string) error {
	target, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", f.Fd()))
	if err != nil {
		return ErrEscape
	}
	target = strings.TrimSuffix(target, " (deleted)")
	if !filepath.IsAbs(target) || !within(root, target) {
		return ErrEscape
	}
	return nil
}
