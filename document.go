package main

type Document struct {
	ID     string `json:"ID" mapstructure:"id"`
	Name   string `json:"name" mapstructure:"name"`
	Author string `json:"author" mapstructure:"author"`
}
