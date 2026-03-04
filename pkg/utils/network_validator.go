package utils

import (
	"regexp"
	"strings"
)

var (
	macRegexp  = regexp.MustCompile(`^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$`)
	ipv4Regexp = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,3}){3}$`)
)

func NormalizeMAC(mac string) string {
	return strings.ToLower(strings.TrimSpace(mac))
}

func ValidMAC(mac string) bool {
	return macRegexp.MatchString(mac)
}

func ValidIPv4(ip string) bool {
	return ipv4Regexp.MatchString(ip)
}

func NormalizeDNS(raw string) string {
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

func ValidateDNSList(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return true
	}
	for _, part := range strings.Split(raw, ",") {
		ip := strings.TrimSpace(part)
		if ip == "" || !ValidIPv4(ip) {
			return false
		}
	}
	return true
}
