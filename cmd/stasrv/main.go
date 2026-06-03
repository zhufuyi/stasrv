// Package main is the entry point of the program
package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/zhufuyi/stasrv/pkg/config"
	"github.com/zhufuyi/stasrv/pkg/httpsrv"
	"github.com/zhufuyi/stasrv/pkg/logger"
	"github.com/zhufuyi/stasrv/pkg/middleware"
	"github.com/zhufuyi/stasrv/pkg/spa"
)

var version, buildTime, commit string // inject from build

func main() {
	cfg, err := config.Load() // Load service configuration
	if err != nil {
		panic(err)
	}

	if cfg.ShowVersion {
		config.PrintVersion(version, buildTime, commit)
		return
	}

	logger.Init(logger.WithJSONFormat(cfg.EnableJSONLog))
	defer logger.CloseAsyncLogger()

	if config.IsReleaseVersion(version) {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.SlogLogger())
	r.Use(gin.Recovery())

	f, err := spa.NewLocal(cfg.StaticDir, cfg.BasePath,
		spa.With404ToHome(true),
		spa.WithListFiles(cfg.EnableListFiles),
		spa.WithCacheMaxAge(cfg.CacheMaxAge),
	)
	if err != nil {
		logger.Errorf("Failed to init stasrv, %v", err)
		os.Exit(1)
	}

	err = f.Register(r)
	if err != nil {
		logger.Errorf("Failed to register, %v", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%d", cfg.Port)

	logger.Info("Configuration",
		"dir", cfg.StaticDir,
		"base-path", cfg.BasePath,
		"cache-age", cfg.CacheMaxAge)

	logger.Infof("Starting service on %s", addr)
	err = httpsrv.ListenAndServeGracefully(addr, r)
	if err != nil {
		logger.Fatalf("%v", err)
	}
}
