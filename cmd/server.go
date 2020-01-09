package cmd

import (
	"io/ioutil"

	"github.com/shidel-dev/sftp-sass/config"
	"github.com/shidel-dev/sftp-sass/server"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var serverPort int
var serverConfigSource string
var serverPrivateKey string

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.PersistentFlags().IntVarP(&serverPort, "port", "p", 22, "ssh/sftp port")
	serverCmd.PersistentFlags().StringVarP(&serverConfigSource, "config-source", "c", "cloud-sftp-config.json", "file path or a blob url https://gocloud.dev/concepts/urls/")
	serverCmd.PersistentFlags().StringVarP(&serverPrivateKey, "private-key", "k", "", "path to private key")
	serverCmd.MarkFlagRequired("private-key")
	serverCmd.MarkFlagFilename("private-key")
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a cloud-sftp server",
	Run: func(cmd *cobra.Command, args []string) {
		log.SetLevel(log.DebugLevel)
		privateBytes, err := ioutil.ReadFile(serverPrivateKey)
		if err != nil {
			log.Fatal("Failed to load private key", err)
		}

		private, err := ssh.ParsePrivateKey(privateBytes)
		if err != nil {
			log.Fatal("Failed to parse private key", err)
		}

		configProvider, err := config.ParseConfigSource(serverConfigSource)
		if err != nil {
			log.Fatal(err)
		}

		configDefaults := server.Config{
			HostKey:  private,
			BindAddr: "0.0.0.0",
			Port:     serverPort,
		}

		serverConfig, err := configProvider.ServerConfig(configDefaults)

		if err != nil {
			log.Fatal(err)
		}

		server := server.NewServer(serverConfig)
		err = server.ListenAndServe(nil)
		if err != nil {
			log.Fatal("Failed to start server", err)
		}
	},
}
