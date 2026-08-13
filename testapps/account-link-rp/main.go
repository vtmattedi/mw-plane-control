package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	state := NewState(cfg)
	oidc := NewOIDCClient(cfg)
	app := NewApp(cfg, state, oidc)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	log.Printf("account-link-rp listening on %s", cfg.Addr)
	log.Fatal(server.ListenAndServe())
}
