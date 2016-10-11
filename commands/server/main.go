package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/kevinburke/handlers"
	"github.com/saintpete/logrole/server"
)

const DefaultPort = "4114"

func main() {
	port := flag.String("port", DefaultPort, "Port to listen on")
	flag.Parse()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", *port))
	if err != nil {
		handlers.Logger.Error("Error listening", "err", err, "port", *port)
		os.Exit(2)
	}
	publicMux := http.NewServeMux()
	publicMux.Handle("/", server.Server)
	publicServer := http.Server{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		Handler: handlers.Log(
			handlers.Debug(
				handlers.UUID(
					handlers.Server(publicMux, "hfe"),
				),
			),
		),
	}
	go func(p string) {
		time.Sleep(30 * time.Millisecond)
		handlers.Logger.Info("Started server", "port", p)
	}(*port)
	publicServer.Serve(listener)
}
