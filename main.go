package main

import (
	"github.com/shidel-dev/sftp-sass/cmd"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/memblob"
)

func main() {
	cmd.Execute()
}
