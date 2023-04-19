package main

import (
	api_pb "github.com/ssau-fiit/cloudocs-api/proto/api"
	"sync"
	"testing"
)

var data = []byte("privet")

type operationsList struct {
	mu   sync.Mutex
	list []*api_pb.Operation
}

func (o *operationsList) Add(t *testing.T, op *api_pb.Operation) {
	//if len(op.Text) > 1 {
	//	panic("text size is greater than one")
	//}

	o.mu.Lock()
	defer o.mu.Unlock()

	conflictOps := make([]*api_pb.Operation, 0, len(o.list))
	for _, operation := range o.list {
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
		tmp := data[:op.Index]
		data = []byte(string(tmp) + op.Text + string(data[op.Index:]))
	case api_pb.OpType_DELETE:
		if int(op.Index) == len(data) {
			data = data[:op.Index-op.Len]
		} else {
			data = append(data[:op.Index-op.Len+1], data[op.Index+1:]...)
		}
	}
	o.list = append(o.list, op)

	t.Log(string(data), o.list)
}

func TestOp(t *testing.T) {
	ops := &operationsList{}

	ops.Add(t, &api_pb.Operation{
		Type:    api_pb.OpType_INSERT,
		Index:   1,
		Version: 1,
		Text:    "blyat",
		Len:     5,
	})
	ops.Add(t, &api_pb.Operation{
		Type:    api_pb.OpType_DELETE,
		Index:   10,
		Len:     5,
		Version: 2,
	})
}
