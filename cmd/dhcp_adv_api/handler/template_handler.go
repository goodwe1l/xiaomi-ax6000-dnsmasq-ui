package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dhcp_adv/pkg/utils"
)

func (h *APIHandler) HandleTemplateUpsert(w http.ResponseWriter, r *http.Request) {
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
}

func (h *APIHandler) HandleTemplateDelete(w http.ResponseWriter, r *http.Request) {
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
}

func applyTemplateUpsert(templateSec, templateTag, templateGateway, templateDNS string) (string, error) {
	tname := strings.TrimSpace(templateTag)
	tgw := strings.TrimSpace(templateGateway)
	tdns := utils.NormalizeDNS(templateDNS)

	if tname == "" {
		return "", newCodedError(validationErrorCode, "标签名不能为空")
	}
	if strings.HasPrefix(tname, "adv_") {
		return "", newCodedError(validationErrorCode, "标签名不能以 adv_ 开头")
	}
	if tgw != "" && !utils.ValidIPv4(tgw) {
		return "", newCodedError(validationErrorCode, "标签网关格式不正确")
	}
	if !utils.ValidateDNSList(tdns) {
		return "", newCodedError(validationErrorCode, "标签 DNS 格式不正确")
	}

	var tsec string
	if templateSec != "" {
		isSec, err := utils.IsTagSection(uciClient, templateSec)
		if err != nil {
			return "", err
		}
		if !isSec {
			return "", newCodedError(validationErrorCode, "标签模板不存在")
		}
		oldName, _, err := uciClient.Get("dhcp." + templateSec + ".tag")
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(oldName, "adv_") {
			return "", newCodedError(validationErrorCode, "系统自动标签不可修改")
		}
		existingSec, found, err := utils.ResolveTagSec(uciClient, appConfig.DHCP.ConfigName, tname)
		if err != nil {
			return "", err
		}
		if found && existingSec != templateSec {
			return "", newCodedError(validationErrorCode, "标签名已存在")
		}
		tsec = templateSec
	} else {
		existingSec, found, err := utils.ResolveTagSec(uciClient, appConfig.DHCP.ConfigName, tname)
		if err != nil {
			return "", err
		}
		if found {
			tsec = existingSec
		} else {
			safe := utils.SanitizeTagKey(tname)
			tsec = "tag_" + safe
			idx := 0
			for {
				exists, err := utils.SectionExists(uciClient, tsec)
				if err != nil {
					return "", err
				}
				if !exists {
					break
				}
				idx++
				tsec = fmt.Sprintf("tag_%s_%d", safe, idx)
			}
			if err := uciClient.Set("dhcp."+tsec, "tag"); err != nil {
				return "", err
			}
		}
	}

	if err := uciClient.Set("dhcp."+tsec+".tag", tname); err != nil {
		return "", err
	}
	if err := uciClient.Delete("dhcp." + tsec + ".dhcp_option"); err != nil {
		return "", err
	}
	if tgw != "" {
		if err := uciClient.AddList("dhcp."+tsec+".dhcp_option", "3,"+tgw); err != nil {
			return "", err
		}
	}
	if tdns != "" {
		if err := uciClient.AddList("dhcp."+tsec+".dhcp_option", "6,"+tdns); err != nil {
			return "", err
		}
	}

	if err := uciClient.Commit(appConfig.DHCP.ConfigName); err != nil {
		return "", err
	}
	if err := uciClient.RestartDNSMasq(); err != nil {
		return "", err
	}
	return "标签模板保存成功", nil
}

func applyTemplateDelete(templateSec string) (string, error) {
	sec := strings.TrimSpace(templateSec)
	isSec, err := utils.IsTagSection(uciClient, sec)
	if err != nil {
		return "", err
	}
	if !isSec {
		return "", newCodedError(validationErrorCode, "删除失败：标签模板不存在")
	}
	label, _, err := uciClient.Get("dhcp." + sec + ".tag")
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(label, "adv_") {
		return "", newCodedError(validationErrorCode, "删除失败：系统自动标签不可删除")
	}
	inUse, err := utils.TemplateInUse(uciClient, appConfig.DHCP.ConfigName, sec, label)
	if err != nil {
		return "", err
	}
	if inUse {
		return "", newCodedError(validationErrorCode, "删除失败：还有静态租约正在使用该标签")
	}
	if err := uciClient.Delete("dhcp." + sec); err != nil {
		return "", err
	}
	if err := uciClient.Commit(appConfig.DHCP.ConfigName); err != nil {
		return "", err
	}
	if err := uciClient.RestartDNSMasq(); err != nil {
		return "", err
	}
	return "标签模板删除成功", nil
}
