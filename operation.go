package main

import "time"

const (
	opTypeInsert = "insert"
	opTypeDelete = "delete"
)

type Operation struct {
	Type      string    `json:"type"`
	Revision  int       `json:"revision"`
	Index     int       `json:"index"`
	Length    int       `json:"length"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}
