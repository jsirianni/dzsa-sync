// Package main is the entry point for dzsa-sync.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jsirianni/dzsa-sync/client"
	"github.com/jsirianni/dzsa-sync/config"
	"github.com/jsirianni/dzsa-sync/internal/ifconfig"
	"github.com/jsirianni/dzsa-sync/internal/metrics"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	metricsPort        = "8888"
	metricsPath        = "/metrics"
	syncInterval       = 1 * time.Hour
	defaultLogPath     = "/var/log/dzsa-sync/dzsa-sync.log"
	defaultLogMaxSize  = 100
	defaultLogMaxBackups = 3
	defaultLogMaxAge   = 28
)

func main() {
	configPath := flag.String("config", "", "Path to the YAML configuration file")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "missing required flag: -config")
		os.Exit(1)
	}

	cfg, err := config.NewFromFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	logger, err := setupLogger(defaultLogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalCtx, signalCancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer signalCancel()

	metricsProvider, err := metrics.NewProvider()
	if err != nil {
		logger.Fatal("metrics provider", zap.Error(err))
	}
	defer func() {
		_ = metricsProvider.Shutdown(context.Background())
	}()

	recorder, err := metrics.NewHTTPRecorder()
	if err != nil {
		logger.Fatal("metrics recorder", zap.Error(err))
	}

	httpClient := &http.Client{
		Timeout:   client.DefaultHTTPTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}

	dzsaClient := client.New(client.Options{
		HTTPClient: httpClient,
		Recorder:   recorder,
	})

	ifconfigClient := ifconfig.New(
		logger.With(zap.String("module", "ifconfig")),
		httpClient,
		recorder,
	)

	if !cfg.DetectIP {
		if cfg.ExternalIP == "" {
			logger.Fatal("external_ip required when detect_ip is false")
		}
		ifconfigClient.SetAddress(cfg.ExternalIP)
	}

	// Metrics HTTP server
	mux := http.NewServeMux()
	mux.Handle(metricsPath, metricsProvider.Handler())
	metricsServer := &http.Server{
		Addr:              net.JoinHostPort("", metricsPort),
		Handler:            mux,
		ReadHeaderTimeout:  10 * time.Second,
		ReadTimeout:        10 * time.Second,
		WriteTimeout:       10 * time.Second,
		IdleTimeout:        60 * time.Second,
	}
	go func() {
		logger.Info("metrics server listening", zap.String("addr", ":"+metricsPort), zap.String("path", metricsPath))
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server", zap.Error(err))
			cancel()
		}
	}()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = metricsServer.Shutdown(shutdownCtx)
	}()

	// Trigger channels: one per port; sending triggers an immediate sync and resets the 1h ticker.
	triggerChans := make([]chan struct{}, len(cfg.Ports))
	for i := range triggerChans {
		triggerChans[i] = make(chan struct{}, 1)
	}

	onIPChanged := func(oldIP, newIP string) {
		logger.Info("external IP changed, triggering sync for all servers",
			zap.String("old_ip", oldIP),
			zap.String("new_ip", newIP))
		for _, ch := range triggerChans {
			select {
			case ch <- struct{}{}:
			default:
				// already pending trigger
			}
		}
	}

	if cfg.DetectIP {
		go ifconfigClient.Run(signalCtx, onIPChanged)
		// Give ifconfig one chance to populate IP before starting port workers
		time.Sleep(2 * time.Second)
	}

	var wg sync.WaitGroup
	for i, port := range cfg.Ports {
		wg.Add(1)
		go func(port int, trigger <-chan struct{}) {
			defer wg.Done()
			runPortWorker(signalCtx, logger, dzsaClient, ifconfigClient, cfg, port, trigger)
		}(port, triggerChans[i])
	}

	<-signalCtx.Done()
	logger.Info("shutdown signal received, stopping workers")
	cancel()
	wg.Wait()
	logger.Info("shutdown complete")
}

func runPortWorker(ctx context.Context, logger *zap.Logger, dzsa client.Client, ifconfig *ifconfig.Client, cfg *config.Config, port int, trigger <-chan struct{}) {
	logger = logger.With(zap.Int("port", port))
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	syncOnce := func() {
		ip := ifconfig.GetAddress()
		if ip == "" {
			ip = cfg.ExternalIP
		}
		if ip == "" {
			logger.Warn("no external IP available, skipping sync")
			return
		}
		ctx, cancelReq := context.WithTimeout(ctx, client.DefaultHTTPTimeout)
		defer cancelReq()
		resp, err := dzsa.Query(ctx, ip, port)
		if err != nil {
			logger.Error("dzsa sync failed",
				zap.String("endpoint", fmt.Sprintf("%s:%d", ip, port)),
				zap.Error(err))
			return
		}
		result := resp.Result
		logger.Info("dzsa sync completed",
			zap.String("endpoint", result.Endpoint.String()),
			zap.String("name", result.Name),
			zap.Int("players", result.Players),
			zap.Int("max_players", result.MaxPlayers),
			zap.String("version", result.Version),
			zap.String("map", result.Map),
		)
	}

	for {
		select {
		case <-ticker.C:
			syncOnce()
		case <-trigger:
			syncOnce()
			ticker.Reset(syncInterval)
		case <-ctx.Done():
			return
		}
	}
}

func setupLogger(logPath string) (*zap.Logger, error) {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.CallerKey = ""
	encoderConfig.StacktraceKey = ""
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.MessageKey = "message"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	writer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    defaultLogMaxSize,
		MaxBackups: defaultLogMaxBackups,
		MaxAge:     defaultLogMaxAge,
		Compress:   true,
	})

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		writer,
		zap.DebugLevel,
	)
	return zap.New(core), nil
}
