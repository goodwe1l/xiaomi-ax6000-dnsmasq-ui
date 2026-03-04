package uci

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

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

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if err != nil {
		return result, err
	}
	return result, nil
}

func IsExitCode(err error, code int) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == code
	}
	return false
}

func (c *Client) Get(key string) (string, bool, error) {
	out, err := c.run("uci", "-q", "get", key)
	if err != nil {
		if IsExitCode(err, 1) {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(out), true, nil
}

func (c *Client) Show(target string) (string, error) {
	return c.run("uci", "-q", "show", target)
}

func (c *Client) Set(key, value string) error {
	_, err := c.run("uci", "set", fmt.Sprintf("%s=%s", key, value))
	return err
}

func (c *Client) Delete(key string) error {
	_, err := c.run("uci", "-q", "delete", key)
	if err != nil && !IsExitCode(err, 1) {
		return err
	}
	return nil
}

func (c *Client) AddList(key, value string) error {
	_, err := c.run("uci", "add_list", fmt.Sprintf("%s=%s", key, value))
	return err
}

func (c *Client) Commit(configName string) error {
	_, err := c.run("uci", "commit", configName)
	return err
}

func (c *Client) RestartDNSMasq() error {
	_, err := c.run("/etc/init.d/dnsmasq", "restart")
	return err
}
