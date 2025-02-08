package rest

import (
	"github.com/dfryer1193/goblog/api"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetPosts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func GetPost(c *gin.Context) {
	postId := c.Param("postId")

	// TODO: Fetch post from db
	post := api.Post{}
	c.JSON(http.StatusOK, post)
}
