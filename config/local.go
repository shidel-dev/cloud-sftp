package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/bcrypt"

	"github.com/shidel-dev/cloud-sftp/server"
)

type local struct {
	path string
}

func (l *local) ServerConfig(defaultConfig server.Config) (*server.Config, error) {

	c, err := l.readConfigFile()
	if err != nil {
		return nil, err
	}

	return &server.Config{
		Port:             defaultConfig.Port,
		BindAddr:         defaultConfig.BindAddr,
		HostKey:          defaultConfig.HostKey,
		StorageURL:       c.StorageURL,
		PasswordCallback: passwordCallback(c),
	}, nil
}

func (l *local) AddUser(username string, password string, publicKey []byte) error {
	c, err := l.readConfigFile()
	if err != nil {
		return err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	c.Users = append(c.Users, UserConfig{
		UserName:     username,
		PasswordHash: string(passwordHash),
	})

	return l.writeConfigFile(c)
}

func (l *local) readConfigFile() (*ServerConfig, error) {
	jsonFile, err := os.Open(l.path)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	jsonBytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	var c ServerConfig

	err = json.Unmarshal(jsonBytes, &c)
	if err != nil {
		return nil, errors.New("Failed to parse config file")
	}

	return &c, nil
}

func (l *local) writeConfigFile(c *ServerConfig) error {
	jsonFile, err := os.Open(l.path)
	if err != nil {
		return err
	}
	defer jsonFile.Close()
	info, err := jsonFile.Stat()

	if err != nil {
		return err
	}

	d, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(l.path, d, info.Mode())
}
