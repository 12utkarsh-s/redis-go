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

var evictionStrategies = map[string]bool{
	"simple-first":   true,
	"allkeys-random": true,
	"allkeys-lru":    true,
	"allkeys-lfu":    true,
}

func setupFlags() {
	flag.StringVar(&config.Host, "host", "0.0.0.0", "redis host")
	flag.IntVar(&config.Port, "port", 7379, "redis port")
	flag.StringVar(&config.EvictionStrategy, "eviction-strategy", config.EvictionStrategy,
		"key eviction strategy: simple-first, allkeys-random, allkeys-lru, allkeys-lfu")
	flag.Parse()

	if !evictionStrategies[config.EvictionStrategy] {
		log.Fatalf("unknown eviction strategy %q", config.EvictionStrategy)
	}
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
