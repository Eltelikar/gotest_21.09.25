package main

import (
	"gotest/internal/config"
	"gotest/internal/database"
	"gotest/internal/slogpretty"
	"log/slog"
	"os"
)

// Константы для уровня логгера
const (
	envLocal = "local"
	envDebug = "debug"
	envProd  = "prod"
)

func main() {

	cfg := config.NewConfigFile()
	slog.Info("config successfully loaded!")

	log := setupLogger(cfg.Env)
	slog.SetDefault(log)

	db, err := database.NewDB(cfg)
	if err != nil {
		slog.Error("fatal error, service shutdown", "err", err)
		os.Exit(1)
	}

	_ = db

	//TODO: HTTP сервер

	//TODO: Хендлеры
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		opts := slogpretty.PrettyHandlerOptions{
			SlogOpts: &slog.HandlerOptions{
				Level: slog.LevelDebug,
			},
		}
		log = slog.New(opts.NewPrettyHandler(os.Stdout))
	case envDebug:
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case envProd:
		log = slog.New((slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	}

	return log
}
