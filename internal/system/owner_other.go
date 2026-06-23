//go:build !unix

package system

// fileOwner is a no-op on non-unix platforms.
func fileOwner(path string) (uid, gid int, ok bool) { return 0, 0, false }
