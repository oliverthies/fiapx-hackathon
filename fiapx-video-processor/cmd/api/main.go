package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oliverthies/fiapx-video-processor/internal/adapter/crypto"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/httpapi"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/httpjwt"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/postgres"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/rabbit"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/smtp"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/wiring"
	"github.com/oliverthies/fiapx-video-processor/internal/application"
	"github.com/oliverthies/fiapx-video-processor/internal/platform"
)

func main() {
	ctx := context.Background()
	dbURL := platform.Env("DATABASE_URL", "postgres://fiapx:fiapx@localhost:5432/fiapx?sslmode=disable")
	if err := postgres.RunMigrations(dbURL); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	db, err := postgres.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	store, storageDriver, err := wiring.NewStorage(ctx)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	q, err := rabbit.DialRetry(platform.Env("RABBIT_URL", "amqp://guest:guest@localhost:5672/"), 20, 2*time.Second)
	if err != nil {
		log.Fatalf("rabbit: %v", err)
	}
	defer q.Close()

	jwt := httpjwt.New(platform.Env("JWT_SECRET", "dev-secret-change-me"), 24*time.Hour)
	auth := application.NewAuthService(db.Users(), crypto.Hasher{}, jwt)
	videos := application.NewVideoService(
		db.Jobs(), db.Users(), store, q,
		nil, smtp.New(platform.Env("SMTP_ADDR", "localhost:1025"), "noreply@fiapx.local"),
	)

	h := httpapi.New(auth, videos, store, jwt, db.Ping)
	addr := platform.Env("HTTP_ADDR", ":8080")
	srv := &http.Server{Addr: addr, Handler: h}

	go func() {
		log.Printf("fiapx-api listening on %s storage=%s", addr, storageDriver)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
