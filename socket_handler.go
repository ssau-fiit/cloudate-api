package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"github.com/ssau-fiit/cloudocs-api/database"
	"net/http"
	"time"
)

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

	text, err := database.Database().Get(ctx, fmt.Sprintf("texts.%v", docID)).Result()
	if err != nil {
		log.Error().Err(err).Msg("error getting document text")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	for {
		// Read incoming message from client
		mt, msg, err := conn.ReadMessage()

		go func() {
			if err != nil {
				log.Error().Err(err).Msg("failed to read message from client")
				return
			}

			fmt.Println(string(msg))

			err := conn.WriteMessage(mt, []byte(text))
			if err != nil {
				log.Error().Err(err).Msg("failed to write message")
			}
		}()
	}
}
