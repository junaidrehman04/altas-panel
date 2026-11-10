//go:build linux

package metrics

import "syscall"

func statPath(path string) (total, used, avail uint64, err error) {
	total, used, avail, _, err = statPathFull(path)
	return total, used, avail, err
}

func statPathFull(path string) (total, used, avail uint64, inodePct *float64, err error) {
	var st syscall.Statfs_t
	if err = syscall.Statfs(path, &st); err != nil {
		return 0, 0, 0, nil, err
	}
	if st.Bsize <= 0 {
		return 0, 0, 0, nil, syscall.EINVAL
	}
	bsize := uint64(st.Bsize)
	total = st.Blocks * bsize
	free := st.Bfree * bsize
	avail = st.Bavail * bsize
	if total >= free {
		used = total - free
	}
	if st.Files > 0 {
		usedInodes := st.Files - st.Ffree
		pct := float64(usedInodes) / float64(st.Files) * 100
		inodePct = &pct
	}
	return total, used, avail, inodePct, nil
}
