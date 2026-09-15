//go:build unix

package main

import (
	"os"
	"syscall"
)

func lockShared(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_SH)
}
