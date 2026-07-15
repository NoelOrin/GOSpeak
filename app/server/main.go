package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"GOSpeak/cmd"
	"os"
	"path/filepath"
	"runtime"
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

func init() {
	_, file, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(file)
		if err := os.Chdir(dir); err != nil {
			logrus.Warnf("failed to chdir to %s: %v", dir, err)
		}
	}
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	if err := cmd.RootCmd.Execute(); err != nil {
		_, err := fmt.Fprintln(os.Stderr, err)
		if err != nil {
			return
		}
		os.Exit(1)
	}
}
