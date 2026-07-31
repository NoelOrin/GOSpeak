package cmd

import (
	"fmt"

	"GOSpeak/internal/config"
	"GOSpeak/internal/repository"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var resetPasswordCmd = &cobra.Command{
	Use:   "reset-password [username] [new-password]",
	Short: "reset user password",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		username := args[0]
		newPassword := args[1]

		env, _ := cmd.Flags().GetString("env")

		config.LoadEnvFiles(env)
		cfg, err := config.Load()
		if err != nil {
			panic(fmt.Sprintf("load config: %v", err))
		}

		if err := repository.InitDB(cfg); err != nil {
			panic(fmt.Sprintf("init db: %v", err))
		}

		userRepo := repository.NewUserRepository(repository.DB)
		user, err := userRepo.GetByName(username)
		if err != nil {
			panic(fmt.Sprintf("user %q not found: %v", username, err))
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			panic(fmt.Sprintf("hash password: %v", err))
		}

		user.Password = string(hashed)
		if err := userRepo.UpdatePasswordAndInvalidate(user); err != nil {
			panic(fmt.Sprintf("update password: %v", err))
		}

		fmt.Printf("password reset for %q, all tokens invalidated\n", username)
	},
}

func init() {
	resetPasswordCmd.Flags().StringP("env", "e", "", "environment (dev)")
	RootCmd.AddCommand(resetPasswordCmd)
}
