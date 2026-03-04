# DHCP 管理页 `uhttpd` 配置与一键部署说明

## 1. 目标与架构

本项目采用独立 `uhttpd` 实例运行在 `10.0.0.1:8088`，避免改动原生 LuCI。

- 前端：静态资源（`web/`）
- 后端：Go CGI JSON API（`cgi-bin/dhcp_adv_api`）
- 启动脚本：`start.sh`（由 `dhcp_adv_start.sh` 同步而来）
- 保活脚本：`ensure.sh`（由 `dhcp_adv_ensure.sh` 同步而来）

远端目录（默认）：

```text
/data/dhcp_adv/
├── auth.conf            # 管理页登录密码文件：password=你的密码
├── cgi-bin/
│   └── dhcp_adv_api
├── web/
│   ├── index.html
│   └── assets/...
├── start.sh
├── ensure.sh
└── www/                 # start.sh 生成的实际站点目录
```

---

## 2. `uhttpd` 如何配置

核心配置在 `start.sh` 中，通过命令行启动：

```sh
/usr/sbin/uhttpd \
  -f -p 10.0.0.1:8088 \
  -h /data/dhcp_adv/www \
  -x /cgi-bin \
  -i .sh=/bin/sh \
  -t 60 -T 30
```

参数说明：

- `-f`：前台模式（由 `start-stop-daemon` 托管为后台）
- `-p 10.0.0.1:8088`：监听地址和端口
- `-h /data/dhcp_adv/www`：网站根目录
- `-x /cgi-bin`：CGI 路径前缀
- `-i .sh=/bin/sh`：保留 `.sh` 解释器映射（Go 二进制 CGI 不依赖此项）
- `-t 60`：HTTP 连接超时
- `-T 30`：脚本执行超时

---

## 3. 启动脚本做了什么

`start.sh`（即本地 `dhcp_adv_start.sh`）执行流程：

1. 校验关键文件存在（`cgi-bin/dhcp_adv_api`、`web/index.html`）。
2. 同步 `web/*` 到 `/data/dhcp_adv/www/`。
3. 同步 `cgi-bin/dhcp_adv_api` 到 `/data/dhcp_adv/www/cgi-bin/`。
4. 清理旧 CGI（`dhcp_adv.sh`、`dhcp_adv_api.sh`）。
5. 停掉旧 `uhttpd:8088` 进程。
6. 重新拉起新实例。

---

## 4. 认证与权限控制

现在 API 已加会话鉴权：

- 未登录访问 `/cgi-bin/dhcp_adv_api?action=get_state` 会返回 `UNAUTHORIZED`
- 前端页面先显示登录页，登录成功后才可操作 DHCP
- Session 通过 Cookie 保存（默认 1 小时）

密码来源：

- 文件：`/data/dhcp_adv/auth.conf`
- 格式：`password=你的密码`

若该文件不存在，API 会使用默认密码 `admin123456`，并在首次登录时写入默认配置。

---

## 5. 手工部署（可学习完整过程）

> 下面命令假设你在本项目目录执行，且路由器密码已知。

1. 本地交叉编译 Go CGI（OpenWrt aarch64）：

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o cgi-bin/dhcp_adv_api ./cmd/dhcp_adv_api
```

2. 上传前端、二进制与脚本：

```sh
sshpass -p '你的密码' scp -O -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -r web root@10.0.0.1:/data/dhcp_adv/
sshpass -p '你的密码' scp -O -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null cgi-bin/dhcp_adv_api root@10.0.0.1:/data/dhcp_adv/cgi-bin/dhcp_adv_api
sshpass -p '你的密码' scp -O -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null dhcp_adv_start.sh root@10.0.0.1:/data/dhcp_adv/start.sh
sshpass -p '你的密码' scp -O -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null dhcp_adv_ensure.sh root@10.0.0.1:/data/dhcp_adv/ensure.sh
```

3. 远端赋权并重启：

```sh
sshpass -p '你的密码' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@10.0.0.1 "\
rm -f /data/dhcp_adv/cgi-bin/dhcp_adv_api.sh && \
chmod +x /data/dhcp_adv/start.sh /data/dhcp_adv/ensure.sh /data/dhcp_adv/cgi-bin/dhcp_adv_api && \
/data/dhcp_adv/start.sh"
```

4. 验证：

```sh
curl -I http://10.0.0.1:8088/
curl -I "http://10.0.0.1:8088/cgi-bin/dhcp_adv_api?action=auth_status"
curl -I http://10.0.0.1:8088/cgi-bin/dhcp_adv.sh
```

期望：

- `/` 返回 `200`
- `...dhcp_adv_api?action=auth_status` 返回 `200`
- `/cgi-bin/dhcp_adv.sh` 返回 `404`

---

## 6. 一键部署脚本

项目根目录新增：

- `deploy_oneclick.sh`

### 6.1 最简用法

```sh
ROUTER_PASS='你的密码' ./deploy_oneclick.sh
```

### 6.2 常用参数

```sh
./deploy_oneclick.sh \
  --host 10.0.0.1 \
  --user root \
  --port 22 \
  --remote-dir /data/dhcp_adv \
  --http-port 8088 \
  --dashboard-password '改成你自己的管理页密码' \
  --enable-cron
```

参数说明：

- `--enable-cron`：自动写入
  - `* * * * * /data/dhcp_adv/ensure.sh`
  - `@reboot /data/dhcp_adv/start.sh`
- `--skip-verify`：跳过部署后的 HTTP 自动验收
- `--dashboard-password`：写入 `/data/dhcp_adv/auth.conf`，用于管理页登录

---

## 7. 常见问题

### 7.1 `scp: /usr/libexec/sftp-server: not found`

该 OpenWrt 环境不支持 SFTP 子系统，需要强制旧 SCP 协议：

- 使用 `scp -O`

脚本已内置该参数。

### 7.2 Host key 变更告警

脚本默认使用：

- `StrictHostKeyChecking=no`
- `UserKnownHostsFile=/dev/null`

适合内网临时维护；若你有更高安全要求，建议改为固定主机指纹策略。

### 7.3 登录不上管理页

先在路由器检查密码文件：

```sh
cat /data/dhcp_adv/auth.conf
```

若需要重置密码：

```sh
echo 'password=新密码' > /data/dhcp_adv/auth.conf
chmod 600 /data/dhcp_adv/auth.conf
/data/dhcp_adv/start.sh
```

---

## 8. 建议

- 改动前先备份 `/data/dhcp_adv/`
- 上线后执行一次 `ensure.sh` 与 `start.sh` 手工验证
- 若后续改端口，记得同步改 `start.sh` 与验证 URL
