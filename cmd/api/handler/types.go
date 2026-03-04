package handler

import (
	"xiaomi-dnsmasq-gui/cmd/api/config"
	uciutil "xiaomi-dnsmasq-gui/pkg/uci"
)

const (
	validationErrorCode   = "VALIDATION_ERROR"
	badMethodErrorCode    = "BAD_METHOD"
	badActionErrorCode    = "BAD_ACTION"
	unauthorizedErrorCode = "UNAUTHORIZED"
	authFailedErrorCode   = "AUTH_FAILED"
	internalErrorCode     = "INTERNAL_ERROR"
)

var (
	appConfig                   = config.Default()
	uciClient uciutil.UCIClient = uciutil.NewClient()
)

type APIHandler struct{}

func NewAPIHandler(cfg config.AppConfig, client uciutil.UCIClient) *APIHandler {
	appConfig = cfg
	if client != nil {
		uciClient = client
	}
	return &APIHandler{}
}

type apiResponse struct {
	OK      bool        `json:"ok"`
	Code    string      `json:"code,omitempty"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type codedError struct {
	Code    string
	Message string
}

func (e *codedError) Error() string {
	return e.Message
}

func newCodedError(code, message string) error {
	return &codedError{Code: code, Message: message}
}

type dhcpState struct {
	Enabled     bool   `json:"enabled"`
	State       string `json:"state"`
	ToggleTo    string `json:"toggleTo"`
	ToggleLabel string `json:"toggleLabel"`
}

type defaultsState struct {
	Gateway string `json:"gateway"`
	DNS     string `json:"dns"`
}

type staticLease struct {
	Sec     string `json:"sec"`
	Name    string `json:"name"`
	MAC     string `json:"mac"`
	IP      string `json:"ip"`
	Tag     string `json:"tag"`
	Gateway string `json:"gateway"`
	DNS     string `json:"dns"`
}

type tagTemplate struct {
	Sec     string `json:"sec"`
	Tag     string `json:"tag"`
	Gateway string `json:"gateway"`
	DNS     string `json:"dns"`
	InUse   bool   `json:"inUse"`
}

type dynamicLease struct {
	Hostname string `json:"hostname"`
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Type     string `json:"type"`
	Remain   string `json:"remain"`
	IsStatic bool   `json:"isStatic"`
}

type dynamicState struct {
	LANPrefix string         `json:"lanPrefix"`
	LAN       []dynamicLease `json:"lan"`
	Other     []dynamicLease `json:"other"`
}

type statePayload struct {
	DHCP         dhcpState     `json:"dhcp"`
	Defaults     defaultsState `json:"defaults"`
	StaticLeases []staticLease `json:"staticLeases"`
	Templates    []tagTemplate `json:"templates"`
	Dynamic      dynamicState  `json:"dynamic"`
}

type authStatusPayload struct {
	Authenticated   bool   `json:"authenticated"`
	DefaultPassword bool   `json:"defaultPassword"`
	AuthFile        string `json:"authFile"`
}
