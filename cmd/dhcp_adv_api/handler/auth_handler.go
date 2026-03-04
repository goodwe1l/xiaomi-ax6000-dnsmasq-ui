package handler

import (
	"net/http"
	"os"
	"strings"

	"dhcp_adv/pkg/utils"
)

func (h *APIHandler) IsAuthenticated(r *http.Request) bool {
	return utils.IsAuthenticated(r, appConfig.Auth.SessionCookieName, appConfig.Paths.SessionFilePath)
}

func (h *APIHandler) HandleUnauthorized(w http.ResponseWriter, _ *http.Request) {
	respondError(w, unauthorizedErrorCode, "未登录或会话已过期，请先登录")
}

func (h *APIHandler) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	respondOK(w, "获取认证状态成功", buildAuthStatusPayload(r))
}

func (h *APIHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := requirePost(r); err != nil {
		respondErrorFromErr(w, err)
		return
	}
	if err := applyLogin(w, strings.TrimSpace(r.FormValue("password"))); err != nil {
		respondErrorFromErr(w, err)
		return
	}
	respondOK(w, "登录成功", map[string]interface{}{})
}

func (h *APIHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if err := requirePost(r); err != nil {
		respondErrorFromErr(w, err)
		return
	}
	if err := applyLogout(w); err != nil {
		respondErrorFromErr(w, err)
		return
	}
	respondOK(w, "已退出登录", map[string]interface{}{})
}

func buildAuthStatusPayload(r *http.Request) authStatusPayload {
	_, statErr := os.Stat(appConfig.Paths.AuthFilePath)
	defaultPassword := statErr != nil
	return authStatusPayload{
		Authenticated:   utils.IsAuthenticated(r, appConfig.Auth.SessionCookieName, appConfig.Paths.SessionFilePath),
		DefaultPassword: defaultPassword,
		AuthFile:        appConfig.Paths.AuthFilePath,
	}
}

func applyLogin(w http.ResponseWriter, password string) error {
	if password == "" {
		return newCodedError(authFailedErrorCode, "密码不能为空")
	}
	authPassword, err := utils.LoadAuthPassword(appConfig.Paths.AuthFilePath, appConfig.Auth.DefaultAuthPassword)
	if err != nil {
		return err
	}
	if password != authPassword {
		return newCodedError(authFailedErrorCode, "密码错误")
	}
	token := utils.GenerateSessionToken()
	if err := utils.WriteSession(appConfig.Paths.SessionFilePath, token, appConfig.Auth.SessionTTLSeconds); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     appConfig.Auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(appConfig.Auth.SessionTTLSeconds),
	})
	return nil
}

func applyLogout(w http.ResponseWriter) error {
	utils.ClearSession(appConfig.Paths.SessionFilePath)
	http.SetCookie(w, &http.Cookie{
		Name:     appConfig.Auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	return nil
}
