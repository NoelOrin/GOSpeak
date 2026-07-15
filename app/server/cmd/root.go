package cmd

import (
	"fmt"

	"GOSpeak/server"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "gospeak",
	Short: "GoRTC - WebRTC server",
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "start server",
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")
		if env != "" {
			server.StartGin(server.EnvEnum(env))
		} else {
			server.StartGin(server.Prod)
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
	serverCmd.Flags().StringP("env", "e", "", "specify environment (dev, prod)")
	RootCmd.AddCommand(serverCmd)
	RootCmd.AddCommand(versionCmd)
}
