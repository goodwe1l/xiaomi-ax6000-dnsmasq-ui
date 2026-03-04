package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dhcp_adv/pkg/utils"
)

func (h *APIHandler) HandleLeaseUpsert(w http.ResponseWriter, r *http.Request) {
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
}

func (h *APIHandler) HandleLeaseDelete(w http.ResponseWriter, r *http.Request) {
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
}

func applyLeaseUpsert(r *http.Request) (string, error) {
	name := strings.TrimSpace(r.FormValue("name"))
	mac := utils.NormalizeMAC(r.FormValue("mac"))
	ip := strings.TrimSpace(r.FormValue("ip"))
	gateway := strings.TrimSpace(r.FormValue("gateway"))
	dns := utils.NormalizeDNS(strings.TrimSpace(r.FormValue("dns")))
	tag := strings.TrimSpace(r.FormValue("tag"))

	if name == "" {
		return "", newCodedError(validationErrorCode, "设备名不能为空")
	}
	if !utils.ValidMAC(mac) {
		return "", newCodedError(validationErrorCode, "MAC 格式不正确（示例：AA:BB:CC:DD:EE:FF）")
	}
	if !utils.ValidIPv4(ip) {
		return "", newCodedError(validationErrorCode, "IP 格式不正确（示例：10.0.0.120）")
	}
	if gateway != "" && !utils.ValidIPv4(gateway) {
		return "", newCodedError(validationErrorCode, "网关格式不正确")
	}
	if !utils.ValidateDNSList(dns) {
		return "", newCodedError(validationErrorCode, "DNS 格式不正确")
	}

	macID := utils.SanitizeMACID(mac)
	hostSec := "advh_" + macID
	tagSec := "advt_" + macID
	tagName := "adv_" + macID

	oldSec, foundOld, err := utils.FindHostSecByMAC(uciClient, appConfig.DHCP.ConfigName, mac)
	if err != nil {
		return "", err
	}
	oldTag := ""
	if foundOld {
		if val, ok, err := uciClient.Get("dhcp." + oldSec + ".tag"); err != nil {
			return "", err
		} else if ok {
			oldTag = val
		}
	}
	if foundOld && oldSec != hostSec {
		if err := uciClient.Delete("dhcp." + oldSec); err != nil {
			return "", err
		}
		if err := utils.DeleteAdvTagByName(uciClient, appConfig.DHCP.ConfigName, oldTag); err != nil {
			return "", err
		}
	}

	if err := uciClient.Set("dhcp."+hostSec, "host"); err != nil {
		return "", err
	}
	if err := uciClient.Set("dhcp."+hostSec+".name", name); err != nil {
		return "", err
	}
	if err := uciClient.Set("dhcp."+hostSec+".mac", mac); err != nil {
		return "", err
	}
	if err := uciClient.Set("dhcp."+hostSec+".ip", ip); err != nil {
		return "", err
	}

	if tag != "" {
		tSec, foundTagSec, err := utils.ResolveTagSec(uciClient, appConfig.DHCP.ConfigName, tag)
		if err != nil {
			return "", err
		}
		if !foundTagSec {
			if gateway == "" && dns == "" {
				return "", newCodedError(validationErrorCode, "标签不存在，请从模板选择，或同时填写网关/DNS创建新标签")
			}
			safeTag := utils.SanitizeTagKey(tag)
			tSec = "tag_" + safeTag
			idx := 0
			for {
				exists, err := utils.SectionExists(uciClient, tSec)
				if err != nil {
					return "", err
				}
				if !exists {
					break
				}
				idx++
				tSec = fmt.Sprintf("tag_%s_%d", safeTag, idx)
			}
			if err := uciClient.Set("dhcp."+tSec, "tag"); err != nil {
				return "", err
			}
			if err := uciClient.Set("dhcp."+tSec+".tag", tag); err != nil {
				return "", err
			}
		}

		if err := uciClient.Set("dhcp."+hostSec+".tag", tSec); err != nil {
			return "", err
		}
		if err := uciClient.Delete("dhcp." + hostSec + ".adv_gateway"); err != nil {
			return "", err
		}
		if err := uciClient.Delete("dhcp." + hostSec + ".adv_dns"); err != nil {
			return "", err
		}
		if err := utils.DeleteAdvTagByName(uciClient, appConfig.DHCP.ConfigName, oldTag); err != nil {
			return "", err
		}
		if gateway != "" || dns != "" {
			if err := uciClient.Delete("dhcp." + tSec + ".dhcp_option"); err != nil {
				return "", err
			}
			if gateway != "" {
				if err := uciClient.AddList("dhcp."+tSec+".dhcp_option", "3,"+gateway); err != nil {
					return "", err
				}
			}
			if dns != "" {
				if err := uciClient.AddList("dhcp."+tSec+".dhcp_option", "6,"+dns); err != nil {
					return "", err
				}
			}
		}
		if err := uciClient.Delete("dhcp." + tagSec); err != nil {
			return "", err
		}
	} else if gateway != "" || dns != "" {
		if err := uciClient.Set("dhcp."+hostSec+".tag", tagSec); err != nil {
			return "", err
		}
		if err := uciClient.Set("dhcp."+hostSec+".adv_gateway", gateway); err != nil {
			return "", err
		}
		if err := uciClient.Set("dhcp."+hostSec+".adv_dns", dns); err != nil {
			return "", err
		}
		if err := uciClient.Set("dhcp."+tagSec, "tag"); err != nil {
			return "", err
		}
		if err := uciClient.Set("dhcp."+tagSec+".tag", tagName); err != nil {
			return "", err
		}
		if err := uciClient.Delete("dhcp." + tagSec + ".dhcp_option"); err != nil {
			return "", err
		}
		if gateway != "" {
			if err := uciClient.AddList("dhcp."+tagSec+".dhcp_option", "3,"+gateway); err != nil {
				return "", err
			}
		}
		if dns != "" {
			if err := uciClient.AddList("dhcp."+tagSec+".dhcp_option", "6,"+dns); err != nil {
				return "", err
			}
		}
	} else {
		if err := uciClient.Delete("dhcp." + hostSec + ".tag"); err != nil {
			return "", err
		}
		if err := uciClient.Delete("dhcp." + hostSec + ".adv_gateway"); err != nil {
			return "", err
		}
		if err := uciClient.Delete("dhcp." + hostSec + ".adv_dns"); err != nil {
			return "", err
		}
		if err := uciClient.Delete("dhcp." + tagSec); err != nil {
			return "", err
		}
		if err := utils.DeleteAdvTagByName(uciClient, appConfig.DHCP.ConfigName, oldTag); err != nil {
			return "", err
		}
	}

	if err := uciClient.Commit(appConfig.DHCP.ConfigName); err != nil {
		return "", err
	}
	if err := uciClient.RestartDNSMasq(); err != nil {
		return "", err
	}
	return "保存成功", nil
}

func applyLeaseDelete(mac string) (string, error) {
	mac = utils.NormalizeMAC(mac)
	if !utils.ValidMAC(mac) {
		return "", newCodedError(validationErrorCode, "删除失败：MAC 格式不正确")
	}
	sec, found, err := utils.FindHostSecByMAC(uciClient, appConfig.DHCP.ConfigName, mac)
	if err != nil {
		return "", err
	}
	if !found {
		return "", newCodedError(validationErrorCode, "删除失败：未找到该设备")
	}
	htag, _, err := uciClient.Get("dhcp." + sec + ".tag")
	if err != nil {
		return "", err
	}
	if err := uciClient.Delete("dhcp." + sec); err != nil {
		return "", err
	}
	if err := utils.DeleteAdvTagByName(uciClient, appConfig.DHCP.ConfigName, htag); err != nil {
		return "", err
	}
	if err := uciClient.Commit(appConfig.DHCP.ConfigName); err != nil {
		return "", err
	}
	if err := uciClient.RestartDNSMasq(); err != nil {
		return "", err
	}
	return "删除成功", nil
}
