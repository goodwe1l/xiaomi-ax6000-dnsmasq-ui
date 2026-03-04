package config

import "os"

type PathsConfig struct {
	AuthFilePath    string
	SessionFilePath string
}

type AuthConfig struct {
	SessionCookieName   string
	SessionTTLSeconds   int64
	DefaultAuthPassword string
}

type DHCPConfig struct {
	ConfigName string
}

type ServerConfig struct {
	ListenAddr string
}

type RouteConfig struct {
	APIPath string
}

type AppConfig struct {
	Paths  PathsConfig
	Auth   AuthConfig
	DHCP   DHCPConfig
	Server ServerConfig
	Route  RouteConfig
}

func Default() AppConfig {
	cfg := AppConfig{
		Paths: PathsConfig{
			AuthFilePath:    "/data/dhcp_adv/auth.conf",
			SessionFilePath: "/tmp/dhcp_adv_session",
		},
		Auth: AuthConfig{
			SessionCookieName:   "dhcp_adv_session",
			SessionTTLSeconds:   3600,
			DefaultAuthPassword: "admin123456",
		},
		DHCP: DHCPConfig{
			ConfigName: "dhcp",
		},
		Server: ServerConfig{
			ListenAddr: "0.0.0.0:8088",
		},
		Route: RouteConfig{
			APIPath: "/cgi-bin/dhcp_adv_api",
		},
	}

	if v := os.Getenv("DHCP_ADV_AUTH_FILE"); v != "" {
		cfg.Paths.AuthFilePath = v
	}
	if v := os.Getenv("DHCP_ADV_SESSION_FILE"); v != "" {
		cfg.Paths.SessionFilePath = v
	}
	if v := os.Getenv("DHCP_ADV_LISTEN_ADDR"); v != "" {
		cfg.Server.ListenAddr = v
	}
	if v := os.Getenv("DHCP_ADV_API_PATH"); v != "" {
		cfg.Route.APIPath = v
	}

	return cfg
}
