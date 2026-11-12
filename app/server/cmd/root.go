package cmd

import (
	"fmt"
	"os"

	"GOSpeak/internal/buildinfo"
	"GOSpeak/internal/logger"
	"GOSpeak/server"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "gospeak",
	Short: "GoRTC - WebRTC server",
}

func newServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start server",
		Run: func(cmd *cobra.Command, args []string) {
			env, _ := cmd.Flags().GetString("env")
			appEnv := server.Dev
			if env != "" {
				appEnv = server.EnvEnum(env)
			}
			if err := server.StartGin(appEnv); err != nil {
				logger.WithComponent("Server").Errorf("server exited with error: %v", err)
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringP("env", "e", "", "specify environment (dev, prod)")
	return cmd
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(buildinfo.Version)
	},
}

func init() {
	RootCmd.AddCommand(newServerCommand())
	RootCmd.AddCommand(versionCmd)
}
