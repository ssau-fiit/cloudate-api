package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"github.com/ssau-fiit/cloudate-api/common/uuid"
	"github.com/ssau-fiit/cloudate-api/database"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var conns map[string]*websocket.Conn
var localOps []Operation
var doc []byte

func init() {
	conns = make(map[string]*websocket.Conn)
	doc = make([]byte, 0, 1000)
}

func handleAuth(c *gin.Context) {
	var r AuthRequest
	err := c.BindJSON(&r)
	if err != nil {
		log.Error().Err(err).Msg("could not parse request")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	id := uuid.Must(uuid.NewV4())
	uid := id.String()

	c.JSON(http.StatusOK, gin.H{
		"session_id": uid,
	})
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	uid := uuid.Must(uuid.NewV4()).String()
	_, err = database.Database().HSet(ctx, fmt.Sprintf("documents.%v", uid), "id", uid, "name", r.Name, "author", r.Author).Result()
	if err != nil {
		log.Error().Err(err).Msg("error uploading document")
		c.AbortWithStatus(500)
		return
	}

	c.JSON(200, Document{
		ID:     uid,
		Name:   r.Name,
		Author: r.Author,
	})
}

func main() {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	v1.POST("/handleAuth", handleAuth)
	v1.GET("/documents", handleGetDocuments)
	v1.POST("/documents/create", handleCreateDocument)

	v1.GET("/socket", handleSocket)

	err := r.Run("0.0.0.0:8080")
	if err != nil {
		log.Fatal().Err(err).Msg("could not start server")
	}
}

func handleSocket(c *gin.Context) {
	sessionID := uuid.Must(uuid.NewV4()).String()
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to upgrade connection")
		return
	}
	defer conn.Close()
	// TODO: create a copy of the document for this session

	conns[sessionID] = conn

	for {
		// Read incoming message from client
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Error().Err(err).Msg("failed to read message from client")
			return
		}
		// Decode incoming operation
		var remoteOps []Operation
		err = json.Unmarshal(msg, &remoteOps)
		if err != nil {
			log.Error().Err(err).Msg("failed to unmarshal client message")
			continue
		}

		// Применить удаленные операции к локальным
		transformedOps := transformRemoteOperations(localOps, remoteOps)

		// Применяем удаленные операции к локальному документу
		updatedDoc, err := applyLocalOperations(doc, transformedOps)
		if err != nil {
			log.Error().Err(err).Msg("could not apply remote operations to local copy")
			continue
		}

		// Обновляем список локальных операций
		localOps = append(localOps, remoteOps...)

		// Уведомляем остальных клиентов об изменениях
		broadcast(conns, updatedDoc)
	}
}

func applyLocalOperations(doc []byte, ops []Operation) ([]byte, error) {
	buffer := bytes.NewBuffer(doc)
	offset := 0
	for _, op := range ops {
		switch op.Type {
		case opTypeInsert:
			// Move the buffer to the insertion position
			buffer.Next(op.Index - offset)
			// Insert the new text
			_, err := buffer.Write([]byte(op.Text))
			if err != nil {
				return nil, err
			}
			// Update the offset to account for the insertion
			offset = op.Index + op.Length
		case opTypeDelete:
			// Move the buffer to the deletion position
			buffer.Next(op.Index - offset)
			// Delete the specified number of bytes
			buffer.Next(op.Length)
			// Update the offset to account for the deletion
			offset = op.Index
		default:
			return nil, fmt.Errorf("invalid operation type: %v", op.Type)
		}
	}
	return buffer.Bytes(), nil
}

func transformRemoteOperations(localOps, remoteOps []Operation) []Operation {
	transformedOps := make([]Operation, len(remoteOps))
	offset := 0

	// Apply local operations to remote operations
	for _, localOp := range localOps {
		for j, remoteOp := range remoteOps {
			if remoteOp.Type == opTypeInsert && localOp.Index > remoteOp.Index {
				offset += len(localOp.Text)
			} else if remoteOp.Type == opTypeDelete && localOp.Index > remoteOp.Index+remoteOp.Length {
				offset -= remoteOp.Length
			}
			localOps[j].Index += offset
		}

		// Применяем трансформированные операции к локальным
		for i, op := range transformedOps {
			if op.Index > localOp.Index {
				if localOp.Type == opTypeInsert {
					transformedOps[i].Index += len(localOp.Text)
				} else if localOp.Type == opTypeDelete {
					transformedOps[i].Index -= localOp.Length
				}
			}
		}
	}

	return transformedOps
}

func broadcast(connections map[string]*websocket.Conn, doc []byte) {
	for _, conn := range connections {
		err := conn.WriteMessage(websocket.TextMessage, doc)
		if err != nil {
			log.Error().Err(err).Msg("failed to write message")
		}
	}
}
