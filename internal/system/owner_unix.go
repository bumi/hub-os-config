//go:build unix

package system

import (
	"os"
	"syscall"
)

// fileOwner returns the uid/gid owning path, and whether they were obtained.
func fileOwner(path string) (uid, gid int, ok bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, 0, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return int(st.Uid), int(st.Gid), true
}
