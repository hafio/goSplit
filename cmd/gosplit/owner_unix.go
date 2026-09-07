//go:build !windows

package main

import (
	"fmt"
	"io/fs"
	"syscall"
)

// ownerOf reports a directory's uid:gid, which together with the process's own
// uid is the whole story behind a "permission denied" on a bind mount.
func ownerOf(info fs.FileInfo) string {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d:%d", st.Uid, st.Gid)
}
