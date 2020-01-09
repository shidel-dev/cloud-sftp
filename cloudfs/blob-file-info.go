package cloudfs

import (
	"os"
	"path"
	"time"
)

type blobFileInfo struct {
	key     string
	modTime time.Time
	size    int64
	md5     []byte
	isDir   bool
}

func (info *blobFileInfo) Name() string {
	return path.Base(info.key)
}

func (info *blobFileInfo) Size() int64 {
	return info.size
}

func (info *blobFileInfo) Mode() os.FileMode {
	ret := os.FileMode(0644)
	if info.isDir {
		ret = os.FileMode(0755) | os.ModeDir
	}

	return ret
}

func (info *blobFileInfo) ModTime() time.Time {
	return info.modTime
}

func (info *blobFileInfo) IsDir() bool {
	return info.isDir
}

func (info *blobFileInfo) Sys() interface{} {
	return nil
}
