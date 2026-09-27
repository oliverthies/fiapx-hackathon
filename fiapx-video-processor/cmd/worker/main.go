package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oliverthies/fiapx-video-processor/internal/adapter/metrics"
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
	proc, processorDriver, err := wiring.NewProcessor(store, storageDriver)
	if err != nil {
		log.Fatalf("processor: %v", err)
	}
	q, err := rabbit.DialRetry(platform.Env("RABBIT_URL", "amqp://guest:guest@localhost:5672/"), 20, 2*time.Second)
	if err != nil {
		log.Fatalf("rabbit: %v", err)
	}
	defer q.Close()

	videos := application.NewVideoService(
		db.Jobs(), db.Users(), store, q,
		proc,
		smtp.New(platform.Env("SMTP_ADDR", "mailhog:1025"), "noreply@fiapx.local"),
	)

	deliveries, err := q.Consume()
	if err != nil {
		log.Fatalf("consume: %v", err)
	}
	log.Printf("fiapx-worker consuming video.process storage=%s processor=%s", storageDriver, processorDriver)
	metrics.Serve(platform.Env("METRICS_ADDR", ":9090"))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-stop:
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			msg, err := rabbit.Parse(d)
			if err != nil {
				log.Printf("bad message: %v", err)
				_ = d.Nack(false, false)
				continue
			}
			log.Printf("processing job=%s corr=%s", msg.JobID, msg.CorrelationID)
			metrics.JobsProcessing.Inc()
			err = videos.Process(context.Background(), msg.JobID)
			metrics.JobsProcessing.Dec()
			if err != nil {
				if postgres.IsUndefinedColumn(err) {
					log.Printf("job %s schema not ready, requeue: %v", msg.JobID, err)
					time.Sleep(2 * time.Second)
					_ = d.Nack(false, true)
					continue
				}
				metrics.JobsFailed.Inc()
				log.Printf("job %s failed: %v", msg.JobID, err)
				_ = d.Nack(false, false)
				continue
			}
			metrics.JobsReady.Inc()
			_ = d.Ack(false)
			log.Printf("job %s ready", msg.JobID)
		}
	}
}
