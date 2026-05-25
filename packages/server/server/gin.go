package server

import (
	"fmt"
	"go_rtc/internal/handler"
	"go_rtc/internal/livekit"
	"go_rtc/internal/repository"
	"go_rtc/internal/router"
	"go_rtc/internal/service"
	"go_rtc/internal/signal"
	"io"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
)

type EnvEnum string

const (
	Dev  EnvEnum = "dev"
	Prod EnvEnum = "prod"
)

func StartGin(env EnvEnum) {
	loadingEnv(env)

	if env == Prod || env == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	if err := repository.InitDB(); err != nil {
		panic(fmt.Sprintf("failed to initialize database: %v", err))
	}

	liveKitSvc := livekit.NewService()

	userRepo := repository.NewUserRepository(repository.DB)
	roomRepo := repository.NewRoomRepository(repository.DB)

	authSvc := service.NewAuthService(userRepo)
	userSvc := service.NewUserService(userRepo)
	_ = service.NewRoomService(roomRepo)

	authH := handler.NewAuthHandler(authSvc)
	userH := handler.NewUserHandler(userSvc)
	signalH := handler.NewSignalHandler(liveKitSvc)

	r := gin.Default()

	sioServer := socketio.NewServer(nil)
	signalHub := signal.NewHub()
	signalHub.SetupRoutes(sioServer)

	r.GET("/socket.io/*any", gin.WrapH(sioServer))
	r.POST("/socket.io/*any", gin.WrapH(sioServer))
	r.OPTIONS("/socket.io/*any", gin.WrapH(sioServer))

	router.SetupRoutes(r, &router.Handlers{
		Auth:   authH,
		User:   userH,
		Signal: signalH,
	})

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8998"
	}
	if err := r.Run(":" + port); err != nil {
		panic(err)
	}
}

func loadingEnv(env EnvEnum) {
	switch env {
	case Dev:
		if err := loadEnvFile("./.env.dev"); err != nil {
			panic(err)
		}
	case Prod:
		if err := loadEnvFile("./.env.prod"); err != nil {
			panic(err)
		}
	}
}

func loadEnvFile(path string) error {
	// user can choose to use dotenv or export env vars manually
	_ = path
	return nil
}

func init() {
	logFile, err := os.OpenFile("./server/logs/xxx.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("open log file failed, err:", err)
		return
	}
	log.SetOutput(logFile)
	log.SetFlags(log.Llongfile | log.Lmicroseconds | log.Ldate)

	gin.DisableConsoleColor()
	f, _ := os.Create("gin.log")
	gin.DefaultWriter = io.MultiWriter(f, os.Stdout)
}
