package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/jsonpb"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"github.com/ssau-fiit/cloudocs-api/database"
	api_pb "github.com/ssau-fiit/cloudocs-api/proto/api"
	"net/http"
	"sync"
	"time"
)

var (
	decoder = jsonpb.Unmarshaler{
		AllowUnknownFields: false,
	}
	encoder = jsonpb.Marshaler{
		EnumsAsInts: false,
	}
	opsList = operationsList{
		mu:  make(map[string]*sync.Mutex),
		ops: make(map[string][]*api_pb.Operation),
	}
)

type operationsList struct {
	mu  map[string]*sync.Mutex
	ops map[string][]*api_pb.Operation
}

func (o *operationsList) Add(docID string, op *api_pb.Operation) error {
	o.mu[docID].Lock()
	defer o.mu[docID].Unlock()

	db := database.Database()
	text, err := db.Get(context.Background(), fmt.Sprintf("texts.%v", docID)).Result()
	if err != nil {
		log.Error().Err(err).Msg("error getting document text")
		return err
	}
	textBytes := []byte(text)

	conflictOps := make([]*api_pb.Operation, 0, len(o.ops[docID]))
	for _, operation := range o.ops[docID] {
		if operation.Version == op.Version {
			conflictOps = append(conflictOps, operation)
		}
	}

	if len(conflictOps) > 0 {
		for _, conflictOp := range conflictOps {
			switch true {
			case op.Index < conflictOp.Index:
				switch op.Type {
				case api_pb.OpType_INSERT:
					conflictOp.Index += op.Len
				case api_pb.OpType_DELETE:
					conflictOp.Index -= op.Len
				}

			case op.Index > conflictOp.Index:
				fallthrough
			default:
				switch conflictOp.Type {
				case api_pb.OpType_INSERT:
					op.Index += conflictOp.Len
				case api_pb.OpType_DELETE:
					op.Index -= conflictOp.Len
				}
			}
		}
	}

	switch op.Type {
	case api_pb.OpType_INSERT:
		tmp := text[:op.Index]
		textBytes = []byte(tmp + op.Text + string(textBytes[op.Index:]))
	case api_pb.OpType_DELETE:
		if int(op.Index) == len(textBytes) {
			textBytes = textBytes[:op.Index-op.Len]
		} else {
			textBytes = append(textBytes[:op.Index-op.Len+1], textBytes[op.Index+1:]...)
		}
	}
	o.ops[docID] = append(o.ops[docID], op)

	_, err = db.Set(context.Background(), fmt.Sprintf("texts.%v", docID), string(textBytes), 0).Result()

	return err
}

func handleSocket(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if _, ok := opsList.mu[docID]; !ok {
		opsList.mu[docID] = &sync.Mutex{}
	}
	if _, ok := opsList.ops[docID]; !ok {
		opsList.ops[docID] = []*api_pb.Operation{}
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

	for {
		// Read incoming message from client
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			log.Error().Err(err).Msg("failed to read message from client")
			return
		}

		var ev api_pb.Event
		err = decoder.Unmarshal(bytes.NewReader(msg), &ev)
		if err != nil {
			log.Error().Err(err).Msg("error unmarshaling message")
			continue
		}

		switch ev.Type {
		case api_pb.Event_OPERATION:
			var op api_pb.Operation
			err = decoder.Unmarshal(bytes.NewReader(ev.Event), &op)
			if err != nil {
				log.Error().Err(err).Msg("error unmarshaling operation")
				continue
			}

			err = opsList.Add(docID, &op)
			if err != nil {
				log.Error().Err(err).Msg("error while doing operation")
			}
			log.Debug().Interface("operation", op).Msg("operation received")

			ack := &api_pb.OperationAck{
				LastVersion: opsList.ops[docID][len(opsList.ops[docID])-1].Version,
			}
			ackStr, _ := encoder.MarshalToString(ack)

			resEv := &api_pb.Event{
				Type:  api_pb.Event_OPERATION_ACK,
				Event: []byte(ackStr),
			}
			res, _ := encoder.MarshalToString(resEv)

			conn.WriteMessage(mt, []byte(res))
		}
	}
}
