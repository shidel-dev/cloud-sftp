package cmd

import (
	"fmt"

	"github.com/shidel-dev/sftp-sass/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var userConfigSource string
var username string
var password string

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.PersistentFlags().StringVarP(&userConfigSource, "config-source", "c", "cloud-sftp-config.json", "file path or a blob url https://gocloud.dev/concepts/urls/")
	userCmd.MarkFlagRequired("config-source")
	userCmd.AddCommand(addUserCmd)

	addUserCmd.Flags().StringVar(&username, "username", "", "")
	addUserCmd.Flags().StringVar(&password, "password", "", "")
	addUserCmd.MarkFlagRequired("password")
	addUserCmd.MarkFlagRequired("username")
}

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage users",
}

var addUserCmd = &cobra.Command{
	Use:   "add",
	Short: "add a user",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(userConfigSource)
		configProvider, err := config.ParseConfigSource(userConfigSource)
		if err != nil {
			log.Fatal(err)
		}

		err = configProvider.AddUser(username, password, []byte{})
		if err != nil {
			log.Fatal(err)
		}
	},
}
