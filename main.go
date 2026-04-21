package main

import (
	"log"
	"os"
	"os/signal"

	derodpkg "github.com/civilware/derodpkg/cmd"
)

func main() {
	initparams := make(map[string]interface{})
	initparams["--rpc-bind"] = "127.0.0.1:20202"
	initparams["--p2p-bind"] = "127.0.0.1:20201"
	initparams["--getwork-bind"] = "127.0.0.1:20200"

	d, err := derodpkg.NewDaemon(initparams)
	if err != nil {
		log.Fatalf("failed to create daemon: %v", err)
	}

	if err := d.Initialize(); err != nil {
		log.Fatalf("failed to initialize daemon: %v", err)
	}

	if err := d.Start(); err != nil {
		log.Fatalf("failed to start daemon: %v", err)
	}

	defer func() {
		if err := d.Stop(); err != nil {
			log.Printf("error stopping daemon: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
}
