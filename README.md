# Xiaomi DNSMasq GUI

> 🍥 面向小米 OpenWrt 的 DHCP 可视化管理服务（Go 单体服务 + 内嵌前端）

![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)
![OpenWrt](https://img.shields.io/badge/OpenWrt-arm64-66CC33)
![Release](https://img.shields.io/badge/Release-xiaomi--dnsmasq--gui-blue)
![Deploy](https://img.shields.io/badge/Deploy-Script%20%2F%20Manual-005FA6)

**🔥 项目热度**

[![GitHub Stars](https://img.shields.io/github/stars/goodwe1l/xiaomi-ax6000-dnsmasq-ui?style=flat-square)](https://github.com/goodwe1l/xiaomi-ax6000-dnsmasq-ui/stargazers)
[![GitHub Forks](https://img.shields.io/github/forks/goodwe1l/xiaomi-ax6000-dnsmasq-ui?style=flat-square)](https://github.com/goodwe1l/xiaomi-ax6000-dnsmasq-ui/network/members)
[![GitHub Issues](https://img.shields.io/github/issues/goodwe1l/xiaomi-ax6000-dnsmasq-ui?style=flat-square)](https://github.com/goodwe1l/xiaomi-ax6000-dnsmasq-ui/issues)
[![GitHub Downloads](https://img.shields.io/github/downloads/goodwe1l/xiaomi-ax6000-dnsmasq-ui/total?style=flat-square)](https://github.com/goodwe1l/xiaomi-ax6000-dnsmasq-ui/releases)
[![Last Commit](https://img.shields.io/github/last-commit/goodwe1l/xiaomi-ax6000-dnsmasq-ui?style=flat-square)](https://github.com/goodwe1l/xiaomi-ax6000-dnsmasq-ui/commits/main)

[快速开始](#-快速开始) • [核心能力](#-核心能力) • [安装与部署](#-安装与部署) • [一键卸载](#-一键卸载) • [运行配置](#-运行配置) • [访问与验收](#-访问与验收)

---

## 📝 项目说明

> [!IMPORTANT]
>
> - 本项目会直接修改路由器 DHCP/UCI 配置，请在内网环境使用并做好备份。
> - 本项目定位为家庭/小型办公网络管理面板，不提供云端托管能力。
> - 当前版本不依赖 `uhttpd/cgi-bin` 托管页面，服务由 Go 程序独立监听。

你可以把它理解成一个“面向路由器运维的 DHCP 控制台”：

- 把静态租约、标签模板、默认网关/DNS 规则统一到一个 Web 页面
- 用异步交互替代传统命令行反复修改 UCI
- 保持后端行为与 OpenWrt 原生 `dnsmasq + uci` 逻辑一致

---

## ✨ 核心能力

- DHCP 开关管理（LAN）
- 默认 DHCP 规则管理（Option 3 / Option 6）
- 静态租约管理（新增、行内编辑、删除）
- 标签模板管理（新增、行内编辑、删除）
- 动态租约查看与转静态
- 登录态控制（Cookie 会话 + 本地会话文件）

---

## 🏗️ 架构概览

```text
浏览器
  -> Go HTTP 服务（默认 :8088）
     -> 内嵌前端静态资源（embed）
     -> API 路由（/cgi-bin/xiaomi-dnsmasq-gui_api）
        -> UCI 命令封装
        -> dnsmasq 重启与状态同步
```

---

## 🚀 快速开始

### 方式 A：源码仓库一键部署（推荐）

适用：你在本机有源码，希望本地编译后直接部署到路由器。

前置条件：

- 本机已安装 `go`、`sshpass`、`ssh`、`scp`、`curl`
- 路由器可 SSH 登录（默认 `root@10.0.0.1:22`）

执行：

```sh
ROUTER_PASS='你的SSH密码' ./scripts/deploy.sh --host 10.0.0.1
```

脚本为交互式，会询问：

- 路由器 IP
- SSH 端口、账号、密码
- 面板端口
- 面板密码

---

## 🚢 安装与部署

### 1) Release 包手动安装（不编译）

适用：只下载 CI/CD 产物，不希望本地安装 Go。

第一步：下载 Release 文件：

- `xiaomi-dnsmasq-gui_<tag>_arm64.tar.gz`
- `xiaomi-dnsmasq-gui_<tag>_arm64.tar.gz.sha256`

第二步：校验完整性：

```sh
sha256sum -c xiaomi-dnsmasq-gui_<tag>_arm64.tar.gz.sha256
```

第三步：解压：

```sh
mkdir -p /tmp/xiaomi-dnsmasq-gui_release
tar -xzf xiaomi-dnsmasq-gui_<tag>_arm64.tar.gz -C /tmp/xiaomi-dnsmasq-gui_release
cd /tmp/xiaomi-dnsmasq-gui_release
ls -1
```

默认包含：

- `xiaomi-dnsmasq-gui`
- `deploy.sh`
- `start.sh`
- `ensure.sh`
- `api_regression.sh`
- `README.md`

第四步：在解压目录直接部署：

```sh
ROUTER_PASS='你的SSH密码' ./deploy.sh --host 10.0.0.1
```

该脚本会优先使用同目录 `xiaomi-dnsmasq-gui`，无需本地 Go 编译环境。

### 2) 路由器在线一键安装（curl | sh）

适用：你已进入路由器 shell，希望就地在线安装。

```sh
curl -fsSL https://raw.githubusercontent.com/goodwe1l/xiaomi-ax6000-dnsmasq-ui/refs/heads/main/scripts/deploy.sh | sh -s -- install
```

说明：

- 自动下载对应 release 包并安装
- 交互询问路由器 LAN IP、面板端口、面板密码
- 默认监听 `路由器IP:面板端口`

### 3) 可选参数示例

```sh
./scripts/deploy.sh \
  --host 10.0.0.1 \
  --user root \
  --port 22 \
  --remote-dir /data/xiaomi-dnsmasq-gui \
  --http-port 8088 \
  --listen-addr 10.0.0.1:8088 \
  --dashboard-password '你的管理页密码' \
  --enable-cron
```

---

## 🧹 一键卸载

### 方式 A：本地远程卸载（推荐）

源码仓库执行：

```sh
./scripts/deploy.sh uninstall --host 10.0.0.1
```

Release 解压目录执行：

```sh
./deploy.sh uninstall --host 10.0.0.1
```

会执行：

- 停止进程
- 清理 cron 保活项
- 删除安装目录（默认 `/data/xiaomi-dnsmasq-gui`）

默认会二次确认，追加 `--yes` 可跳过确认。

### 方式 B：路由器本机在线卸载

```sh
curl -fsSL https://raw.githubusercontent.com/goodwe1l/xiaomi-ax6000-dnsmasq-ui/refs/heads/main/scripts/deploy.sh | sh -s -- uninstall-local --yes
```

可选指定目录：

```sh
curl -fsSL https://raw.githubusercontent.com/goodwe1l/xiaomi-ax6000-dnsmasq-ui/refs/heads/main/scripts/deploy.sh | sh -s -- uninstall-local --remote-dir /data/xiaomi-dnsmasq-gui --yes
```

---

## ⚙️ 运行配置

支持环境变量：

- `XIAOMI_DNSMASQ_GUI_LISTEN_ADDR`：监听地址，程序默认 `0.0.0.0:8088`
- `XIAOMI_DNSMASQ_GUI_AUTH_FILE`：密码文件，默认 `/data/xiaomi-dnsmasq-gui/auth.conf`
- `XIAOMI_DNSMASQ_GUI_SESSION_FILE`：会话文件，默认 `/tmp/xiaomi-dnsmasq-gui_session`
- `XIAOMI_DNSMASQ_GUI_API_PATH`：API 路径，默认 `/cgi-bin/xiaomi-dnsmasq-gui_api`

密码文件格式：

```text
password=你的管理页密码
```

---

## 🔍 访问与验收

访问地址：

- 页面：`http://<路由器IP>:8088/`
- API：`http://<路由器IP>:8088/cgi-bin/xiaomi-dnsmasq-gui_api?action=auth_status`

快速检查：

```sh
curl -I http://10.0.0.1:8088/
curl -I 'http://10.0.0.1:8088/cgi-bin/xiaomi-dnsmasq-gui_api?action=auth_status'
```

> [!TIP]
> 如果你访问 `http://10.0.0.1:8080/` 看到的是小米原生后台，这是正常现象。
> 本项目默认端口是 `8088`（或你部署时自定义的端口）。

---

## 📂 项目结构

```text
.
├── cmd/api/                                 # Go 服务代码（含 web embed）
├── pkg/uci/                                 # UCI 相关封装与解析
├── pkg/utils/                     # 公共辅助函数
├── scripts/                       # 部署/卸载/运维脚本
├── misc/                          # 回归测试脚本等辅助文件
├── .github/workflows/             # CI/CD 工作流
├── go.mod
└── README.md
```

---

## 🤝 反馈与改进

如果你在使用中遇到部署、权限、UCI 兼容性问题，建议直接提 Issue 并附带：

- 路由器型号与 OpenWrt 版本
- 执行命令与完整错误日志
- `curl -I` 验证结果

---

**仓库文档统一入口：`README.md`**
