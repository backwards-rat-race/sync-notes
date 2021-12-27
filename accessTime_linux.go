package main

import (
	"os"
	"syscall"
	"time"
)

func accessTime(info os.FileInfo) time.Time {
	atime := info.Sys().(*syscall.Stat_t).Atim
	return time.Unix(atime.Sec, atime.Nsec)
}
