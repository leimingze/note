package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"pulseframe-review/internal/judge"
	"pulseframe-review/internal/migration"
	"pulseframe-review/internal/platform/config"
	"pulseframe-review/internal/platform/infra"
	"pulseframe-review/internal/platform/logging"
	"pulseframe-review/internal/review"
)

func main() {
	logging.Setup()
	slog.Info("PulseFrame Review Service starting")

	httpCfg, err := config.LoadHTTP()
	must(err, "load HTTP config")
	mysqlCfg, err := config.LoadMySQL()
	must(err, "load MySQL config")
	contentCfg, err := config.LoadContentModule()
	must(err, "load content module config")
	reviewCfg, err := config.LoadReview()
	must(err, "load review config")
	vlmCfg, err := judge.LoadConfigFromEnv()
	must(err, "load VLM config")
	serviceKey := os.Getenv("SERVICE_KEY")
	if serviceKey == "" {
		logging.Fatal("SERVICE_KEY is required", nil)
	}
	identVersion := os.Getenv("IDENT_VERSION")
	if identVersion == "" {
		logging.Fatal("IDENT_VERSION is required (模型 + 提示词 + 观察项定义的版本)", nil)
	}
	prompt, err := os.ReadFile(reviewCfg.PromptPath)
	must(err, "read observation prompt")

	db, err := infra.NewMySQL(mysqlCfg)
	must(err, "connect MySQL")
	must(migration.Migrate(db), "migrate database")

	repo := review.NewRepository(db)
	engine := judge.Engine{Inference: judge.NewClientFromConfig(vlmCfg)}
	judging := judge.Policy{Version: identVersion, AllowedLabels: judge.AllowedLabels, SystemPrompt: string(prompt)}
	service, err := review.NewService(repo, review.NewContentClient(contentCfg), review.NewWorkerClient(reviewCfg.WorkerURL),
		engine, judging, reviewCfg)
	must(err, "load moderation policies")

	mux := http.NewServeMux()
	review.RegisterRoutes(mux, review.NewHandler(service, serviceKey))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("OK")) })
	server := &http.Server{Addr: fmt.Sprintf(":%d", httpCfg.Port), Handler: mux,
		ReadTimeout: httpCfg.ReadTimeout, WriteTimeout: httpCfg.WriteTimeout}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	workerDone := make(chan struct{})
	go func() {
		review.NewWorker(service, repo).Run(ctx)
		close(workerDone)
	}()
	go func() {
		slog.Info("HTTP server listening", "addr", server.Addr, "ident_version", identVersion,
			"vlm", vlmCfg.Model, "workers", reviewCfg.Workers, "policy", reviewCfg.PolicyTable)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Fatal("HTTP server failed", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down: waiting for running tasks")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpCfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown failed", "error", err)
	}
	<-workerDone
	slog.Info("server stopped")
}

func must(err error, step string) {
	if err != nil {
		logging.Fatal("failed to "+step, err)
	}
}
