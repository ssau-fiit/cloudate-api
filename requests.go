package main

type AuthRequest struct {
	Name string `json:"name"`
}

type CreateDocRequest struct {
	Name   string `json:"name"`
	Author string `json:"author"`
}
