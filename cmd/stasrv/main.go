// Package main is the entry point of the program
package main

import (
	"context"
	"embed"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/middlewares/server/recovery"
	"github.com/cloudwego/hertz/pkg/app/server"
	hconfig "github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/hlog"

	"github.com/zhufuyi/stasrv/pkg/config"
	"github.com/zhufuyi/stasrv/pkg/httpsrv"
	"github.com/zhufuyi/stasrv/pkg/logger"
	"github.com/zhufuyi/stasrv/pkg/logger/hertzlogger"
	"github.com/zhufuyi/stasrv/pkg/middleware"
	"github.com/zhufuyi/stasrv/pkg/spa"
)

var version, buildTime, commit string // inject from build

//go:embed embedded_dir/*
var embedFS embed.FS

func main() {
	logger.Init(
		logger.WithVersion(version),
		logger.WithServiceName("stasrv"),
	)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("configuration error: %v", err)
	}
	if cfg.ShowVersion {
		config.PrintVersion(version, buildTime, commit)
		return
	}
	logger.Info("configuration information", logger.Any("flags", cfg))

	addr := fmt.Sprintf(":%d", cfg.Port)
	opts := []hconfig.Option{
		server.WithHostPorts(addr),
	}
	if config.IsReleaseVersion(version) {
		opts = append(opts, server.WithDisablePrintRoute(true))
	}
	h := server.New(opts...)
	hlog.SetLogger(hertzlogger.NewHertzLogger())
	h.Use(middleware.AccessLog(middleware.WithLogger(logger.Get())))
	h.Use(recovery.Recovery())

	h.GET("/health", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, map[string]any{
			"status": "UP",
		})
	})

	err = registerStaticFile(cfg, h)
	if err != nil {
		logger.Fatalf("failed to register, %v", err)
	}

	err = httpsrv.RunGracefully(h)
	if err != nil {
		logger.Fatalf("%v", err)
	}
	logger.Info("server exited gracefully")
}

func registerStaticFile(cfg *config.Config, h *server.Hertz) error {
	opts := []spa.Option{
		spa.With404ToHome(true),
		spa.WithListFiles(cfg.EnableListFiles),
		spa.WithCacheMaxAge(cfg.CacheMaxAge),
	}

	for _, location := range cfg.Locations {
		srv, err := spa.NewLocal(location.Path, location.Root, opts...)
		if err != nil {
			return err
		}
		if err = srv.Register(h); err != nil {
			return fmt.Errorf("register '%s -> %s' error: %v", srv.GetBasePath(), srv.GetLocalDir(), err)
		}
		logger.Infof("register static file '%s -> %s' success", srv.GetBasePath(), srv.GetLocalDir())
	}

	if cfg.EmbedFSBasePath != "" {
		srv, err := spa.NewEmbedFS(cfg.EmbedFSBasePath, embedFS, opts...)
		if err != nil {
			return err
		}
		if err = srv.Register(h); err != nil {
			return fmt.Errorf("register embed file '%s -> %s' error: %v", srv.GetBasePath(), srv.GetLocalDir(), err)
		}
		logger.Infof("register embed file '%s -> %s' success", srv.GetBasePath(), srv.GetLocalDir())
	}

	return nil
}
