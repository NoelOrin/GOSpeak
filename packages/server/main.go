// main.go
package main

import (
	"fmt"
	_ "github.com/caarlos0/env/v9"
	"github.com/sirupsen/logrus"
	"go_rtc/cmd"
	"os"
)

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
