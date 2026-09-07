package main

import "io/fs"

// ownerOf reports nothing on Windows: there are no POSIX uid/gid bits to read,
// and the permission problem this exists to diagnose is a Linux container one.
func ownerOf(fs.FileInfo) string { return "" }
