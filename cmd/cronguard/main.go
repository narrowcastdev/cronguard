package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/narrowcastdev/cronguard/internal/alert"
	"github.com/narrowcastdev/cronguard/internal/checker"
	"github.com/narrowcastdev/cronguard/internal/server"
	"github.com/narrowcastdev/cronguard/internal/store"
	"github.com/narrowcastdev/cronguard/ui"
)

func main() {
	listen := flag.String("listen", "", "address to bind")
	data := flag.String("data", "./cronguard.db", "path to SQLite database")
	password := flag.String("password", "", "admin password (or CRONGUARD_PASSWORD env)")
	flag.Parse()

	pass := *password
	if pass == "" {
		pass = os.Getenv("CRONGUARD_PASSWORD")
	}

	addr := *listen
	if addr == "" {
		if pass != "" {
			addr = "0.0.0.0:8099"
		} else {
			addr = "127.0.0.1:8099"
		}
	}

	db, err := store.Open(*data)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	sender := alert.NewMultiSender(alert.NewWebhookSender(), db)
	chk := checker.New(db, sender)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go chk.Run(ctx)

	srv := server.New(db, chk, pass, ui.Files)

	fmt.Fprintf(os.Stderr, "cronguard (narrowcast.dev) listening on %s\n", addr)
	if pass == "" {
		fmt.Fprintln(os.Stderr, "  No password set — local-only mode (no auth)")
	}

	httpServer := &http.Server{Addr: addr, Handler: srv}
	go func() {
		<-ctx.Done()
		httpServer.Close()
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
