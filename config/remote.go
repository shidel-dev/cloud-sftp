package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/shidel-dev/cloud-sftp/server"
	"gocloud.dev/blob"
	"golang.org/x/crypto/bcrypt"
)

type remote struct {
	key    string
	bucket *blob.Bucket
}

func newRemoteConfigProvider(configSource string) (*remote, error) {
	u, err := url.Parse(configSource)
	if err != nil {
		return nil, err
	}

	bucketURL := fmt.Sprintf("%v://%v?%v", u.Scheme, u.Host, u.RawQuery)
	bucket, err := blob.OpenBucket(context.Background(), bucketURL)
	if err != nil {
		return nil, err
	}

	return &remote{
		bucket: bucket,
		key:    u.Path,
	}, nil
}

func (r *remote) ServerConfig(defaultConfig server.Config) (*server.Config, error) {
	c, err := r.readConfigFile()
	if err != nil {
		return nil, err
	}

	return &server.Config{
		Port:             defaultConfig.Port,
		BindAddr:         defaultConfig.BindAddr,
		HostKey:          defaultConfig.HostKey,
		DriverURL:        c.DriverURL,
		PasswordCallback: passwordCallback(c),
	}, nil
}

func (r *remote) AddUser(username string, password string, publicKey []byte) error {
	c, err := r.readConfigFile()
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

	return r.writeConfigFile(c)
}

func (r *remote) readConfigFile() (*ServerConfig, error) {
	reader, err := r.bucket.NewReader(context.Background(), r.key, nil)
	if err != nil {
		return nil, err
	}

	defer reader.Close()

	p := make([]byte, 0)
	buf := bytes.NewBuffer(p)

	_, err = io.Copy(buf, reader)
	if err != nil {
		return nil, err
	}

	c := &ServerConfig{}
	err = json.Unmarshal(p, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *remote) writeConfigFile(c *ServerConfig) error {
	d, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	writer, err := r.bucket.NewWriter(context.Background(), r.key, nil)
	if err != nil {
		return err
	}
	defer writer.Close()

	return r.bucket.WriteAll(context.Background(), r.key, d, nil)
}
