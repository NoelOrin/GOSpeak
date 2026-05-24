package main

import (
	"fmt"
	_ "github.com/caarlos0/env/v9"
	"github.com/sirupsen/logrus"
	"go_rtc/cmd"
	"os"
)

// @title           GoRTC API
// @version         1.0
// @description     GoRTC - WebRTC Server API
// @termsOfService  https://github.com/NoelOrin/GoRTC

// @contact.name   NoelOrin
// @contact.url    https://github.com/NoelOrin

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8098
// @BasePath  /api/v1

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Type "Bearer " followed by your JWT token

func main() {
	// 初始化日志
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	if err := cmd.RootCmd.Execute(); err != nil {
		_, err := fmt.Fprintln(os.Stderr, err)
		if err != nil {
			return
		}
		os.Exit(1)
	}
}
