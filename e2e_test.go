package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/shidel-dev/sftp-sass/config"

	"github.com/pkg/sftp"
	"github.com/shidel-dev/sftp-sass/server"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

func TestE2E(t *testing.T) {
	_ = os.RemoveAll("/Users/joeshidel/sftp-test")
	err := os.Mkdir("/Users/joeshidel/sftp-test", 0700)
	if err != nil {
		t.Fatal("Could not create sftp-test dir")
	}

	os.Remove("test-config.json")
	c := config.ServerConfig{
		DriverURL: "file:///Users/joeshidel/sftp-test",
	}
	d, err := json.Marshal(&c)
	if err != nil {
		t.Fatal("Failed to encode ServerConfig as json")
	}
	ioutil.WriteFile("test-config.json", d, 0700)

	provider, err := config.ParseConfigSource("test-config.json")
	if err != nil {
		t.Fatalf("Failed to ParseConfigSource %v", err)
	}

	err = provider.AddUser("testuser", "securetestpassword", []byte{})
	if err != nil {
		t.Fatalf("Failed to add user %v", err)
	}

	server, cond := startTestServer()
	defer server.Close()

	//indicates that the server is ready for requests
	cond.Wait()

	password := "securetestpassword"
	config := ssh.ClientConfig{
		User:            "testuser",
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", "127.0.0.1", 2022)
	conn, err := ssh.Dial("tcp", addr, &config)
	if err != nil {
		t.Fatalf("Could not create client ssh.Dial failed %v", err)
	}

	client, err := sftp.NewClient(conn)
	defer client.Close()
	if err != nil {
		t.Fatalf("Creating sftp client failed with %v", err)
	}

	f, err := client.Create("hello.txt")
	if err != nil {
		t.Fatalf("Creating hello.txt failed with %v", err)
	}

	if _, err := f.Write([]byte("Hello world!")); err != nil {
		t.Fatalf("Writing data to hello.txt failed with %v", err)
	}

	f.Close()

	_, err = client.Lstat("hello.txt")
	if err != nil {
		time.Sleep(100 * time.Second)
		t.Fatal("Failed to stat hello.txt file")
	}
	file, err := client.Open("hello.txt")
	if err != nil {
		t.Fatalf("Failed to open hello.txt %v", err)
	}

	buf := bytes.NewBufferString("")
	_, err = io.Copy(buf, file)
	if err != nil {
		t.Fatalf("Failed to copy file %v", err)
	}

	if buf.String() != "Hello world!" {
		t.Fatalf("Expected file to eq 'Hello world!' no %v", buf.String())
	}

	err = client.Rename("hello.txt", "hello_miss_president.txt")
	if err != nil {
		t.Fatalf("Failed to rename file %v", err)
	}

	list, err := client.ReadDir("/")

	if err != nil {
		t.Fatalf("Listing root dir failed %v", err)
	}

	found := false
	for _, l := range list {
		fmt.Println(l.Name())
		if l.Name() == "hello_miss_president.txt" {
			found = true
		}
	}

	if found == false {
		t.Fatalf("failed to find file after renaming %v", err)
	}

	if len(list) != 1 {
		t.Fatal("Too many files for dir when listing")
	}
}

func startTestServer() (*server.Server, *sync.Cond) {
	m := &sync.Mutex{}
	m.Lock()
	cond := sync.NewCond(m)
	privateBytes, err := ioutil.ReadFile("id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key", err)
	}

	provider, err := config.ParseConfigSource("test-config.json")
	if err != nil {
		log.Fatal("Failed to ParseConfigSource", err)
	}
	defaultConfig := server.Config{
		HostKey:  private,
		BindAddr: "0.0.0.0",
		Port:     2022,
	}
	config, err := provider.ServerConfig(defaultConfig)
	if err != nil {
		log.Fatal("Failed to load ServerConfig", err)
	}

	server := server.NewServer(config)
	go func() {
		err = server.ListenAndServe(cond)
		if err != nil {
			log.Fatal("Failed to start server", err)
		}
	}()

	return server, cond
}
