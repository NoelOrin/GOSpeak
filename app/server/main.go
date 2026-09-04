package main

import (
	"fmt"
	"os"

	"GOSpeak/cmd"
	"GOSpeak/internal/logger"
)

// @title           GoRTC API
// @version         1.0
// @description     GoRTC - WebRTC 服务器端 API 接口文档
// @termsOfService  https://github.com/NoelOrin/GoRTC

// @contact.name   NoelOrin
// @contact.url    https://github.com/NoelOrin

// @license.name  MIT 许可证
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8998
// @BasePath  /api/v1

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Type "Bearer " followed by your JWT token

func main() {
	// 仅在显式配置 GOSPEAK_WORKDIR 时切换工作目录，避免改变运行环境默认行为。
	if wd := os.Getenv("GOSPEAK_WORKDIR"); wd != "" {
		if err := os.Chdir(wd); err != nil {
			logger.Warnf("failed to chdir to GOSPEAK_WORKDIR %s: %v", wd, err)
		}
	}

	// 完整配置驱动初始化在 server 启动时完成；此处仅保证入口可用
	_ = logger.Init(logger.Options{Level: "info", Format: "text", Output: "stdout"})

	if err := cmd.RootCmd.Execute(); err != nil {
		_, err := fmt.Fprintln(os.Stderr, err)
		if err != nil {
			return
		}
		os.Exit(1)
	}
}
