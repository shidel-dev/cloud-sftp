package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
	"github.com/shidel-dev/cloud-sftp/config"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"

	"github.com/pkg/sftp"
	"github.com/shidel-dev/cloud-sftp/server"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

func TestE2EFile(t *testing.T) {

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal("Failed to get working dir", err)
	}

	tmpDir := path.Join(wd, "/tmp/sftp-test")

	_ = os.RemoveAll(tmpDir)
	err = os.MkdirAll(tmpDir, 0700)
	if err != nil {
		t.Fatal("Could not create sftp-test dir")
	}

	c := config.ServerConfig{
		StorageURL: fmt.Sprintf("file://%v", tmpDir),
	}
	d, err := json.Marshal(&c)
	if err != nil {
		t.Fatal("Failed to encode ServerConfig as json")
	}
	err = ioutil.WriteFile("tmp/test-config.json", d, 0700)
	if err != nil {
		t.Fatalf("Failed to write config %v", err)
	}
	defer os.Remove("tmp/test-config.json")
	provider, err := config.ParseConfigSource("tmp/test-config.json")
	if err != nil {
		t.Fatalf("Failed to ParseConfigSource %v", err)
	}

	err = provider.AddUser("testuser", "securetestpassword", []byte{})
	if err != nil {
		t.Fatalf("Failed to add user %v", err)
	}

	privateBytes, err := ioutil.ReadFile("testdata/id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key", err)
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

	server, cond := startTestServer(config)
	defer server.Close()

	//indicates that the server is ready for requests
	cond.Wait()

	password := "securetestpassword"
	clientConfig := ssh.ClientConfig{
		User:            "testuser",
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", "127.0.0.1", 2022)
	conn, err := ssh.Dial("tcp", addr, &clientConfig)
	if err != nil {
		t.Fatalf("Could not create client ssh.Dial failed %v", err)
	}

	client, err := sftp.NewClient(conn)
	defer client.Close()
	if err != nil {
		t.Fatalf("Creating sftp client failed with %v", err)
	}

	runSharedExamples(t, client)
	fmt.Println("File finished")
}

func TestE2EMinio(t *testing.T) {
	sess, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials("minio", "miniosecret", ""),
		Region:           aws.String("us-west-1"),
		Endpoint:         aws.String("localhost:9000"),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
	})

	if err != nil {
		t.Fatalf("Could not create minio session: %v", err)
	}
	s3serv := s3.New(sess)
	createBucketInput := &s3.CreateBucketInput{
		Bucket: aws.String("test-bucket"),
	}
	_, err = s3serv.CreateBucket(createBucketInput)
	if err != nil {
		t.Fatal("Failed to create bucket ", err)
	}

	defer func() {
		listObjectsInput := &s3.ListObjectsInput{
			Bucket: aws.String("test-bucket"),
		}

		listObjectsOutput, err := s3serv.ListObjects(listObjectsInput)
		for _, obj := range listObjectsOutput.Contents {
			deleteObjectInput := &s3.DeleteObjectInput{
				Bucket: aws.String("test-bucket"),
				Key:    obj.Key,
			}
			_, err := s3serv.DeleteObject(deleteObjectInput)
			if err != nil {
				t.Fatal("Failed to delete object", err)
			}
		}
		deleteBucketInput := &s3.DeleteBucketInput{
			Bucket: aws.String("test-bucket"),
		}

		_, err = s3serv.DeleteBucket(deleteBucketInput)
		if err != nil {
			t.Fatal("Failed to delete bucket", err)
		}
	}()

	privateBytes, err := ioutil.ReadFile("testdata/id_rsa")
	if err != nil {
		t.Fatal("Failed to load private key", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		t.Fatal("Failed to parse private key", err)
	}

	serverConfig := server.Config{
		HostKey: private,
		Port:    2022,
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) error {
			if string(pass) != "securetestpassword" {
				return errors.New("Unexpected password")
			}
			return nil
		},
		BucketCallback: func(c ssh.ConnMetadata) (*blob.Bucket, error) {
			if err != nil {
				return nil, err
			}

			bucket, err := s3blob.OpenBucket(context.Background(), sess, "test-bucket", nil)
			if err != nil {
				return nil, err
			}

			return bucket, nil
		},
	}

	server, cond := startTestServer(&serverConfig)
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
	if err != nil {
		t.Fatalf("Creating sftp client failed with %v", err)
	}
	defer client.Close()
	runSharedExamples(t, client)
	fmt.Println("Minio finished")
}

