package rest

import "github.com/gin-gonic/gin"

func NewApi(router *gin.Engine) {
	postsV1 := router.Group("posts/v1")
	{
		postsV1.GET("/", GetPosts)
		postsV1.GET("/:postId", GetPost)
	}

	commentsV1 := router.Group("comments/v1")
	{
		commentsV1.POST("/", PostComment)
		commentsV1.GET("/:postId", GetComments)
	}
}
