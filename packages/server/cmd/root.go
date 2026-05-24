// cmd/root.go
package cmd

import (
	"fmt"
	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go_rtc/server"
	"path/filepath"
)

var (
	// GlobalFlags 全局配置变量
	GlobalFlags struct {
		DataDir          string
		Dev              bool
		LogStd           bool
		ForceAutoMigrate bool
		GitHubBaseURL    string
		EnvNoPrefix      bool
		SkipEnvFlag      bool
	}
)

// RootCmd 主命令
var RootCmd = &cobra.Command{
	Use:   "",
	Short: "My CLI Tool",
	Long:  `A simple CLI tool with config loading, env, and subcommands`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 处理 ~ 路径
		dataDir, _ := homedir.Expand(GlobalFlags.DataDir)
		GlobalFlags.DataDir = dataDir

		// 日志提示
		if GlobalFlags.Dev {
			logrus.SetLevel(logrus.DebugLevel)
		}
		//logrus.Infof("运行模式: %s", "dev" if GlobalFlags.Dev else "normal")
		logrus.Infof("数据目录: %s", GlobalFlags.DataDir)

		// 加载环境变量（带前缀 MYCLI_）
		prefix := "MYCLI_"
		if GlobalFlags.EnvNoPrefix {
			prefix = ""
		}

		if !GlobalFlags.SkipEnvFlag {
			if err := env.ParseWithOptions(&GlobalFlags, env.Options{Prefix: prefix}); err != nil {
				logrus.Fatalf("解析环境变量失败: %v", err)
			}
		}
	},
}

func init() {
	// 设置默认值
	home, _ := homedir.Dir()
	if home == "" {
		home = "/tmp"
	}

	// 定义全局标志
	RootCmd.PersistentFlags().StringVar(&GlobalFlags.DataDir, "data-dir", filepath.Join(home, ".mycli"), "数据目录")
	RootCmd.PersistentFlags().BoolVar(&GlobalFlags.Dev, "dev", false, "开发模式")
	RootCmd.PersistentFlags().BoolVar(&GlobalFlags.LogStd, "log-std", true, "是否输出日志到控制台")
	RootCmd.PersistentFlags().BoolVar(&GlobalFlags.ForceAutoMigrate, "force-auto-migrate", false, "强制自动迁移")
	RootCmd.PersistentFlags().StringVar(&GlobalFlags.GitHubBaseURL, "github-base-url", "https://api.github.com/", "GitHub API 地址")
	RootCmd.PersistentFlags().BoolVar(&GlobalFlags.EnvNoPrefix, "env-no-prefix", false, "环境变量是否无需 MYCLI_ 前缀")
	RootCmd.PersistentFlags().BoolVar(&GlobalFlags.SkipEnvFlag, "skip-env-flag", false, "跳过环境变量加载")

	// 添加子命令
	serverCmd.Flags().StringP("env", "e", "", "指定运行环境 (dev, prod, staging)")
	RootCmd.AddCommand(serverCmd)
	RootCmd.AddCommand(versionCmd)
	RootCmd.AddCommand(configCmd)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "start server",
	Run: func(cmd *cobra.Command, args []string) {
		err := godotenv.Load()
		if err != nil {
			return
		}
		env, _ := cmd.Flags().GetString("env")
		if env != "" {
			server.StartGin(server.EnvEnum(env))
		} else {
			server.StartGin("prod")

		}
	},
}

// versionCmd 子命令 version
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("v0.1.0")
	},
}

// configCmd 子命令：mycli config
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "显示当前配置",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("DataDir: %s\n", GlobalFlags.DataDir)
		fmt.Printf("Dev: %t\n", GlobalFlags.Dev)
		fmt.Printf("LogStd: %t\n", GlobalFlags.LogStd)
		fmt.Printf("GitHubBaseURL: %s\n", GlobalFlags.GitHubBaseURL)
	},
}
