package handler

import (
	"encoding/json"
	"errors"
	"net/http"
)

func (h *APIHandler) HandleUnsupportedAction(w http.ResponseWriter, action string) {
	respondError(w, badActionErrorCode, "不支持的 action: "+action)
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
