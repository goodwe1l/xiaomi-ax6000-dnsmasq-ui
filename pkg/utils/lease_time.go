package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func FormatLeaseExpire(exp string) string {
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
