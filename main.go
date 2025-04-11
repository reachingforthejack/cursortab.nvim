package main

import (
	"cursortab.nvim/rpcplugin"
	"log"
	"os"
)

func main() {
	f, err := os.OpenFile("cursortablogs", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)
	plugin, err := rpcplugin.New()
	if err != nil {
		log.Fatalf("failed to create plugin: %v", err)
	}

	if err := plugin.BeginListening(); err != nil {
		log.Fatalf("failed to register handlers: %v", err)
	}
}
