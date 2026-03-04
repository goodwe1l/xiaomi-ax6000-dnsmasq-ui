- 本轮完成 Shell CGI -> Go CGI 迁移：新增 go.mod 与 cmd/dhcp_adv_api/main.go（CGI 主程序）。
- API 入口切换为 /cgi-bin/dhcp_adv_api（无 .sh），前端 web/assets/js/api.js 已同步。
- 启动脚本 dhcp_adv_start.sh 改为同步二进制 cgi-bin/dhcp_adv_api 到 /data/dhcp_adv/www/cgi-bin，并清理 dhcp_adv.sh/dhcp_adv_api.sh。
- 一键部署 deploy_oneclick.sh 已改为：本地交叉编译 Go（默认 linux/arm64）-> 上传 web 与二进制 -> 重启 8088 -> 验证新路径。
- 文档已更新：httpd配置与一键部署说明.md、操作记录.md。
- 10.0.0.1 实机验证结果：
  1) / 返回 200
  2) /cgi-bin/dhcp_adv_api?action=auth_status 返回 200
  3) /cgi-bin/dhcp_adv_api?action=get_state 未登录返回 UNAUTHORIZED
  4) POST login(admin123456) 成功后 get_state 返回完整状态
  5) /cgi-bin/dhcp_adv.sh 返回 404
  6) /cgi-bin/dhcp_adv_api.sh 返回 404