package main

import (
	"github.com/shidel-dev/cloud-sftp/cmd"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/memblob"
)

func main() {
	cmd.Execute()
}
