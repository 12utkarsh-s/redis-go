package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"redis-go/config"
	"redis-go/server"
	"sync"
	"syscall"
)

func setupFlags() {
	flag.StringVar(&config.Host, "host", "0.0.0.0", "redis host")
	flag.IntVar(&config.Port, "port", 7379, "redis port")
	flag.Parse()
}

func main() {
	setupFlags()
	log.Println("Connecting to redis server 📶")

	var sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		err := server.RunAsyncTCPServer(&wg)
		if err != nil {
			log.Fatal(err)
		}
	}()
	go server.WaitForSignal(&wg, sigs)

	wg.Wait()
}
