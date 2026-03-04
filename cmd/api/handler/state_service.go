package handler

import (
	"bufio"
	"net/http"
	"os"
	"strings"

	uciutil "xiaomi-dnsmasq-gui/pkg/uci"
	"xiaomi-dnsmasq-gui/pkg/utils"
)

func (h *APIHandler) HandleGetState(w http.ResponseWriter, _ *http.Request) {
	payload, err := buildStatePayload()
	if err != nil {
		respondErrorFromErr(w, err)
		return
	}
	respondOK(w, "获取成功", payload)
}

func (h *APIHandler) HandleToggleDHCP(w http.ResponseWriter, r *http.Request) {
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
}

func (h *APIHandler) HandleSaveDefault(w http.ResponseWriter, r *http.Request) {
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
}

func applyToggle(enable string) (string, error) {
	switch enable {
	case "1":
		if err := uciClient.Set("dhcp.lan.ignore", "0"); err != nil {
			return "", err
		}
		if err := uciClient.Commit(appConfig.DHCP.ConfigName); err != nil {
			return "", err
		}
		if err := uciClient.RestartDNSMasq(); err != nil {
			return "", err
		}
		return "已开启 LAN DHCP", nil
	case "0":
		if err := uciClient.Set("dhcp.lan.ignore", "1"); err != nil {
			return "", err
		}
		if err := uciClient.Commit(appConfig.DHCP.ConfigName); err != nil {
			return "", err
		}
		if err := uciClient.RestartDNSMasq(); err != nil {
			return "", err
		}
		return "已关闭 LAN DHCP", nil
	default:
		return "", newCodedError(validationErrorCode, "切换失败：参数错误")
	}
}

func applySaveDefault(defaultGateway, defaultDNS string) (string, error) {
	defaultGateway = strings.TrimSpace(defaultGateway)
	defaultDNS = utils.NormalizeDNS(defaultDNS)
	if defaultGateway != "" && !utils.ValidIPv4(defaultGateway) {
		return "", newCodedError(validationErrorCode, "默认网关格式不正确")
	}
	if !utils.ValidateDNSList(defaultDNS) {
		return "", newCodedError(validationErrorCode, "默认 DNS 格式不正确")
	}
	if err := uciClient.Delete("dhcp.lan.dhcp_option"); err != nil {
		return "", err
	}
	if defaultGateway != "" {
		if err := uciClient.AddList("dhcp.lan.dhcp_option", "3,"+defaultGateway); err != nil {
			return "", err
		}
	}
	if defaultDNS != "" {
		if err := uciClient.AddList("dhcp.lan.dhcp_option", "6,"+defaultDNS); err != nil {
			return "", err
		}
	}
	if err := uciClient.Commit(appConfig.DHCP.ConfigName); err != nil {
		return "", err
	}
	if err := uciClient.RestartDNSMasq(); err != nil {
		return "", err
	}
	return "默认 DHCP 规则保存成功", nil
}

func buildStaticLeases() ([]staticLease, error) {
	hostSections, err := uciutil.ListSectionsByType(uciClient, appConfig.DHCP.ConfigName, "host")
	if err != nil {
		return nil, err
	}
	leases := make([]staticLease, 0)
	for _, sec := range hostSections {
		hname, _, err := uciClient.Get("dhcp." + sec + ".name")
		if err != nil {
			return nil, err
		}
		hmac, okMAC, err := uciClient.Get("dhcp." + sec + ".mac")
		if err != nil {
			return nil, err
		}
		if !okMAC || strings.TrimSpace(hmac) == "" {
			continue
		}
		hip, _, err := uciClient.Get("dhcp." + sec + ".ip")
		if err != nil {
			return nil, err
		}
		hgw, _, err := uciClient.Get("dhcp." + sec + ".adv_gateway")
		if err != nil {
			return nil, err
		}
		hdns, _, err := uciClient.Get("dhcp." + sec + ".adv_dns")
		if err != nil {
			return nil, err
		}
		htag, _, err := uciClient.Get("dhcp." + sec + ".tag")
		if err != nil {
			return nil, err
		}

		if hgw == "" && htag != "" {
			if value, err := uciutil.GetTagOptionValue(uciClient, appConfig.DHCP.ConfigName, htag, "3"); err != nil {
				return nil, err
			} else {
				hgw = value
			}
		}
		if hdns == "" && htag != "" {
			if value, err := uciutil.GetTagOptionValue(uciClient, appConfig.DHCP.ConfigName, htag, "6"); err != nil {
				return nil, err
			} else {
				hdns = value
			}
		}

		tagShow := ""
		if htag != "" {
			if label, err := uciutil.GetTagLabelFromHostTag(uciClient, appConfig.DHCP.ConfigName, htag); err != nil {
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
	tagSections, err := uciutil.ListSectionsByType(uciClient, appConfig.DHCP.ConfigName, "tag")
	if err != nil {
		return nil, err
	}
	templates := make([]tagTemplate, 0)
	for _, sec := range tagSections {
		tname, ok, err := uciClient.Get("dhcp." + sec + ".tag")
		if err != nil {
			return nil, err
		}
		if !ok || tname == "" || strings.HasPrefix(tname, "adv_") {
			continue
		}
		tgw, err := uciutil.GetSectionOptionValue(uciClient, sec, "3")
		if err != nil {
			return nil, err
		}
		tdns, err := uciutil.GetSectionOptionValue(uciClient, sec, "6")
		if err != nil {
			return nil, err
		}
		inUse, err := uciutil.TemplateInUse(uciClient, appConfig.DHCP.ConfigName, sec, tname)
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
		mac := utils.NormalizeMAC(parts[1])
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
			Remain:   utils.FormatLeaseExpire(exp),
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

	lanIgnore, ok, err := uciClient.Get("dhcp.lan.ignore")
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

	defaultGateway, err := uciutil.GetSectionOptionValue(uciClient, "lan", "3")
	if err != nil {
		return payload, err
	}
	defaultDNS, err := uciutil.GetSectionOptionValue(uciClient, "lan", "6")
	if err != nil {
		return payload, err
	}
	payload.Defaults = defaultsState{
		Gateway: defaultGateway,
		DNS:     defaultDNS,
	}

	lanIP, _, err := uciClient.Get("network.lan.ipaddr")
	if err != nil {
		return payload, err
	}
	lanPrefix := ""
	if idx := strings.LastIndex(lanIP, "."); idx > 0 {
		lanPrefix = lanIP[:idx]
	}

	staticSet, err := uciutil.BuildStaticMACSet(uciClient, appConfig.DHCP.ConfigName)
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
