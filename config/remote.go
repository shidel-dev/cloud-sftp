package config

import "github.com/shidel-dev/sftp-sass/server"

type remote struct {
	url string
}

func (l *remote) ServerConfig(defaultConfig server.Config) (*server.Config, error) {
	return nil, nil
}
