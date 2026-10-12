package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"chainmaker.org/chainmaker-go/module/sharding/gateway"
)

func main() {
	confPath := flag.String("conf", "", "gateway config file path")
	flag.Parse()
	if *confPath == "" {
		log.Fatalf("missing --conf")
	}
	cfg, err := gateway.LoadConfig(*confPath)
	if err != nil {
		log.Fatalf("load gateway config failed: %v", err)
	}
	srv, err := gateway.NewServer(cfg)
	if err != nil {
		log.Fatalf("create gateway server failed: %v", err)
	}

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stopCh
		log.Printf("[gateway] shutting down")
		srv.Stop()
	}()

	if err := srv.Start(); err != nil {
		log.Fatalf("gateway server exited with error: %v", err)
	}
}
