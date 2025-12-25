package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/opensyria/opensy-mining/coopmine"
)

func main() {
	var (
		coordinatorAddr = flag.String("coordinator", "localhost:5555", "Coordinator address")
		workerID        = flag.String("worker-id", "", "Worker ID (auto-generated if empty)")
		workerName      = flag.String("worker-name", "", "Worker name (auto-generated if empty)")
		threads         = flag.Int("threads", 0, "Mining threads (0 = auto-detect)")
		logLevel        = flag.String("log-level", "info", "Log level: debug, info, warn, error")
		logFormat       = flag.String("log-format", "text", "Log format: text or json")
	)

	flag.Parse()

	if *workerID == "" {
		b := make([]byte, 4)
		rand.Read(b)
		*workerID = hex.EncodeToString(b)
	}

	if *workerName == "" {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "worker"
		}
		*workerName = fmt.Sprintf("%s-%s", hostname, (*workerID)[:4])
	}

	if *threads <= 0 {
		*threads = runtime.NumCPU()
	}

	var handler slog.Handler
	level := parseLogLevel(*logLevel)
	opts := &slog.HandlerOptions{Level: level}

	if *logFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	printBanner(*workerName)

	logger.Info("Starting CoopMine Worker",
		"worker_id", *workerID,
		"worker_name", *workerName,
		"coordinator", *coordinatorAddr,
		"threads", *threads,
		"os", runtime.GOOS,
		"arch", runtime.GOARCH,
	)

	cfg := coopmine.ServiceConfig{
		Mode:            "worker",
		CoordinatorAddr: *coordinatorAddr,
		WorkerID:        *workerID,
		WorkerName:      *workerName,
		Threads:         *threads,
		Logger:          logger,
	}

	service := coopmine.NewService(cfg)
	if err := service.Start(); err != nil {
		logger.Error("Failed to start worker", "err", err)
		os.Exit(1)
	}

	go statsReporter(service, logger)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.Info("Received shutdown signal", "signal", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		service.Stop()
		close(done)
	}()

	select {
	case <-shutdownCtx.Done():
		logger.Error("Shutdown timed out")
	case <-done:
		logger.Info("Worker stopped gracefully")
	}
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func printBanner(name string) {
	fmt.Println()
	fmt.Println("  COOPMINE - Cooperative Mining Cluster")
	fmt.Printf("  Mode: WORKER  Name: %s\n", name)
	fmt.Println()
}

func statsReporter(service *coopmine.Service, logger *slog.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := service.GetStats()
		logger.Info("Worker stats",
			"hashrate", formatHashrate(stats.TotalHashrate),
			"shares_valid", stats.SharesValid,
			"shares_invalid", stats.SharesInvalid,
			"threads", stats.Threads,
			"mining", stats.Mining,
			"coordinator", stats.CoordinatorConnected,
			"uptime", stats.Uptime.Round(time.Second),
		)
	}
}

func formatHashrate(h float64) string {
	switch {
	case h >= 1e12:
		return fmt.Sprintf("%.2f TH/s", h/1e12)
	case h >= 1e9:
		return fmt.Sprintf("%.2f GH/s", h/1e9)
	case h >= 1e6:
		return fmt.Sprintf("%.2f MH/s", h/1e6)
	case h >= 1e3:
		return fmt.Sprintf("%.2f KH/s", h/1e3)
	default:
		return fmt.Sprintf("%.2f H/s", h)
	}
}
