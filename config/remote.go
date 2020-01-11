package config

import "github.com/shidel-dev/cloud-sftp/server"

type remote struct {
	url string
}

func (l *remote) ServerConfig(defaultConfig server.Config) (*server.Config, error) {
	return nil, nil
}
