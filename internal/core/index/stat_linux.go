//go:build linux

package index

import (
	"os"
	"syscall"
	"time"
)

func getStatTimes(fileInfo os.FileInfo) (ctime time.Time, ctimeNs uint32, dev uint32, ino uint32, uid uint32, gid uint32) {
	stat := fileInfo.Sys().(*syscall.Stat_t)
	return time.Unix(stat.Ctim.Sec, 0),
		uint32(stat.Ctim.Nsec),
		uint32(stat.Dev),
		uint32(stat.Ino),
		stat.Uid,
		stat.Gid
}
