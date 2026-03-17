package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/akhil-datla/pagechat"
)

var version = "dev"

func main() {
	port := flag.Int("port", 8080, "server port")
	ttl := flag.Duration("ttl", 24*time.Hour, "message time-to-live")
	maxMsgs := flag.Int("max-messages", 1000, "max messages stored per room")
	filter := flag.Bool("filter", true, "enable profanity filter")
	ver := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *ver {
		fmt.Println("pagechat", version)
		return
	}

	srv := pagechat.NewServer(pagechat.Config{
		Addr:          fmt.Sprintf(":%d", *port),
		MessageTTL:    *ttl,
		MaxMessages:   *maxMsgs,
		ContentFilter: *filter,
	})

	// Graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
