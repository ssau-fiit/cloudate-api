package main

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateDocRequest struct {
	Name   string `json:"name"`
	Author string `json:"author"`
}
