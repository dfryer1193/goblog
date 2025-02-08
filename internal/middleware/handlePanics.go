package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func HandlePanics() gin.RecoveryFunc {
	return func(c *gin.Context, recovered any) {
		if err, ok := recovered.(error); ok {
			c.String(http.StatusInternalServerError, err.Error())
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}
