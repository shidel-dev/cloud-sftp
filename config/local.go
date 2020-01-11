package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"

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
		Port:      defaultConfig.Port,
		BindAddr:  defaultConfig.BindAddr,
		HostKey:   defaultConfig.HostKey,
		DriverURL: c.DriverURL,
		PasswordCallback: func(cm ssh.ConnMetadata, password []byte) error {
			username := cm.User()
			fmt.Println(username)
			fmt.Println(string(password))

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
		},
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
