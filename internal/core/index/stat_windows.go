//go:build windows

package index

// Windows is not supported due to missing filesystem metadata required for Git index compatibility.
//
// Note: I'm unable to find any resonable solution and i don't really care about Windows implementation right now
// so i will just stick to the Unix-style filesystem metadata (device ID, inode, UID, GID, creation time) that
// Windows does not provide.
import (
	"os"
	"time"
)

func getStatTimes(fileInfo os.FileInfo) (ctime time.Time, ctimeNs uint32, dev uint32, ino uint32, uid uint32, gid uint32) {
	_ = Windows_Is_Not_Supported()
	return
}
