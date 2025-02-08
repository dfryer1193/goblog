package rest

import (
	"github.com/dfryer1193/goblog/api"
	"github.com/gin-gonic/gin"
	"net/http"
)

func NewCommentsApi(service *gin.Engine) {

}

func PostComment(c *gin.Context) {
	commentProto := &api.CommentProto{}
	if err := c.ShouldBindJSON(commentProto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: save comment to database
	c.Status(http.StatusOK)
}

func GetComments(c *gin.Context) {
	postID := c.Param("postId")

	// TODO: get comment tree from db
	comments := []api.Comment{}

	c.JSON(http.StatusOK, comments)
}
