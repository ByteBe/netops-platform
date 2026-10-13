交换机/路由器 SNMP 监控配置范本（只读）
=============================================

【华三 v2c】
snmp-agent
snmp-agent sys-info contact noc@example.com
snmp-agent sys-info location IDC-1F
snmp-agent sys-info version v2c
snmp-agent community read <团体字>
snmp-agent packet max-size 10000
snmp-agent trap enable
snmp-agent trap standard
snmp-agent target-host trap address udp-domain <平台IP> udp-port 162 params securityname <团体字> v2c

【华三 v3】
snmp-agent
snmp-agent sys-info version v3
snmp-agent group v3 <组名> privacy
snmp-agent usm-user v3 <用户名> <组名>
snmp-agent usm-user v3 <用户名> authentication-mode sha <认证密码>
snmp-agent usm-user v3 <用户名> privacy-mode aes128 <加密密码>
snmp-agent target-host trap address udp-domain <平台IP> udp-port 162 params securityname <用户名> v3 privacy

【华为 v2c】
snmp-agent
snmp-agent sys-info contact noc@example.com
snmp-agent sys-info location IDC-1F
snmp-agent sys-info version v2c
snmp-agent community read <团体字>
snmp-agent packet max-size 10000
snmp-agent trap enable
snmp-agent target-host trap-hostname netops address <平台IP> udp-port 162 trap-paramsname netops
snmp-agent trap-paramsname netops udp-port 162 security-model v2c security-name <团体字>

【华为 v3】
snmp-agent
snmp-agent sys-info version v3
snmp-agent group v3 <组名> privacy
snmp-agent usm-user v3 <用户名> <组名>
snmp-agent usm-user v3 <用户名> authentication-mode sha <认证密码>
snmp-agent usm-user v3 <用户名> privacy-mode aes128 <加密密码>
snmp-agent target-host trap-hostname netops address <平台IP> udp-port 162 trap-paramsname netops
snmp-agent trap-paramsname netops udp-port 162 security-model v3 security-name <用户名> privacy

【思科 v2c】
snmp-server community <团体字> RO
snmp-server contact noc@example.com
snmp-server location IDC-1F
snmp-server enable traps
snmp-server host <平台IP> version 2c <团体字>

【思科 v3】
snmp-server group <组名> v3 priv
snmp-server user <用户名> <组名> v3 auth sha <认证密码> priv aes 128 <加密密码>
snmp-server enable traps
snmp-server host <平台IP> version 3 priv <用户名>

说明：
1. 本范本只开启只读监控，不修改业务配置。
2. <团体字>、<用户名>、<认证密码>、<加密密码>、<平台IP> 按实际填写。
3. 平台侧在“设备监控”中填相同版本和凭证即可采集。
