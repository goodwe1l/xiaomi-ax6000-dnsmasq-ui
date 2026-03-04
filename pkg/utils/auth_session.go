package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func LoadAuthPassword(authFilePath, defaultAuthPassword string) (string, error) {
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

func GenerateSessionToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d_%d", time.Now().Unix(), os.Getpid())
	}
	return hex.EncodeToString(buf)
}

func WriteSession(sessionFilePath, token string, ttlSeconds int64) error {
	expire := time.Now().Unix() + ttlSeconds
	content := fmt.Sprintf("%s %d\n", token, expire)
	return os.WriteFile(sessionFilePath, []byte(content), 0o600)
}

func ClearSession(sessionFilePath string) {
	_ = os.Remove(sessionFilePath)
}

func ReadSession(sessionFilePath string) (string, int64, error) {
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

func IsAuthenticated(r *http.Request, sessionCookieName, sessionFilePath string) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie == nil || cookie.Value == "" {
		return false
	}
	token, expire, err := ReadSession(sessionFilePath)
	if err != nil {
		return false
	}
	if cookie.Value != token {
		return false
	}
	if expire <= time.Now().Unix() {
		ClearSession(sessionFilePath)
		return false
	}
	return true
}
