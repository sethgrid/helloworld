package main

import (
	"log"

	"github.com/sethgrid/helloworld/server"
)

var Version string = "v0.0.0-dev"

func main() {
	log.Println("starting helloworld " + Version)
	conf, err := server.NewConfigFromEnv()
	if err != nil {
		log.Fatalf("unable to parse configuration: %v", err)
	}
	conf.Version = Version
	srv, err := server.New(conf)
	if err != nil {
		log.Fatalf("unable to create helloworld server: %v", err)
	}

	log.Printf("Configuration: %s", conf.String())

	defer func() {
		err := srv.Close()
		if err != nil {
			log.Fatalf("unable to close server cleanly: %v", err)
		}
	}()

	if err := srv.Serve(); err != nil {
		log.Fatalf("unable to serve helloworld: %v", err)
	}
}
