package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	authFilePath          = "/data/dhcp_adv/auth.conf"
	sessionFilePath       = "/tmp/dhcp_adv_session"
	sessionCookieName     = "dhcp_adv_session"
	sessionTTLSeconds     = 3600
	defaultAuthPassword   = "admin123456"
	stateSectionDHCP      = "dhcp"
	validationErrorCode   = "VALIDATION_ERROR"
	badMethodErrorCode    = "BAD_METHOD"
	badActionErrorCode    = "BAD_ACTION"
	unauthorizedErrorCode = "UNAUTHORIZED"
	authFailedErrorCode   = "AUTH_FAILED"
	internalErrorCode     = "INTERNAL_ERROR"
)

var (
	macRegexp  = regexp.MustCompile(`^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$`)
	ipv4Regexp = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,3}){3}$`)
)

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

func main() {
	if err := cgi.Serve(http.HandlerFunc(handleRequest)); err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "Status: 500 Internal Server Error\r\nContent-Type: application/json; charset=UTF-8\r\n\r\n")
		_ = json.NewEncoder(os.Stdout).Encode(apiResponse{OK: false, Code: internalErrorCode, Message: err.Error()})
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	action := strings.TrimSpace(r.FormValue("action"))
	if action == "" {
		action = "get_state"
	}

	switch action {
	case "auth_status":
		respondOK(w, "获取认证状态成功", buildAuthStatusPayload(r))
		return
	case "login":
		if err := requirePost(r); err != nil {
			respondErrorFromErr(w, err)
			return
		}
		if err := applyLogin(w, strings.TrimSpace(r.FormValue("password"))); err != nil {
			respondErrorFromErr(w, err)
			return
		}
		respondOK(w, "登录成功", map[string]interface{}{})
		return
	case "logout":
		if err := requirePost(r); err != nil {
			respondErrorFromErr(w, err)
			return
		}
		if err := applyLogout(w); err != nil {
			respondErrorFromErr(w, err)
			return
		}
		respondOK(w, "已退出登录", map[string]interface{}{})
		return
	}

	if !isAuthenticated(r) {
		respondError(w, unauthorizedErrorCode, "未登录或会话已过期，请先登录")
		return
	}

	switch action {
	case "get_state":
		payload, err := buildStatePayload()
		if err != nil {
			respondErrorFromErr(w, err)
			return
		}
		respondOK(w, "获取成功", payload)
	case "toggle_dhcp":
		if err := requirePost(r); err != nil {
			respondErrorFromErr(w, err)
			return
		}
		msg, err := applyToggle(strings.TrimSpace(r.FormValue("enable")))
		if err != nil {
			respondErrorFromErr(w, err)
			return
		}
		respondOK(w, msg, map[string]interface{}{})
	case "save_default":
		if err := requirePost(r); err != nil {
			respondErrorFromErr(w, err)
			return
		}
		msg, err := applySaveDefault(strings.TrimSpace(r.FormValue("default_gateway")), strings.TrimSpace(r.FormValue("default_dns")))
		if err != nil {
			respondErrorFromErr(w, err)
			return
		}
		respondOK(w, msg, map[string]interface{}{})
	case "lease_upsert":
		if err := requirePost(r); err != nil {
			respondErrorFromErr(w, err)
			return
		}
		msg, err := applyLeaseUpsert(r)
		if err != nil {
			respondErrorFromErr(w, err)
			return
		}
		respondOK(w, msg, map[string]interface{}{})
	case "lease_delete":
		if err := requirePost(r); err != nil {
			respondErrorFromErr(w, err)
			return
		}
		msg, err := applyLeaseDelete(strings.TrimSpace(r.FormValue("mac")))
		if err != nil {
			respondErrorFromErr(w, err)
			return
		}
		respondOK(w, msg, map[string]interface{}{})
	case "template_upsert":
		if err := requirePost(r); err != nil {
			respondErrorFromErr(w, err)
			return
		}
		msg, err := applyTemplateUpsert(
			strings.TrimSpace(r.FormValue("template_sec")),
			strings.TrimSpace(r.FormValue("template_tag")),
			strings.TrimSpace(r.FormValue("template_gateway")),
			strings.TrimSpace(r.FormValue("template_dns")),
		)
		if err != nil {
			respondErrorFromErr(w, err)
			return
		}
		respondOK(w, msg, map[string]interface{}{})
	case "template_delete":
		if err := requirePost(r); err != nil {
			respondErrorFromErr(w, err)
			return
		}
		msg, err := applyTemplateDelete(strings.TrimSpace(r.FormValue("template_sec")))
		if err != nil {
			respondErrorFromErr(w, err)
			return
		}
		respondOK(w, msg, map[string]interface{}{})
	default:
		respondError(w, badActionErrorCode, "不支持的 action: "+action)
	}
}

func respondOK(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-store")
	resp := apiResponse{OK: true, Message: message, Data: data}
	_ = json.NewEncoder(w).Encode(resp)
}

func respondError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-store")
	resp := apiResponse{OK: false, Code: code, Message: message}
	_ = json.NewEncoder(w).Encode(resp)
}

func respondErrorFromErr(w http.ResponseWriter, err error) {
	var ce *codedError
	if errors.As(err, &ce) {
		respondError(w, ce.Code, ce.Message)
		return
	}
	respondError(w, internalErrorCode, "服务内部错误")
}

func requirePost(r *http.Request) error {
	if r.Method != http.MethodPost {
		return newCodedError(badMethodErrorCode, "该接口仅支持 POST")
	}
	return nil
}

func buildAuthStatusPayload(r *http.Request) authStatusPayload {
	_, statErr := os.Stat(authFilePath)
	defaultPassword := statErr != nil
	return authStatusPayload{
		Authenticated:   isAuthenticated(r),
		DefaultPassword: defaultPassword,
		AuthFile:        authFilePath,
	}
}

func loadAuthPassword() (string, error) {
	content, err := os.ReadFile(authFilePath)
	if err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "password=") {
				value := strings.TrimSpace(strings.TrimPrefix(line, "password="))
				if value != "" {
					return value, nil
				}
			}
		}
		return defaultAuthPassword, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	if mkErr := os.MkdirAll(filepath.Dir(authFilePath), 0o755); mkErr == nil {
		_ = os.WriteFile(authFilePath, []byte("password="+defaultAuthPassword+"\n"), 0o600)
	}
	return defaultAuthPassword, nil
}

func generateSessionToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d_%d", time.Now().Unix(), os.Getpid())
	}
	return hex.EncodeToString(buf)
}

func writeSession(token string) error {
	expire := time.Now().Unix() + sessionTTLSeconds
	content := fmt.Sprintf("%s %d\n", token, expire)
	return os.WriteFile(sessionFilePath, []byte(content), 0o600)
}

func clearSession() {
	_ = os.Remove(sessionFilePath)
}

func readSession() (string, int64, error) {
	content, err := os.ReadFile(sessionFilePath)
	if err != nil {
		return "", 0, err
	}
	parts := strings.Fields(string(content))
	if len(parts) < 2 {
		return "", 0, errors.New("invalid session")
	}
	expire, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, err
	}
	return parts[0], expire, nil
}

func isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie == nil || cookie.Value == "" {
		return false
	}
	token, expire, err := readSession()
	if err != nil {
		return false
	}
	if cookie.Value != token {
		return false
	}
	if expire <= time.Now().Unix() {
		clearSession()
		return false
	}
	return true
}

func applyLogin(w http.ResponseWriter, password string) error {
	if password == "" {
		return newCodedError(authFailedErrorCode, "密码不能为空")
	}
	authPassword, err := loadAuthPassword()
	if err != nil {
		return err
	}
	if password != authPassword {
		return newCodedError(authFailedErrorCode, "密码错误")
	}
	token := generateSessionToken()
	if err := writeSession(token); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionTTLSeconds,
	})
	return nil
}

func applyLogout(w http.ResponseWriter) error {
	clearSession()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	return nil
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if err != nil {
		return result, err
	}
	return result, nil
}

func isExitCode(err error, code int) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == code
	}
	return false
}

func uciGet(key string) (string, bool, error) {
	out, err := runCommand("uci", "-q", "get", key)
	if err != nil {
		if isExitCode(err, 1) {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(out), true, nil
}

func uciShow(target string) (string, error) {
	return runCommand("uci", "-q", "show", target)
}

func uciSet(key, value string) error {
	_, err := runCommand("uci", "set", fmt.Sprintf("%s=%s", key, value))
	return err
}

func uciDelete(key string) error {
	_, err := runCommand("uci", "-q", "delete", key)
	if err != nil && !isExitCode(err, 1) {
		return err
	}
	return nil
}

func uciAddList(key, value string) error {
	_, err := runCommand("uci", "add_list", fmt.Sprintf("%s=%s", key, value))
	return err
}

func uciCommit(config string) error {
	_, err := runCommand("uci", "commit", config)
	return err
}

func restartDNSMasq() error {
	_, err := runCommand("/etc/init.d/dnsmasq", "restart")
	return err
}

func listSectionsByType(sectionType string) ([]string, error) {
	out, err := uciShow(stateSectionDHCP)
	if err != nil {
		return nil, err
	}
	sections := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "dhcp.") {
			continue
		}
		rest := strings.TrimPrefix(line, "dhcp.")
		parts := strings.SplitN(rest, "=", 2)
		if len(parts) != 2 {
			continue
		}
		lhs := strings.TrimSpace(parts[0])
		typ := strings.TrimSpace(parts[1])
		if typ != sectionType {
			continue
		}
		if strings.Contains(lhs, ".") {
			continue
		}
		sections = append(sections, lhs)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return sections, nil
}

func normalizeMAC(mac string) string {
	return strings.ToLower(strings.TrimSpace(mac))
}

func validMAC(mac string) bool {
	return macRegexp.MatchString(mac)
}

func validIPv4(ip string) bool {
	return ipv4Regexp.MatchString(ip)
}

func normalizeDNS(raw string) string {
	raw = strings.ReplaceAll(raw, " ", ",")
	parts := strings.Split(raw, ",")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			clean = append(clean, p)
		}
	}
	return strings.Join(clean, ",")
}

func validateDNSList(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return true
	}
	for _, part := range strings.Split(raw, ",") {
		ip := strings.TrimSpace(part)
		if ip == "" || !validIPv4(ip) {
			return false
		}
	}
	return true
}

func sanitizeTagKey(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return "custom"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range text {
		keep := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || r == '_'
		if keep {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "custom"
	}
	return result
}

func sanitizeMACID(mac string) string {
	mac = strings.ReplaceAll(strings.ToLower(mac), ":", "_")
	var b strings.Builder
	for _, r := range mac {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sectionExists(section string) (bool, error) {
	_, err := uciShow("dhcp." + section)
	if err != nil {
		if isExitCode(err, 1) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func findHostSecByMAC(targetMAC string) (string, bool, error) {
	target := normalizeMAC(targetMAC)
	sections, err := listSectionsByType("host")
	if err != nil {
		return "", false, err
	}
	for _, sec := range sections {
		mac, ok, err := uciGet("dhcp." + sec + ".mac")
		if err != nil {
			return "", false, err
		}
		if !ok {
			continue
		}
		if normalizeMAC(mac) == target {
			return sec, true, nil
		}
	}
	return "", false, nil
}

func findTagSecByTagName(tagName string) (string, bool, error) {
	target := strings.TrimSpace(tagName)
	if target == "" {
		return "", false, nil
	}
	sections, err := listSectionsByType("tag")
	if err != nil {
		return "", false, err
	}
	for _, sec := range sections {
		tag, ok, err := uciGet("dhcp." + sec + ".tag")
		if err != nil {
			return "", false, err
		}
		if ok && tag == target {
			return sec, true, nil
		}
	}
	return "", false, nil
}

func isTagSection(sec string) (bool, error) {
	if sec == "" {
		return false, nil
	}
	show, err := uciShow("dhcp." + sec)
	if err != nil {
		if isExitCode(err, 1) {
			return false, nil
		}
		return false, err
	}
	expectPrefix := "dhcp." + sec + "="
	scanner := bufio.NewScanner(strings.NewReader(show))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, expectPrefix) {
			sectionType := strings.TrimPrefix(line, expectPrefix)
			return sectionType == "tag", nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func resolveTagSec(key string) (string, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false, nil
	}
	isSec, err := isTagSection(key)
	if err != nil {
		return "", false, err
	}
	if isSec {
		return key, true, nil
	}
	return findTagSecByTagName(key)
}

func parseOptionValues(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Fields(raw)
	values := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), "'\"")
		if p != "" {
			values = append(values, p)
		}
	}
	return values
}

func getSectionOptionValue(sec string, code string) (string, error) {
	if strings.TrimSpace(sec) == "" {
		return "", nil
	}
	raw, ok, err := uciGet("dhcp." + sec + ".dhcp_option")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	result := ""
	for _, item := range parseOptionValues(raw) {
		prefix := code + ","
		if strings.HasPrefix(item, prefix) {
			result = strings.TrimPrefix(item, prefix)
		}
	}
	return result, nil
}

func getTagOptionValue(tagName string, code string) (string, error) {
	sec, found, err := resolveTagSec(tagName)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	return getSectionOptionValue(sec, code)
}

func getTagLabelFromHostTag(hostTag string) (string, error) {
	hostTag = strings.TrimSpace(hostTag)
	if hostTag == "" {
		return "", nil
	}
	sec, found, err := resolveTagSec(hostTag)
	if err != nil {
		return "", err
	}
	if !found {
		return hostTag, nil
	}
	label, ok, err := uciGet("dhcp." + sec + ".tag")
	if err != nil {
		return "", err
	}
	if ok && label != "" {
		return label, nil
	}
	return hostTag, nil
}

func deleteAdvTagByName(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	isSec, err := isTagSection(target)
	if err != nil {
		return err
	}
	if isSec {
		tagName, _, err := uciGet("dhcp." + target + ".tag")
		if err != nil {
			return err
		}
		if strings.HasPrefix(tagName, "adv_") {
			return uciDelete("dhcp." + target)
		}
		return nil
	}
	if !strings.HasPrefix(target, "adv_") {
		return nil
	}
	sec, found, err := findTagSecByTagName(target)
	if err != nil {
		return err
	}
	if found {
		return uciDelete("dhcp." + sec)
	}
	return nil
}

func buildStaticMACSet() (map[string]struct{}, error) {
	set := make(map[string]struct{})
	sections, err := listSectionsByType("host")
	if err != nil {
		return nil, err
	}
	for _, sec := range sections {
		mac, ok, err := uciGet("dhcp." + sec + ".mac")
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		mac = normalizeMAC(mac)
		if mac != "" {
			set[mac] = struct{}{}
		}
	}
	return set, nil
}

func formatLeaseExpire(exp string) string {
	exp = strings.TrimSpace(exp)
	if exp == "" {
		return "-"
	}
	if exp == "0" {
		return "永久"
	}
	for _, c := range exp {
		if c < '0' || c > '9' {
			return exp
		}
	}
	expireTs, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return exp
	}
	left := expireTs - time.Now().Unix()
	if left <= 0 {
		return "即将过期"
	}
	switch {
	case left >= 86400:
		return fmt.Sprintf("%d天", left/86400)
	case left >= 3600:
		return fmt.Sprintf("%d小时", left/3600)
	case left >= 60:
		return fmt.Sprintf("%d分钟", left/60)
	default:
		return fmt.Sprintf("%d秒", left)
	}
}

func applyToggle(enable string) (string, error) {
	switch enable {
	case "1":
		if err := uciSet("dhcp.lan.ignore", "0"); err != nil {
			return "", err
		}
		if err := uciCommit(stateSectionDHCP); err != nil {
			return "", err
		}
		if err := restartDNSMasq(); err != nil {
			return "", err
		}
		return "已开启 LAN DHCP", nil
	case "0":
		if err := uciSet("dhcp.lan.ignore", "1"); err != nil {
			return "", err
		}
		if err := uciCommit(stateSectionDHCP); err != nil {
			return "", err
		}
		if err := restartDNSMasq(); err != nil {
			return "", err
		}
		return "已关闭 LAN DHCP", nil
	default:
		return "", newCodedError(validationErrorCode, "切换失败：参数错误")
	}
}

func applySaveDefault(defaultGateway, defaultDNS string) (string, error) {
	defaultGateway = strings.TrimSpace(defaultGateway)
	defaultDNS = normalizeDNS(defaultDNS)
	if defaultGateway != "" && !validIPv4(defaultGateway) {
		return "", newCodedError(validationErrorCode, "默认网关格式不正确")
	}
	if !validateDNSList(defaultDNS) {
		return "", newCodedError(validationErrorCode, "默认 DNS 格式不正确")
	}
	if err := uciDelete("dhcp.lan.dhcp_option"); err != nil {
		return "", err
	}
	if defaultGateway != "" {
		if err := uciAddList("dhcp.lan.dhcp_option", "3,"+defaultGateway); err != nil {
			return "", err
		}
	}
	if defaultDNS != "" {
		if err := uciAddList("dhcp.lan.dhcp_option", "6,"+defaultDNS); err != nil {
			return "", err
		}
	}
	if err := uciCommit(stateSectionDHCP); err != nil {
		return "", err
	}
	if err := restartDNSMasq(); err != nil {
		return "", err
	}
	return "默认 DHCP 规则保存成功", nil
}

func applyLeaseUpsert(r *http.Request) (string, error) {
	name := strings.TrimSpace(r.FormValue("name"))
	mac := normalizeMAC(r.FormValue("mac"))
	ip := strings.TrimSpace(r.FormValue("ip"))
	gateway := strings.TrimSpace(r.FormValue("gateway"))
	dns := normalizeDNS(strings.TrimSpace(r.FormValue("dns")))
	tag := strings.TrimSpace(r.FormValue("tag"))

	if name == "" {
		return "", newCodedError(validationErrorCode, "设备名不能为空")
	}
	if !validMAC(mac) {
		return "", newCodedError(validationErrorCode, "MAC 格式不正确（示例：AA:BB:CC:DD:EE:FF）")
	}
	if !validIPv4(ip) {
		return "", newCodedError(validationErrorCode, "IP 格式不正确（示例：10.0.0.120）")
	}
	if gateway != "" && !validIPv4(gateway) {
		return "", newCodedError(validationErrorCode, "网关格式不正确")
	}
	if !validateDNSList(dns) {
		return "", newCodedError(validationErrorCode, "DNS 格式不正确")
	}

	macID := sanitizeMACID(mac)
	hostSec := "advh_" + macID
	tagSec := "advt_" + macID
	tagName := "adv_" + macID

	oldSec, foundOld, err := findHostSecByMAC(mac)
	if err != nil {
		return "", err
	}
	oldTag := ""
	if foundOld {
		if val, ok, err := uciGet("dhcp." + oldSec + ".tag"); err != nil {
			return "", err
		} else if ok {
			oldTag = val
		}
	}
	if foundOld && oldSec != hostSec {
		if err := uciDelete("dhcp." + oldSec); err != nil {
			return "", err
		}
		if err := deleteAdvTagByName(oldTag); err != nil {
			return "", err
		}
	}

	if err := uciSet("dhcp."+hostSec, "host"); err != nil {
		return "", err
	}
	if err := uciSet("dhcp."+hostSec+".name", name); err != nil {
		return "", err
	}
	if err := uciSet("dhcp."+hostSec+".mac", mac); err != nil {
		return "", err
	}
	if err := uciSet("dhcp."+hostSec+".ip", ip); err != nil {
		return "", err
	}

	if tag != "" {
		tSec, foundTagSec, err := resolveTagSec(tag)
		if err != nil {
			return "", err
		}
		if !foundTagSec {
			if gateway == "" && dns == "" {
				return "", newCodedError(validationErrorCode, "标签不存在，请从模板选择，或同时填写网关/DNS创建新标签")
			}
			safeTag := sanitizeTagKey(tag)
			tSec = "tag_" + safeTag
			idx := 0
			for {
				exists, err := sectionExists(tSec)
				if err != nil {
					return "", err
				}
				if !exists {
					break
				}
				idx++
				tSec = fmt.Sprintf("tag_%s_%d", safeTag, idx)
			}
			if err := uciSet("dhcp."+tSec, "tag"); err != nil {
				return "", err
			}
			if err := uciSet("dhcp."+tSec+".tag", tag); err != nil {
				return "", err
			}
		}

		if err := uciSet("dhcp."+hostSec+".tag", tSec); err != nil {
			return "", err
		}
		if err := uciDelete("dhcp." + hostSec + ".adv_gateway"); err != nil {
			return "", err
		}
		if err := uciDelete("dhcp." + hostSec + ".adv_dns"); err != nil {
			return "", err
		}
		if err := deleteAdvTagByName(oldTag); err != nil {
			return "", err
		}
		if gateway != "" || dns != "" {
			if err := uciDelete("dhcp." + tSec + ".dhcp_option"); err != nil {
				return "", err
			}
			if gateway != "" {
				if err := uciAddList("dhcp."+tSec+".dhcp_option", "3,"+gateway); err != nil {
					return "", err
				}
			}
			if dns != "" {
				if err := uciAddList("dhcp."+tSec+".dhcp_option", "6,"+dns); err != nil {
					return "", err
				}
			}
		}
		if err := uciDelete("dhcp." + tagSec); err != nil {
			return "", err
		}
	} else if gateway != "" || dns != "" {
		if err := uciSet("dhcp."+hostSec+".tag", tagSec); err != nil {
			return "", err
		}
		if err := uciSet("dhcp."+hostSec+".adv_gateway", gateway); err != nil {
			return "", err
		}
		if err := uciSet("dhcp."+hostSec+".adv_dns", dns); err != nil {
			return "", err
		}
		if err := uciSet("dhcp."+tagSec, "tag"); err != nil {
			return "", err
		}
		if err := uciSet("dhcp."+tagSec+".tag", tagName); err != nil {
			return "", err
		}
		if err := uciDelete("dhcp." + tagSec + ".dhcp_option"); err != nil {
			return "", err
		}
		if gateway != "" {
			if err := uciAddList("dhcp."+tagSec+".dhcp_option", "3,"+gateway); err != nil {
				return "", err
			}
		}
		if dns != "" {
			if err := uciAddList("dhcp."+tagSec+".dhcp_option", "6,"+dns); err != nil {
				return "", err
			}
		}
	} else {
		if err := uciDelete("dhcp." + hostSec + ".tag"); err != nil {
			return "", err
		}
		if err := uciDelete("dhcp." + hostSec + ".adv_gateway"); err != nil {
			return "", err
		}
		if err := uciDelete("dhcp." + hostSec + ".adv_dns"); err != nil {
			return "", err
		}
		if err := uciDelete("dhcp." + tagSec); err != nil {
			return "", err
		}
		if err := deleteAdvTagByName(oldTag); err != nil {
			return "", err
		}
	}

	if err := uciCommit(stateSectionDHCP); err != nil {
		return "", err
	}
	if err := restartDNSMasq(); err != nil {
		return "", err
	}
	return "保存成功", nil
}

func applyLeaseDelete(mac string) (string, error) {
	mac = normalizeMAC(mac)
	if !validMAC(mac) {
		return "", newCodedError(validationErrorCode, "删除失败：MAC 格式不正确")
	}
	sec, found, err := findHostSecByMAC(mac)
	if err != nil {
		return "", err
	}
	if !found {
		return "", newCodedError(validationErrorCode, "删除失败：未找到该设备")
	}
	htag, _, err := uciGet("dhcp." + sec + ".tag")
	if err != nil {
		return "", err
	}
	if err := uciDelete("dhcp." + sec); err != nil {
		return "", err
	}
	if err := deleteAdvTagByName(htag); err != nil {
		return "", err
	}
	if err := uciCommit(stateSectionDHCP); err != nil {
		return "", err
	}
	if err := restartDNSMasq(); err != nil {
		return "", err
	}
	return "删除成功", nil
}

func templateInUse(sec, label string) (bool, error) {
	hostSections, err := listSectionsByType("host")
	if err != nil {
		return false, err
	}
	for _, hostSec := range hostSections {
		htag, ok, err := uciGet("dhcp." + hostSec + ".tag")
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}
		if htag == sec || htag == label {
			return true, nil
		}
	}
	return false, nil
}

func applyTemplateUpsert(templateSec, templateTag, templateGateway, templateDNS string) (string, error) {
	tname := strings.TrimSpace(templateTag)
	tgw := strings.TrimSpace(templateGateway)
	tdns := normalizeDNS(templateDNS)

	if tname == "" {
		return "", newCodedError(validationErrorCode, "标签名不能为空")
	}
	if strings.HasPrefix(tname, "adv_") {
		return "", newCodedError(validationErrorCode, "标签名不能以 adv_ 开头")
	}
	if tgw != "" && !validIPv4(tgw) {
		return "", newCodedError(validationErrorCode, "标签网关格式不正确")
	}
	if !validateDNSList(tdns) {
		return "", newCodedError(validationErrorCode, "标签 DNS 格式不正确")
	}

	var tsec string
	if templateSec != "" {
		isSec, err := isTagSection(templateSec)
		if err != nil {
			return "", err
		}
		if !isSec {
			return "", newCodedError(validationErrorCode, "标签模板不存在")
		}
		oldName, _, err := uciGet("dhcp." + templateSec + ".tag")
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(oldName, "adv_") {
			return "", newCodedError(validationErrorCode, "系统自动标签不可修改")
		}
		existingSec, found, err := resolveTagSec(tname)
		if err != nil {
			return "", err
		}
		if found && existingSec != templateSec {
			return "", newCodedError(validationErrorCode, "标签名已存在")
		}
		tsec = templateSec
	} else {
		existingSec, found, err := resolveTagSec(tname)
		if err != nil {
			return "", err
		}
		if found {
			tsec = existingSec
		} else {
			safe := sanitizeTagKey(tname)
			tsec = "tag_" + safe
			idx := 0
			for {
				exists, err := sectionExists(tsec)
				if err != nil {
					return "", err
				}
				if !exists {
					break
				}
				idx++
				tsec = fmt.Sprintf("tag_%s_%d", safe, idx)
			}
			if err := uciSet("dhcp."+tsec, "tag"); err != nil {
				return "", err
			}
		}
	}

	if err := uciSet("dhcp."+tsec+".tag", tname); err != nil {
		return "", err
	}
	if err := uciDelete("dhcp." + tsec + ".dhcp_option"); err != nil {
		return "", err
	}
	if tgw != "" {
		if err := uciAddList("dhcp."+tsec+".dhcp_option", "3,"+tgw); err != nil {
			return "", err
		}
	}
	if tdns != "" {
		if err := uciAddList("dhcp."+tsec+".dhcp_option", "6,"+tdns); err != nil {
			return "", err
		}
	}

	if err := uciCommit(stateSectionDHCP); err != nil {
		return "", err
	}
	if err := restartDNSMasq(); err != nil {
		return "", err
	}
	return "标签模板保存成功", nil
}

func applyTemplateDelete(templateSec string) (string, error) {
	sec := strings.TrimSpace(templateSec)
	isSec, err := isTagSection(sec)
	if err != nil {
		return "", err
	}
	if !isSec {
		return "", newCodedError(validationErrorCode, "删除失败：标签模板不存在")
	}
	label, _, err := uciGet("dhcp." + sec + ".tag")
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(label, "adv_") {
		return "", newCodedError(validationErrorCode, "删除失败：系统自动标签不可删除")
	}
	inUse, err := templateInUse(sec, label)
	if err != nil {
		return "", err
	}
	if inUse {
		return "", newCodedError(validationErrorCode, "删除失败：还有静态租约正在使用该标签")
	}
	if err := uciDelete("dhcp." + sec); err != nil {
		return "", err
	}
	if err := uciCommit(stateSectionDHCP); err != nil {
		return "", err
	}
	if err := restartDNSMasq(); err != nil {
		return "", err
	}
	return "标签模板删除成功", nil
}

func buildStaticLeases() ([]staticLease, error) {
	hostSections, err := listSectionsByType("host")
	if err != nil {
		return nil, err
	}
	leases := make([]staticLease, 0)
	for _, sec := range hostSections {
		hname, _, err := uciGet("dhcp." + sec + ".name")
		if err != nil {
			return nil, err
		}
		hmac, okMAC, err := uciGet("dhcp." + sec + ".mac")
		if err != nil {
			return nil, err
		}
		if !okMAC || strings.TrimSpace(hmac) == "" {
			continue
		}
		hip, _, err := uciGet("dhcp." + sec + ".ip")
		if err != nil {
			return nil, err
		}
		hgw, _, err := uciGet("dhcp." + sec + ".adv_gateway")
		if err != nil {
			return nil, err
		}
		hdns, _, err := uciGet("dhcp." + sec + ".adv_dns")
		if err != nil {
			return nil, err
		}
		htag, _, err := uciGet("dhcp." + sec + ".tag")
		if err != nil {
			return nil, err
		}

		if hgw == "" && htag != "" {
			if value, err := getTagOptionValue(htag, "3"); err != nil {
				return nil, err
			} else {
				hgw = value
			}
		}
		if hdns == "" && htag != "" {
			if value, err := getTagOptionValue(htag, "6"); err != nil {
				return nil, err
			} else {
				hdns = value
			}
		}

		tagShow := ""
		if htag != "" {
			if label, err := getTagLabelFromHostTag(htag); err != nil {
				return nil, err
			} else {
				tagShow = label
			}
			if strings.HasPrefix(tagShow, "adv_") {
				tagShow = ""
			}
		}

		leases = append(leases, staticLease{
			Sec:     sec,
			Name:    hname,
			MAC:     hmac,
			IP:      hip,
			Tag:     tagShow,
			Gateway: hgw,
			DNS:     hdns,
		})
	}
	return leases, nil
}

func buildTemplates() ([]tagTemplate, error) {
	tagSections, err := listSectionsByType("tag")
	if err != nil {
		return nil, err
	}
	templates := make([]tagTemplate, 0)
	for _, sec := range tagSections {
		tname, ok, err := uciGet("dhcp." + sec + ".tag")
		if err != nil {
			return nil, err
		}
		if !ok || tname == "" || strings.HasPrefix(tname, "adv_") {
			continue
		}
		tgw, err := getSectionOptionValue(sec, "3")
		if err != nil {
			return nil, err
		}
		tdns, err := getSectionOptionValue(sec, "6")
		if err != nil {
			return nil, err
		}
		inUse, err := templateInUse(sec, tname)
		if err != nil {
			return nil, err
		}
		templates = append(templates, tagTemplate{
			Sec:     sec,
			Tag:     tname,
			Gateway: tgw,
			DNS:     tdns,
			InUse:   inUse,
		})
	}
	return templates, nil
}

func buildDynamicLeases(target, lanPrefix string, staticSet map[string]struct{}) ([]dynamicLease, error) {
	file, err := os.Open("/tmp/dhcp.leases")
	if err != nil {
		if os.IsNotExist(err) {
			return []dynamicLease{}, nil
		}
		return nil, err
	}
	defer file.Close()

	rows := make([]dynamicLease, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}
		exp := parts[0]
		mac := normalizeMAC(parts[1])
		ip := parts[2]
		host := parts[3]
		if host == "*" {
			host = ""
		}

		isLAN := lanPrefix != "" && strings.HasPrefix(ip, lanPrefix+".")
		if target == "lan" && !isLAN {
			continue
		}
		if target == "other" && isLAN {
			continue
		}

		_, isStatic := staticSet[mac]
		leaseType := "动态"
		if isStatic {
			leaseType = "静态保留"
		}

		rows = append(rows, dynamicLease{
			Hostname: host,
			MAC:      mac,
			IP:       ip,
			Type:     leaseType,
			Remain:   formatLeaseExpire(exp),
			IsStatic: isStatic,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func buildStatePayload() (statePayload, error) {
	payload := statePayload{}

	lanIgnore, ok, err := uciGet("dhcp.lan.ignore")
	if err != nil {
		return payload, err
	}
	if !ok {
		lanIgnore = "0"
	}
	if lanIgnore == "1" {
		payload.DHCP = dhcpState{
			Enabled:     false,
			State:       "关闭",
			ToggleTo:    "1",
			ToggleLabel: "开启 LAN DHCP",
		}
	} else {
		payload.DHCP = dhcpState{
			Enabled:     true,
			State:       "开启",
			ToggleTo:    "0",
			ToggleLabel: "关闭 LAN DHCP",
		}
	}

	defaultGateway, err := getSectionOptionValue("lan", "3")
	if err != nil {
		return payload, err
	}
	defaultDNS, err := getSectionOptionValue("lan", "6")
	if err != nil {
		return payload, err
	}
	payload.Defaults = defaultsState{
		Gateway: defaultGateway,
		DNS:     defaultDNS,
	}

	lanIP, _, err := uciGet("network.lan.ipaddr")
	if err != nil {
		return payload, err
	}
	lanPrefix := ""
	if idx := strings.LastIndex(lanIP, "."); idx > 0 {
		lanPrefix = lanIP[:idx]
	}

	staticSet, err := buildStaticMACSet()
	if err != nil {
		return payload, err
	}

	staticLeases, err := buildStaticLeases()
	if err != nil {
		return payload, err
	}
	templates, err := buildTemplates()
	if err != nil {
		return payload, err
	}
	dynamicLAN, err := buildDynamicLeases("lan", lanPrefix, staticSet)
	if err != nil {
		return payload, err
	}
	dynamicOther, err := buildDynamicLeases("other", lanPrefix, staticSet)
	if err != nil {
		return payload, err
	}

	payload.StaticLeases = staticLeases
	payload.Templates = templates
	payload.Dynamic = dynamicState{
		LANPrefix: lanPrefix,
		LAN:       dynamicLAN,
		Other:     dynamicOther,
	}

	return payload, nil
}
