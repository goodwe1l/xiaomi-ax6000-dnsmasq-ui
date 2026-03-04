package uci

import "strings"

func ParseOptionValues(raw string) []string {
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

func GetSectionOptionValue(client UCIClient, sec, code string) (string, error) {
	if strings.TrimSpace(sec) == "" {
		return "", nil
	}
	raw, ok, err := client.Get("dhcp." + sec + ".dhcp_option")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	result := ""
	for _, item := range ParseOptionValues(raw) {
		prefix := code + ","
		if strings.HasPrefix(item, prefix) {
			result = strings.TrimPrefix(item, prefix)
		}
	}
	return result, nil
}

func GetTagOptionValue(client UCIClient, configName, tagName, code string) (string, error) {
	sec, found, err := ResolveTagSec(client, configName, tagName)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	return GetSectionOptionValue(client, sec, code)
}

func GetTagLabelFromHostTag(client UCIClient, configName, hostTag string) (string, error) {
	hostTag = strings.TrimSpace(hostTag)
	if hostTag == "" {
		return "", nil
	}
	sec, found, err := ResolveTagSec(client, configName, hostTag)
	if err != nil {
		return "", err
	}
	if !found {
		return hostTag, nil
	}
	label, ok, err := client.Get("dhcp." + sec + ".tag")
	if err != nil {
		return "", err
	}
	if ok && label != "" {
		return label, nil
	}
	return hostTag, nil
}
