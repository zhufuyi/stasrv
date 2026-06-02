// Package main is the entry point of the program
package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"os"

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

	// If there is Nginx/gateway in front, this must be configured,
	//otherwise c.ClientIP() gets the IP of the intranet gateway.
	//_ = r.SetTrustedProxies([]string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})

	f, err := spa.NewLocal(cfg.StaticDir, cfg.BasePath,
		spa.With404ToHome(),
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
