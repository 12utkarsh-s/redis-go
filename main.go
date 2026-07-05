package main

import (
	"flag"
	"log"
	"redis-go/config"
	"redis-go/server"
)

func setupFlags() {
	flag.StringVar(&config.Host, "host", "0.0.0.0", "redis host")
	flag.IntVar(&config.Port, "port", 7379, "redis port")
	flag.Parse()
}

func main() {
	setupFlags()
	log.Println("Connecting to redis server...")
	server.RunAsyncTCPServer()
}
