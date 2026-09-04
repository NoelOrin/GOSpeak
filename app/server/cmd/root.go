package cmd

import (
	"fmt"
	"os"

	"GOSpeak/internal/logger"
	"GOSpeak/server"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "gospeak",
	Short: "GoRTC - WebRTC server",
}

// version 默认 dev，构建时可通过 ldflags 注入。
var version = "dev"

// commit 默认 unknown，构建时可通过 ldflags 注入。
var commit = "unknown"

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
		fmt.Println(version)
	},
}

func init() {
	RootCmd.AddCommand(newServerCommand())
	RootCmd.AddCommand(versionCmd)
}
