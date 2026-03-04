package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"xiaomi-dnsmasq-gui/cmd/api/config"
	"xiaomi-dnsmasq-gui/cmd/api/handler"
	uciutil "xiaomi-dnsmasq-gui/pkg/uci"
)

//go:embed web/**
var embeddedWebFS embed.FS

func main() {
	cfg := config.Default()
	apiHandler := handler.NewAPIHandler(cfg, uciutil.NewClient())

	webFS, err := fs.Sub(embeddedWebFS, "web")
	if err != nil {
		log.Fatalf("加载嵌入式前端资源失败: %v", err)
	}

	router := NewRouter(cfg, apiHandler, webFS)
	server := &http.Server{
		Addr:    cfg.Server.ListenAddr,
		Handler: router,
	}

	log.Printf("DHCP 管理服务启动: %s", cfg.Server.ListenAddr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("服务退出: %v", err)
	}
}
