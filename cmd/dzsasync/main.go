// Package main is the entry point for dzsa-sync.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/jsirianni/dzsa-sync/client"
	"github.com/jsirianni/dzsa-sync/config"
	"github.com/jsirianni/dzsa-sync/internal/api"
	"github.com/jsirianni/dzsa-sync/internal/ifconfig"
	"github.com/jsirianni/dzsa-sync/internal/metrics"
	"github.com/jsirianni/dzsa-sync/internal/servers"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	defaultAPIPort       = 8888
	syncInterval         = 1 * time.Hour
	syncJitterMaxSeconds = 20
	defaultLogMaxSize    = 100
	defaultLogMaxBackups = 3
	defaultLogMaxAge     = 28
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

	if cfg.LogPath == "" {
		fmt.Fprintln(os.Stderr, "config: log_path is required")
		os.Exit(1)
	}
	logger, err := setupLogger(cfg.LogPath)
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

	apiHost := ""
	apiPort := defaultAPIPort
	if cfg.API != nil {
		apiHost = cfg.API.Host
		if cfg.API.Port != 0 {
			apiPort = cfg.API.Port
		}
	}

	store := servers.New(cfg.Ports)
	apiServer := api.NewServer(
		net.JoinHostPort(apiHost, strconv.Itoa(apiPort)),
		metricsProvider.Handler(),
		store,
	)
	go func() {
		logger.Info("API server listening", zap.String("addr", apiServer.Addr), zap.String("metrics", api.MetricsPath))
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server", zap.Error(err))
			cancel()
		}
	}()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = apiServer.Shutdown(shutdownCtx)
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

	logger.Info("server ports from config, starting sync workers",
		zap.Ints("ports", cfg.Ports))

	var wg sync.WaitGroup
	for i, port := range cfg.Ports {
		wg.Add(1)
		go func(port int, trigger <-chan struct{}) {
			defer wg.Done()
			runPortWorker(signalCtx, logger, dzsaClient, ifconfigClient, cfg, store, port, trigger)
		}(port, triggerChans[i])
	}

	<-signalCtx.Done()
	logger.Info("shutdown signal received, stopping workers")
	cancel()
	wg.Wait()
	logger.Info("shutdown complete")
}

func runPortWorker(ctx context.Context, logger *zap.Logger, dzsa client.Client, ifconfig *ifconfig.Client, cfg *config.Config, store *servers.Store, port int, trigger <-chan struct{}) {
	logger = logger.With(zap.Int("port", port))
	logger.Info("sync worker started for server port")

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	syncOnce := func() {
		jitter := time.Duration(rand.Intn(syncJitterMaxSeconds+1)) * time.Second // #nosec G404 -- jitter only, not security-sensitive
		if jitter > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(jitter):
			}
		}
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
			logger.Error("server sync failed",
				zap.String("endpoint", fmt.Sprintf("%s:%d", ip, port)),
				zap.Error(err))
			return
		}
		result := resp.Result
		store.Set(port, &result)
		logger.Info("server synced with dzsa launcher",
			zap.String("endpoint", result.Endpoint.String()),
			zap.String("name", result.Name),
			zap.Int("players", result.Players),
			zap.Int("max_players", result.MaxPlayers),
			zap.String("version", result.Version),
			zap.String("map", result.Map),
		)
	}

	// Sync once on startup before waiting for the interval
	syncOnce()

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
