package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"github.com/ssau-fiit/cloudocs-api/database"
	api_pb "github.com/ssau-fiit/cloudocs-api/proto/api"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	operationsMu   sync.RWMutex
	operationsList map[string][]*api_pb.Operation
)

func init() {
	operationsList = make(map[string][]*api_pb.Operation)
}

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

func getRandomNumber() int {
	min := 111111
	max := 999999
	return rand.Intn(max-min) + min
}

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

	uid := getRandomNumber()
	_, err = db.HSet(ctx, fmt.Sprintf("documents.%v", uid), "id", uid, "name", r.Name, "author", r.Author).Result()
	if err != nil {
		log.Error().Err(err).Msg("error uploading document")
		c.AbortWithStatus(500)
		return
	}

	if err := db.Set(ctx, fmt.Sprintf("documents.%v.text", uid), "", 0).Err(); err != nil {
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

func main() {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	v1.POST("/auth", handleAuth)
	v1.GET("/documents", handleGetDocuments)
	v1.POST("/documents/create", handleCreateDocument)

	v1.GET("/documents/:id", handleSocket)

	err := r.Run("0.0.0.0:8080")
	if err != nil {
		log.Fatal().Err(err).Msg("could not start server")
	}
}

func handleSocket(c *gin.Context) {
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

	rawRes, err := database.Database().HGetAll(ctx, fmt.Sprintf("documents.%v", docID)).Result()
	if err != nil {
		log.Error().Err(err).Msg("error getting document")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	var doc Document
	err = mapstructure.Decode(rawRes, &doc)
	if err != nil {
		log.Error().Err(err).Msg("error decoding document")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("error upgrading connection")
		c.AbortWithStatus(http.StatusUpgradeRequired)
	}
	defer conn.Close()

	//for {
	//	// Read incoming message from client
	//	_, msg, err := conn.ReadMessage()
	//
	//	go func() {
	//		if err != nil {
	//			log.Error().Err(err).Msg("failed to read message from client")
	//			return
	//		}
	//
	//		...
	//
	//		operationsList = append(operationsList, op)
	//	}()
	//}
}

//func opsHandler(pendingOps chan api_pb.Operation) {
//	for op := range pendingOps {
//
//	}
//}
