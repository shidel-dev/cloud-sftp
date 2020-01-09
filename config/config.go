package config

import (
	"fmt"
	"strings"

	"github.com/shidel-dev/sftp-sass/server"
)

var validGoCloudURLPrefixes = []string{
	"s3://",
	"gs://",
	"azblob://",
}

var validFileExtensions = []string{
	"json",
}

//Provider provides a server.Config
type Provider interface {
	ServerConfig(defaultConfig server.Config) (*server.Config, error)
	AddUser(username string, password string, publicKeyString []byte) error
}

//ServerConfig specfies how to connect to blob storage, and specfies users and their permissions
type ServerConfig struct {
	Users     []UserConfig `json:"users"`
	DriverURL string       `json:"driver_url"`
}

//UserConfig specfies a user and their permissions
type UserConfig struct {
	UserName     string `json:"username"`
	PasswordHash string `json:"password_hash"`
}

//ParseConfigSource takes a gocloud url, or file path, and returns a Provider
func ParseConfigSource(configSource string) (Provider, error) {
	valid := false
	for _, ext := range validFileExtensions {
		match := strings.HasSuffix(configSource, "."+ext)
		if match {
			valid = true
			break
		}
	}

	if !valid {
		return nil, fmt.Errorf("%v does not have a valid file extension", configSource)
	}

	isGoCloudURL := false
	for _, prefix := range validGoCloudURLPrefixes {
		match := strings.HasPrefix(configSource, prefix)
		if match {
			isGoCloudURL = true
			break
		}
	}

	if isGoCloudURL {
		// return &remote{
		// 	url: configSource,
		// }, nil
	}

	return &local{
		path: configSource,
	}, nil
}
