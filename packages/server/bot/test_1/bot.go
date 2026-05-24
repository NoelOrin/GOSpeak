package main

import (
	"fmt"
	"go_rtc/internal/livekit"
)

func main() {
	svc := livekit.NewService()
	token, err := svc.GenerateToken("test-room", "test-bot")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Token:", token)
}