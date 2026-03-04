package utils

import ucipkg "dhcp_adv/cmd/dhcp_adv_api/uci"

// UCIClient 定义了 DHCP 业务所需的最小 UCI 能力集。
type UCIClient interface {
	Get(key string) (string, bool, error)
	Show(target string) (string, error)
	Set(key, value string) error
	Delete(key string) error
	AddList(key, value string) error
	Commit(configName string) error
	RestartDNSMasq() error
}

func IsExitCode(err error, code int) bool {
	return ucipkg.IsExitCode(err, code)
}
