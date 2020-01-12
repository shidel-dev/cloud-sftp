package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/pkg/sftp"
	"github.com/shidel-dev/cloud-sftp/cloudfs"
	log "github.com/sirupsen/logrus"
	"gocloud.dev/blob"
	"golang.org/x/crypto/ssh"
)

//Config configuration for a sftp server
type Config struct {
	HostKey           ssh.Signer
	BindAddr          string
	Port              int
	PasswordCallback  PasswordCallback
	PublicKeyCallback PublicKeyCallback
	DriverURLCallback DriverURLCallback
	DriverURL         string
}

//PasswordCallback authenticates a ssh connection by password
type PasswordCallback func(c ssh.ConnMetadata, pass []byte) error

//PublicKeyCallback authenticates a ssh connection given a public key
type PublicKeyCallback func(conn ssh.ConnMetadata, key ssh.PublicKey) error

//DriverURLCallback when not nil allows a driver URL to be supplied
type DriverURLCallback func(conn ssh.ConnMetadata) (string, error)

//Server Creates/Operates a sftp server
type Server struct {
	config     *Config
	sftpServer *sftp.Server
}

//NewServer Creates a new Server
func NewServer(config *Config) *Server {
	return &Server{
		config: config,
	}
}

//ListenAndServe listens on the TCP network address addr, and serves sftp requests based on the provided Config
func (s *Server) ListenAndServe(cond *sync.Cond) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("%v:%v", s.config.BindAddr, s.config.Port))
	if err != nil {
		log.Fatal("failed to listen for connection", err)
	}
	fmt.Printf("Listening on %v\n", listener.Addr())
	if cond != nil {
		cond.Broadcast()
	}
	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Fatal("failed to accept incoming connection", err)
		}

		go s.serve(nConn)
	}
}

func (s *Server) serve(conn net.Conn) {
	defer conn.Close()
	config := &ssh.ServerConfig{}
	var connectionMetadata ssh.ConnMetadata

	config.BannerCallback = func(c ssh.ConnMetadata) string {
		connectionMetadata = c
		return ""
	}

	if s.config.PasswordCallback != nil {
		config.PasswordCallback = func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			return nil, s.config.PasswordCallback(c, pass)
		}
	}

	if s.config.PublicKeyCallback != nil {
		config.PublicKeyCallback = func(c ssh.ConnMetadata, pk ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, s.config.PublicKeyCallback(c, pk)
		}
	}

	config.AddHostKey(s.config.HostKey)
	// Before use, a handshake must be performed on the incoming net.Conn.
	sconn, chans, reqs, err := ssh.NewServerConn(conn, config)

	if err != nil {
		log.Fatal("failed to handshake", err)
	}
	log.Println("login detected:", sconn.User())
	log.Debug("SSH server established")

	// The incoming Request channel must be serviced.
	go ssh.DiscardRequests(reqs)

	// Service the incoming Channel channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of an SFTP session, this is "subsystem"
		// with a payload string of "<length=4>sftp"
		log.Debugf("Incoming channel: %s\n", newChannel.ChannelType())
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			log.Debugf("Unknown channel type: %s\n", newChannel.ChannelType())
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Fatal("could not accept channel.", err)
		}
		log.Debugf("Channel accepted\n")

		// Sessions have out-of-band requests such as "shell",
		// "pty-req" and "env".  Here we handle only the
		// "subsystem" request.
		go func(in <-chan *ssh.Request) {
			for req := range in {
				log.Debugf("Request: %v\n", req.Type)
				ok := false
				switch req.Type {
				case "subsystem":
					log.Debugf("Subsystem: %s\n", req.Payload[4:])
					if string(req.Payload[4:]) == "sftp" {
						ok = true
					}
				}
				log.Debugf(" - accepted: %v\n", ok)
				req.Reply(ok, nil)
			}
		}(requests)

		log.SetLevel(log.DebugLevel)
		taggedLogger := log.WithFields(log.Fields{
			"bucket": "sftp",
			"user":   "testuser",
		})

		driverURL := ""
		if s.config.DriverURLCallback != nil {
			if connectionMetadata == nil {
				log.Error("Missing connectionMetadata")
				return
			}

			driverURL, err = s.config.DriverURLCallback(connectionMetadata)
			if err != nil {
				log.Errorf("DriverURLCallback failed %v", err)
				return
			}
		} else {
			driverURL = s.config.DriverURL
		}

		fmt.Println(driverURL)

		if len(driverURL) == 0 {
			log.Error("Missing DriverURL")
			return
		}

		bucket, err := blob.OpenBucket(context.Background(), driverURL)
		if err != nil {
			log.Errorf("Failed to open bucket %v", err)
			return
		}

		fs := cloudfs.New(bucket, taggedLogger)
		handlers := sftp.Handlers{
			FileGet:  fs,
			FilePut:  fs,
			FileList: fs,
			FileCmd:  fs,
		}
		server := sftp.NewRequestServer(channel, handlers)

		if err := server.Serve(); err == io.EOF {
			bucket.Close()
			server.Close()
			log.Print("sftp client exited session.")
		} else if err != nil {
			log.Errorf("sftp server completed with error: %v", err)
		}
	}
}

//Close stops a running sftp server
func (s *Server) Close() error {
	if s.sftpServer != nil {
		return s.sftpServer.Close()
	}

	return nil
}
