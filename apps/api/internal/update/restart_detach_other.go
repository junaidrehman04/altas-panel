//go:build !unix

package update

import "os/exec"

func detach(cmd *exec.Cmd) {}
