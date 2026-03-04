# DHCP 高级管理服务

> 面向小米 OpenWrt 的 DHCP 可视化配置系统（Go 单体服务 + 内嵌前端）

![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)
![OpenWrt](https://img.shields.io/badge/OpenWrt-arm64-66CC33)
![Deploy](https://img.shields.io/badge/Deploy-OneClick%20%2F%20Manual-005FA6)

## 1. 业务介绍

这个项目解决的是家庭/小型办公网络中 DHCP 配置复杂、易出错的问题。

传统方式通常依赖命令行逐条修改 UCI 配置，存在这些痛点：

- 静态租约、标签模板、默认网关与 DNS 规则分散，修改成本高
- 误操作后很难快速回滚与定位
- 新增设备、动态转静态场景需要重复录入

本项目提供统一的 Web 管理入口，把 DHCP 常用运维动作收敛为可视化操作，后端保持与 OpenWrt UCI 逻辑一致。

## 2. 核心能力

- DHCP 开关管理（LAN）
- 默认 DHCP 规则管理（Option 3 / Option 6）
- 静态租约管理（新增、行内编辑、删除）
- 标签模板管理（新增、行内编辑、删除）
- 动态租约查看与转静态
- 登录态控制（会话 Cookie + 本地会话文件）

## 3. 架构说明

```text
浏览器
  -> Go HTTP 服务（:8088）
     -> 静态资源（embed）
     -> API 路由（/cgi-bin/dhcp_adv_api）
        -> UCI 命令封装
        -> dnsmasq 重启
```

说明：当前版本不再依赖 `uhttpd/cgi-bin` 托管页面。

## 4. 安装与部署

### 4.1 一键部署（源码仓库方式）

适用场景：你已克隆本仓库，希望本地编译后直接部署到路由器。

前置条件：

- 本机有 `go`、`sshpass`、`ssh`、`scp`、`curl`
- 可通过 SSH 访问路由器（默认 `root@10.0.0.1`）

执行命令：

```sh
ROUTER_PASS='你的SSH密码' ./scripts/deploy_oneclick.sh --host 10.0.0.1
```

可选参数示例：

```sh
./scripts/deploy_oneclick.sh \
  --host 10.0.0.1 \
  --user root \
  --port 22 \
  --remote-dir /data/dhcp_adv \
  --http-port 8088 \
  --listen-addr 0.0.0.0:8088 \
  --dashboard-password '你的管理页密码' \
  --enable-cron
```

### 4.2 手动下载安装部署（Release 包方式）

适用场景：只下载 CI/CD 产出的 Release 包，不拉源码，不想本地编译。

第一步：从 GitHub Release 下载以下两个文件：

- `dhcp_adv_<tag>_xiaomi_arm64.tar.gz`
- `dhcp_adv_<tag>_xiaomi_arm64.tar.gz.sha256`

第二步：校验完整性（推荐）：

```sh
sha256sum -c dhcp_adv_<tag>_xiaomi_arm64.tar.gz.sha256
```

第三步：解压：

```sh
mkdir -p /tmp/dhcp_adv_release
tar -xzf dhcp_adv_<tag>_xiaomi_arm64.tar.gz -C /tmp/dhcp_adv_release
cd /tmp/dhcp_adv_release
ls -1
```

Release 包内容不只有一个二进制，默认包含：

- `dhcp_adv_api`
- `deploy_oneclick.sh`
- `dhcp_adv_start.sh`
- `dhcp_adv_ensure.sh`
- `api_regression.sh`
- `README.md`

第四步（推荐）：在解压目录直接一键部署：

```sh
ROUTER_PASS='你的SSH密码' ./deploy_oneclick.sh --host 10.0.0.1
```

第五步（可选）：完全手工部署：

```sh
sshpass -p '你的SSH密码' scp -O -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null dhcp_adv_api root@10.0.0.1:/data/dhcp_adv/dhcp_adv_api
sshpass -p '你的SSH密码' scp -O -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null dhcp_adv_start.sh root@10.0.0.1:/data/dhcp_adv/start.sh
sshpass -p '你的SSH密码' scp -O -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null dhcp_adv_ensure.sh root@10.0.0.1:/data/dhcp_adv/ensure.sh
sshpass -p '你的SSH密码' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@10.0.0.1 "chmod +x /data/dhcp_adv/dhcp_adv_api /data/dhcp_adv/start.sh /data/dhcp_adv/ensure.sh && APP_DIR=/data/dhcp_adv DHCP_ADV_LISTEN_ADDR=0.0.0.0:8088 /data/dhcp_adv/start.sh"
```

## 5. 运行配置

支持环境变量：

- `DHCP_ADV_LISTEN_ADDR`：监听地址，默认 `0.0.0.0:8088`
- `DHCP_ADV_AUTH_FILE`：密码文件，默认 `/data/dhcp_adv/auth.conf`
- `DHCP_ADV_SESSION_FILE`：会话文件，默认 `/tmp/dhcp_adv_session`
- `DHCP_ADV_API_PATH`：API 路径，默认 `/cgi-bin/dhcp_adv_api`

密码文件格式：

```text
password=你的管理页密码
```

## 6. 访问与验收

- 页面：`http://<路由器IP>:8088/`
- API：`http://<路由器IP>:8088/cgi-bin/dhcp_adv_api?action=auth_status`

快速检查：

```sh
curl -I http://10.0.0.1:8088/
curl -I 'http://10.0.0.1:8088/cgi-bin/dhcp_adv_api?action=auth_status'
```

## 7. 目录结构

```text
.
├── cmd/dhcp_adv_api/              # Go 服务代码（含 web embed）
├── pkg/utils/                     # 按类型拆分的公共辅助函数
├── scripts/                       # 部署与运维脚本
├── .github/workflows/             # CI/CD 工作流定义
├── go.mod
└── README.md
```

## 8. 文档约定

仓库仅保留 `README.md` 作为统一文档入口。
