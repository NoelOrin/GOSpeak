package cmd

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"GOSpeak/server"
)

var (
	envFlag string
)

var RootCmd = &cobra.Command{
	Use:   "gospeak",
	Short: "GoRTC - WebRTC server",
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "start server",
	Run: func(cmd *cobra.Command, args []string) {
		_ = godotenv.Load()
		env, _ := cmd.Flags().GetString("env")
		if env != "" {
			server.StartGin(server.EnvEnum(env))
		} else {
			server.StartGin("prod")
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("v0.1.0")
	},
}

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	serverCmd.Flags().StringP("env", "e", "", "specify environment (dev, prod)")
	RootCmd.AddCommand(serverCmd)
	RootCmd.AddCommand(versionCmd)
}