package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Bagareal/Finance-app/internal/server"
	"github.com/Bagareal/Finance-app/internal/store"
	"github.com/Bagareal/Finance-app/web"
)

// version viene impostata in fase di build: -ldflags "-X main.version=v1.0.0"
var version = "dev"

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := env("FA_ADDR", ":8080")
	dataDir := env("FA_DATA", "./data")
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		log.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dataDir, "finance.db"))
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// genera le ricorrenze dovute all'avvio e poi ogni ora
	go func() {
		for {
			if n, err := st.RunRecurring(""); err != nil {
				log.Printf("ricorrenze: %v", err)
			} else if n > 0 {
				log.Printf("ricorrenze: creati %d movimenti", n)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Hour):
			}
		}
	}()

	srv := &http.Server{
		Addr:              addr,
		Handler:           server.New(st, web.FS, version),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()
	log.Printf("Finance App %s in ascolto su %s (dati in %s)", version, addr, dataDir)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
