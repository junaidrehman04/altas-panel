//go:build linux

package metrics

import "syscall"

func statPath(path string) (total, used, avail uint64, err error) {
	var st syscall.Statfs_t
	if err = syscall.Statfs(path, &st); err != nil {
		return 0, 0, 0, err
	}
	if st.Bsize <= 0 {
		return 0, 0, 0, syscall.EINVAL
	}
	bsize := uint64(st.Bsize)
	total = st.Blocks * bsize
	free := st.Bfree * bsize
	avail = st.Bavail * bsize
	if total >= free {
		used = total - free
	}
	return total, used, avail, nil
}
