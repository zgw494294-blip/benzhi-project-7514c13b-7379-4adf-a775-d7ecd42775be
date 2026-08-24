package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"corelog/internal/httpapi"
	"corelog/internal/repository"
	"corelog/internal/service"
)

type config struct {
	Address   string
	DataPath  string
	Selfcheck bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("corelog server stopped", "error", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	flags := flag.NewFlagSet("corelog-server", flag.ContinueOnError)
	address := flags.String("addr", "", "监听地址，默认 127.0.0.1:19081")
	dataPath := flags.String("data", "data/corelog-ledger.json", "本地 JSON 账本路径")
	selfcheckMode := flags.Bool("selfcheck", false, "执行一次有界自检后退出")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("不支持位置参数: %s", strings.Join(flags.Args(), " "))
	}
	resolved, err := resolveAddress(*address, os.Getenv("PORT"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(*dataPath) == "" {
		return errors.New("-data 不能为空")
	}
	configuration := config{Address: resolved, DataPath: *dataPath, Selfcheck: *selfcheckMode}
	return start(configuration)
}

func start(configuration config) error {
	repo, err := repository.New(configuration.DataPath)
	if err != nil {
		return fmt.Errorf("打开本地账本失败: %w", err)
	}
	svc := service.New(repo)
	if configuration.Selfcheck {
		report := svc.Selfcheck()
		output := struct {
			Address  string `json:"address"`
			DataPath string `json:"dataPath"`
			Report   any    `json:"report"`
		}{Address: configuration.Address, DataPath: configuration.DataPath, Report: report}
		if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
			return err
		}
		if !report.Passed {
			return errors.New("selfcheck 未通过")
		}
		return nil
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &http.Server{
		Addr: configuration.Address, Handler: httpapi.New(svc, logger).Routes(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("岩芯编录复核台已启动", "address", configuration.Address, "dataPath", configuration.DataPath)
		serverErrors <- server.ListenAndServe()
	}()
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdownContext.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
}
