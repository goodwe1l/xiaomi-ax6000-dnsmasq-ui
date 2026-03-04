package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"dhcp_adv/cmd/dhcp_adv_api/config"
	"dhcp_adv/cmd/dhcp_adv_api/handler"
	ucipkg "dhcp_adv/cmd/dhcp_adv_api/uci"
)

//go:embed web/**
var embeddedWebFS embed.FS

func main() {
	cfg := config.Default()
	apiHandler := handler.NewAPIHandler(cfg, ucipkg.NewClient())

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
