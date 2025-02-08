package main

import (
	"fmt"
	"github.com/dfryer1193/goblog/internal/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	fmt.Println("go blog server")
	service := gin.New()
	service.Use(middleware.LoggingMiddleware())
	service.Use(gin.CustomRecovery(middleware.HandlePanics()))

}
