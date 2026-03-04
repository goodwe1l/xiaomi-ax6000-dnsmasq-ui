package main

import (
	"io/fs"
	"net/http"
	"strings"

	"xiaomi-dnsmasq-gui/cmd/api/config"
	"xiaomi-dnsmasq-gui/cmd/api/handler"
	"xiaomi-dnsmasq-gui/cmd/api/middleware"
)

type actionRoute struct {
	handler     http.HandlerFunc
	requireAuth bool
}

func NewRouter(cfg config.AppConfig, api *handler.APIHandler, webFS fs.FS) http.Handler {
	actionRouter := newActionRouter(api)
	mux := http.NewServeMux()
	mux.Handle(cfg.Route.APIPath, actionRouter)
	mux.Handle("/", http.FileServer(http.FS(webFS)))
	return mux
}

func newActionRouter(api *handler.APIHandler) http.Handler {
	routes := map[string]actionRoute{
		"auth_status":     {handler: api.HandleAuthStatus, requireAuth: false},
		"login":           {handler: api.HandleLogin, requireAuth: false},
		"logout":          {handler: api.HandleLogout, requireAuth: false},
		"get_state":       {handler: api.HandleGetState, requireAuth: true},
		"toggle_dhcp":     {handler: api.HandleToggleDHCP, requireAuth: true},
		"save_default":    {handler: api.HandleSaveDefault, requireAuth: true},
		"lease_upsert":    {handler: api.HandleLeaseUpsert, requireAuth: true},
		"lease_delete":    {handler: api.HandleLeaseDelete, requireAuth: true},
		"template_upsert": {handler: api.HandleTemplateUpsert, requireAuth: true},
		"template_delete": {handler: api.HandleTemplateDelete, requireAuth: true},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		action := strings.TrimSpace(r.FormValue("action"))
		if action == "" {
			action = "get_state"
		}

		route, ok := routes[action]
		if !ok {
			api.HandleUnsupportedAction(w, action)
			return
		}

		h := route.handler
		if route.requireAuth {
			h = middleware.RequireAuth(api.IsAuthenticated, api.HandleUnauthorized, h)
		}
		h(w, r)
	})
}
