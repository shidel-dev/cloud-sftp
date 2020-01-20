package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shidel-dev/cloud-sftp/server"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"
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
	Users      []UserConfig `json:"users"`
	StorageURL string       `json:"storage_url"`
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
		return newRemoteConfigProvider(configSource)
	}

	return &local{
		path: configSource,
	}, nil
}

func passwordCallback(c *ServerConfig) server.PasswordCallback {
	return func(cm ssh.ConnMetadata, password []byte) error {
		username := cm.User()

		match := false
		for _, u := range c.Users {
			if u.UserName == username {
				if len(u.PasswordHash) != 0 {
					err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), password)
					if err != nil {
						fmt.Println(err)
						return errors.New("incorrect username or password")
					}

					match = true
					break
				}
			}
		}

		if match {
			return nil
		}

		fmt.Println("Username not found")
		return errors.New("incorrect username or Password")
	}
}
