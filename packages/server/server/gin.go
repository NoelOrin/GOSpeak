package server

import (
	"fmt"
	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
	"github.com/joho/godotenv"
	_ "go_rtc/server/db"
	"go_rtc/server/router"
	"go_rtc/server/router/socket"
	"io"
	"log"
	"net/http"
	"os"
)

type EnvEnum string

const (
	Dev  EnvEnum = "dev"
	Prod EnvEnum = "prod"
)

func loadingEnv(env EnvEnum) {
	switch env {
	case Dev:
		err := godotenv.Load("./.env.dev")
		if err != nil {
			panic(err)
		}
	case Prod:
		err := godotenv.Load("./.env.prod")
		if err != nil {
			panic(err)
		}
	}
}

func StartGin(env EnvEnum) {
	fmt.Println(env)
	// 加载环境变量
	loadingEnv(env)
	// 关闭debug
	if env == Prod || env == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	// 创建 Socket.IO 服务器
	server := socketio.NewServer(nil)

	// 引入所有 Socket 路由
	socket.SetupSocketRoutes(server)
	// 挂载到 Gin 路由
	r.GET("/socket.io/*any", gin.WrapH(server))
	r.POST("/socket.io/*any", gin.WrapH(server))
	r.OPTIONS("/socket.io/*any", gin.WrapH(server))

	//
	router.IndexRoute(r)
	//
	router.UserRoute(r)

	// 启动 HTTP 服务
	err := r.Run(":8098")
	if err != nil {
		println(err)
		return
	}
}

func init() {
	logFile, err := os.OpenFile("./server/logs/xxx.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("open log file failed, err:", err)
		return
	}
	log.SetOutput(logFile)
	log.SetFlags(log.Llongfile | log.Lmicroseconds | log.Ldate)

	// gin log
	// 禁用控制台颜色，将日志写入文件时不需要控制台颜色。
	gin.DisableConsoleColor()

	// 记录到文件。
	f, _ := os.Create("gin.log")
	//gin.DefaultWriter = io.MultiWriter(f)

	// 如果需要同时将日志写入文件和控制台，请使用以下代码。
	gin.DefaultWriter = io.MultiWriter(f, os.Stdout)

}
