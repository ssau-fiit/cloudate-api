package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"github.com/ssau-fiit/cloudocs-api/common/util"
	"github.com/ssau-fiit/cloudocs-api/database"
	"net/http"
	"strconv"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

/////////////////////////////
/// Auth Handlers
/////////////////////////////

func handleAuth(c *gin.Context) {
	var r AuthRequest
	err := c.BindJSON(&r)
	if err != nil {
		log.Error().Err(err).Msg("could not parse request")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	db := database.Database()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	res, err := db.HGetAll(ctx, fmt.Sprintf("users.%v", r.Username)).Result()
	if err != nil {
		log.Error().Err(err).Msg("failed to find user")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var user User
	err = mapstructure.Decode(res, &user)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal user structure")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if user.Password != r.Password {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.JSON(200, gin.H{
		"user_id": user.ID,
	})
}

/////////////////////////////
/// Document Handlers
/////////////////////////////

func handleGetDocuments(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	db := database.Database()
	keys, err := db.Keys(ctx, "documents.*").Result()
	if err != nil {
		log.Error().Err(err).Msg("failed to get document keys")
		c.AbortWithStatus(500)
		return
	}

	var documents []Document
	for _, key := range keys {
		docMap, err := database.Database().HGetAll(ctx, key).Result()
		if err != nil {
			log.Error().Err(err).Msg("error getting document")
			c.AbortWithStatus(500)
			return
		}

		var doc Document
		err = mapstructure.Decode(docMap, &doc)
		if err != nil {
			log.Error().Err(err).Msg("error decoding map")
			c.AbortWithStatus(500)
			return
		}
		documents = append(documents, doc)
	}

	c.JSON(200, documents)
}

func handleCreateDocument(c *gin.Context) {
	var r CreateDocRequest
	err := c.BindJSON(&r)
	if err != nil {
		log.Error().Err(err).Msg("bad request")
		c.AbortWithStatus(500)
		return
	}
	if r.Author == "" {
		r.Author = "Автор"
	}

	db := database.Database()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	uid := util.GetRandomNumber()
	_, err = db.HSet(ctx, fmt.Sprintf("documents.%v", uid), "id", uid, "name", r.Name, "author", r.Author).Result()
	if err != nil {
		log.Error().Err(err).Msg("error uploading document")
		c.AbortWithStatus(500)
		return
	}

	if err := db.Set(ctx, fmt.Sprintf("texts.%v", uid), "start typing", 0).Err(); err != nil {
		log.Error().Err(err).Msg("error creating document text")
		db.HDel(ctx, fmt.Sprintf("documents.%v", uid))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(200, Document{
		ID:     strconv.Itoa(uid),
		Name:   r.Name,
		Author: r.Author,
	})
}

func handleDeleteDocument(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if exists, err := database.Database().
		Exists(ctx, fmt.Sprintf("documents.%v", docID)).
		Result(); exists == 0 || err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	_, err := database.Database().Del(ctx, fmt.Sprintf("documents.%v", docID)).Result()
	if err != nil {
		log.Error().Err(err).Msg("error deleting document info")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	_, err = database.Database().Del(ctx, fmt.Sprintf("texts.%v", docID)).Result()
	if err != nil {
		log.Error().Err(err).Msg("error deleting document text")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(200)
}