func startTestServer(c *server.Config) (*server.Server, *sync.Cond) {
	m := &sync.Mutex{}
	m.Lock()
	cond := sync.NewCond(m)

	server := server.New(c)
	go func() {
		err := server.ListenAndServe(cond)
		if err != nil {
			log.Fatal("Failed to start server", err)
		}
	}()

	return server, cond
}

func runSharedExamples(t *testing.T, client *sftp.Client) {
	f, err := client.Create("hello.txt")
	if err != nil {
		t.Fatalf("Creating hello.txt failed with %v", err)
	}

	if _, err := f.Write([]byte("Hello world!")); err != nil {
		t.Fatalf("Writing data to hello.txt failed with %v", err)
	}

	f.Close()

	stat, err := client.Lstat("hello.txt")
	if err != nil {
		t.Fatal("Failed to stat hello.txt file")
	}
	file, err := client.Open("hello.txt")
	if err != nil {
		t.Fatalf("Failed to open hello.txt %v", err)
	}

	b := make([]byte, stat.Size())
	_, err = file.Read(b)
	if err != nil {
		t.Fatalf("Failed to read file %v", err)
	}

	if string(b) != "Hello world!" {
		t.Fatalf("Expected file to eq 'Hello world!' not %v", string(b))
	}

	largeUUIDString := ""
	for i := 1; i <= 10000; i++ {
		largeUUIDString = largeUUIDString + uuid.New().String()
	}

	_, err = writeStrToRemoteFile(client, "large_file.txt", largeUUIDString)
	if err != nil {
		t.Fatalf("Failed to write large_file.txt err: %v", err)
	}

	stat, err = client.Lstat("large_file.txt")
	if err != nil {
		t.Fatal("Failed to stat large_file.txt file")
	}

	str, err := readStrFromRemoteFile(client, "large_file.txt")
	if err != nil {
		t.Fatalf("Failed to read large_file.txt err: %v", err)
	}
	if str != largeUUIDString {
		t.Fatal("Expected contents from reading large_file.txt to equal largeUUIDString")
	}
	if err = client.Remove("large_file.txt"); err != nil {
		t.Fatalf("Failed to remove large_file.txt err: %v", err)
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

	err = client.Mkdir("my_dir")
	if err != nil {
		t.Fatalf("Failed to create dir %v", err)
	}

	info, err := client.Stat("my_dir")
	if err != nil {
		t.Fatalf("Failed to stat dir %v", err)
	}

	if info.Name() != "my_dir" {
		t.Fatal("Expected dir name to equal my_dir")
	}

	err = client.RemoveDirectory("my_dir")
	if err != nil {
		t.Fatalf("Failed to remove dir %v", err)
	}

	info, err = client.Stat("my_dir")
	if err == nil {
		fmt.Println(info.Name())
		t.Fatal("Expected stat to error for deleted directory")
	}

	err = client.Remove("hello_miss_president.txt")
	if err != nil {
		t.Fatalf("Failed to remove file %v", err)
	}

	_, err = client.Stat("hello_miss_president.txt")
	if err == nil {
		t.Fatalf("Expected there to be an err when stating a deleted file")
	}
}

func writeStrToRemoteFile(client *sftp.Client, remoteFileName string, contents string) (int64, error) {
	f, err := client.Create(remoteFileName)
	defer f.Close()
	if err != nil {
		return 0, err
	}

	return io.Copy(f, bytes.NewBufferString(contents))
}

func readStrFromRemoteFile(client *sftp.Client, remoteFileName string) (string, error) {
	f, err := client.Open(remoteFileName)
	defer f.Close()
	if err != nil {
		return "", err
	}
	fstat, err := f.Stat()
	if err != nil {
		return "", err
	}

	buf := make([]byte, fstat.Size())
	_, err = f.Read(buf)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}
