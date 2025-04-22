package main

import (
	"fmt"
	"log"
	"scanner/auth"
	"scanner/config"
	"scanner/routes"
	"scanner/users"

	"github.com/gin-gonic/gin"
)

func main() {

	cfg := config.MustLoad()

	gin.SetMode(cfg.Environment)

	gdefault := gin.Default()

	routes.SetupAllRoutes(gdefault)

	users.SaveUsers([]users.User{
		{
			Username: "main-admin",
			Role:     auth.AdminRole,
			Password: cfg.AdminPassword,
		},
		{
			Username: "main-user",
			Role:     auth.UserRole,
			Password: cfg.UserPassword,
		},
	})

	go auth.CleanupSessions()

	log.Printf("Starting server on port: %d\n", cfg.Port)
	gdefault.Run(fmt.Sprintf(":%d", cfg.Port))
}
