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
	"github.com/saintpete/logrole/services"
)

const DefaultPort = "4114"

func main() {
	fmt.Printf("environ: %v\n", os.Environ())
	port := flag.String("port", DefaultPort, "Port to listen on")

	user := flag.String("user", "", "Username for HTTP Basic Auth")
	pass := flag.String("password", "", "Password for HTTP Basic Auth")
	flag.Parse()
	realm := services.Realm()
	if realm == services.Prod && (*user == "" || *pass == "") {
		handlers.Logger.Error("Cannot run in production without Basic Auth")
		os.Exit(2)
	}
	allowHTTP := false
	if realm == services.Local {
		allowHTTP = true
	}
	s := server.NewServer(allowHTTP, map[string]string{
		*user: *pass,
	})
	publicMux := http.NewServeMux()
	publicMux.Handle("/", s)
	publicServer := http.Server{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		Handler: handlers.Log(
			handlers.Debug(
				handlers.UUID(
					handlers.Server(publicMux, "logrole"),
				),
			),
		),
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", *port))
	if err != nil {
		handlers.Logger.Error("Error listening", "err", err, "port", *port)
		os.Exit(2)
	}
	go func(p string) {
		time.Sleep(30 * time.Millisecond)
		handlers.Logger.Info("Started server", "port", p)
	}(*port)
	publicServer.Serve(listener)
}
