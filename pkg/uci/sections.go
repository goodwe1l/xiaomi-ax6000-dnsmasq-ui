package uci

import (
	"bufio"
	"strings"
)

func ListSectionsByType(client UCIClient, configName, sectionType string) ([]string, error) {
	out, err := client.Show(configName)
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

func SectionExists(client UCIClient, section string) (bool, error) {
	_, err := client.Show("dhcp." + section)
	if err != nil {
		if IsExitCode(err, 1) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func FindHostSecByMAC(client UCIClient, configName, targetMAC string) (string, bool, error) {
	target := normalizeMAC(targetMAC)
	sections, err := ListSectionsByType(client, configName, "host")
	if err != nil {
		return "", false, err
	}
	for _, sec := range sections {
		mac, ok, err := client.Get("dhcp." + sec + ".mac")
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

func FindTagSecByTagName(client UCIClient, configName, tagName string) (string, bool, error) {
	target := strings.TrimSpace(tagName)
	if target == "" {
		return "", false, nil
	}
	sections, err := ListSectionsByType(client, configName, "tag")
	if err != nil {
		return "", false, err
	}
	for _, sec := range sections {
		tag, ok, err := client.Get("dhcp." + sec + ".tag")
		if err != nil {
			return "", false, err
		}
		if ok && tag == target {
			return sec, true, nil
		}
	}
	return "", false, nil
}

func IsTagSection(client UCIClient, sec string) (bool, error) {
	if sec == "" {
		return false, nil
	}
	show, err := client.Show("dhcp." + sec)
	if err != nil {
		if IsExitCode(err, 1) {
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

func ResolveTagSec(client UCIClient, configName, key string) (string, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false, nil
	}
	isSec, err := IsTagSection(client, key)
	if err != nil {
		return "", false, err
	}
	if isSec {
		return key, true, nil
	}
	return FindTagSecByTagName(client, configName, key)
}

func DeleteAdvTagByName(client UCIClient, configName, target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	isSec, err := IsTagSection(client, target)
	if err != nil {
		return err
	}
	if isSec {
		tagName, _, err := client.Get("dhcp." + target + ".tag")
		if err != nil {
			return err
		}
		if strings.HasPrefix(tagName, "adv_") {
			return client.Delete("dhcp." + target)
		}
		return nil
	}
	if !strings.HasPrefix(target, "adv_") {
		return nil
	}
	sec, found, err := FindTagSecByTagName(client, configName, target)
	if err != nil {
		return err
	}
	if found {
		return client.Delete("dhcp." + sec)
	}
	return nil
}

func BuildStaticMACSet(client UCIClient, configName string) (map[string]struct{}, error) {
	set := make(map[string]struct{})
	sections, err := ListSectionsByType(client, configName, "host")
	if err != nil {
		return nil, err
	}
	for _, sec := range sections {
		mac, ok, err := client.Get("dhcp." + sec + ".mac")
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

func TemplateInUse(client UCIClient, configName, sec, label string) (bool, error) {
	hostSections, err := ListSectionsByType(client, configName, "host")
	if err != nil {
		return false, err
	}
	for _, hostSec := range hostSections {
		htag, ok, err := client.Get("dhcp." + hostSec + ".tag")
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

func normalizeMAC(mac string) string {
	return strings.ToLower(strings.TrimSpace(mac))
}
