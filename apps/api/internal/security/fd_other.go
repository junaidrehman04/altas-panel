//go:build !linux

package security

import "os"

func confirmInside(_ *os.File, _ string) error {
	return nil
}
