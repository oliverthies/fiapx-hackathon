package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oliverthies/fiapx-video-processor/internal/adapter/ffmpeg"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/fs"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/metrics"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/postgres"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/rabbit"
	"github.com/oliverthies/fiapx-video-processor/internal/adapter/smtp"
	"github.com/oliverthies/fiapx-video-processor/internal/application"
	"github.com/oliverthies/fiapx-video-processor/internal/platform"
)

func main() {
	ctx := context.Background()
	dbURL := platform.Env("DATABASE_URL", "postgres://fiapx:fiapx@localhost:5432/fiapx?sslmode=disable")
	db, err := postgres.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	root := platform.Env("STORAGE_ROOT", "/data")
	if _, err := fs.New(root); err != nil {
		log.Fatalf("storage: %v", err)
	}
	q, err := rabbit.DialRetry(platform.Env("RABBIT_URL", "amqp://guest:guest@localhost:5672/"), 20, 2*time.Second)
	if err != nil {
		log.Fatalf("rabbit: %v", err)
	}
	defer q.Close()

	videos := application.NewVideoService(
		db.Jobs(), db.Users(), nil, q,
		ffmpeg.New(root, 15*time.Minute),
		smtp.New(platform.Env("SMTP_ADDR", "mailhog:1025"), "noreply@fiapx.local"),
	)

	deliveries, err := q.Consume()
	if err != nil {
		log.Fatalf("consume: %v", err)
	}
	log.Print("fiapx-worker consuming video.process")
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
