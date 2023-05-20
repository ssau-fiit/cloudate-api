package main

import (
	api_pb "github.com/ssau-fiit/cloudocs-api/proto/api"
	"testing"
)

var data = []byte("privet")

func TestOp(t *testing.T) {
	ops := &operationsList{}

	ops.Add("0", &api_pb.Operation{
		Type:    api_pb.OpType_INSERT,
		Index:   1,
		Version: 1,
		Text:    "blyat",
		Len:     5,
	})
	ops.Add("0", &api_pb.Operation{
		Type:    api_pb.OpType_DELETE,
		Index:   10,
		Len:     5,
		Version: 2,
	})
}
