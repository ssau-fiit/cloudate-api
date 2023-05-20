package main

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func main() {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	v1.POST("/auth", handleAuth)

	v1.GET("/documents", handleGetDocuments)
	v1.POST("/documents/create", handleCreateDocument)
	v1.GET("/documents/:id", handleSocket)
	v1.DELETE("/documents/:id", handleDeleteDocument)

	err := r.Run("0.0.0.0:8080")
	if err != nil {
		log.Fatal().Err(err).Msg("could not start server")
	}
}
