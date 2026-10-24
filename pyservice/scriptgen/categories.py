# -*- coding: utf-8 -*-
"""脚本生成器功能分类与表单字段定义（供校验使用；前端表单由 Go /categories 提供）。

与后端 Go groups 定义保持一致；仅保留校验所需的 key/label/required/type/show_if。
"""
from typing import List, Dict

# 各分类字段定义（key, label, required, type, show_if）
CATEGORIES: List[Dict] = [
    # ===== 路由器 =====
    {"code": "router_acl", "fields": [
        {"key": "acltype", "label": "ACL类型", "type": "select", "required": True},
        {"key": "num", "label": "ACL编号", "type": "number", "required": True},
        {"key": "action", "label": "动作", "type": "select", "required": True},
        {"key": "proto", "label": "协议", "type": "select", "required": True, "show_if": {"key": "acltype", "eq_any": ["三层", "自定义"]}},
        {"key": "src", "label": "源地址", "type": "network", "required": True, "show_if": {"key": "acltype", "not_eq": "二层"}},
        {"key": "dst", "label": "目的地址", "type": "network", "required": False, "show_if": {"key": "acltype", "eq_any": ["三层", "自定义"]}},
        {"key": "dport", "label": "目的端口", "type": "number", "required": False, "show_if": {"key": "acltype", "eq_any": ["三层", "自定义"]}},
        {"key": "srcmac", "label": "源MAC地址", "type": "text", "required": True, "show_if": {"key": "acltype", "eq": "二层"}},
    ]},
    {"code": "router_nat", "fields": [
        {"key": "outif", "label": "出接口", "type": "text", "required": True},
        {"key": "acl", "label": "ACL编号", "type": "number", "required": True},
    ]},
    {"code": "router_ospf", "fields": [
        {"key": "process", "label": "进程号", "type": "number", "required": True},
        {"key": "rid", "label": "Router ID", "type": "ip", "required": True},
        {"key": "net", "label": "宣告网段", "type": "ip", "required": True},
        {"key": "netmask", "label": "掩码", "type": "mask", "required": True},
        {"key": "area", "label": "区域号", "type": "text", "required": True},
        {"key": "enable_vpn", "label": "启用MPLS VPN", "type": "switch", "required": False},
        {"key": "vpn", "label": "VPN实例名", "type": "text", "required": True, "show_if": {"key": "enable_vpn", "eq": "on"}},
    ]},
    {"code": "router_static", "fields": [
        {"key": "dest", "label": "目标网段", "type": "ip", "required": True},
        {"key": "mask", "label": "掩码", "type": "text", "required": True},
        {"key": "nh", "label": "下一跳", "type": "ip", "required": True},
        {"key": "enable_vpn", "label": "启用MPLS VPN", "type": "switch", "required": False},
        {"key": "vpn", "label": "VPN实例名", "type": "text", "required": True, "show_if": {"key": "enable_vpn", "eq": "on"}},
    ]},
    {"code": "router_bgp", "fields": [
        {"key": "asn", "label": "本地AS号", "type": "number", "required": True},
        {"key": "rid", "label": "Router ID", "type": "ip", "required": True},
        {"key": "peer_as", "label": "邻居AS号", "type": "text", "required": True},
        {"key": "peer_ip", "label": "邻居IP", "type": "text", "required": True},
        {"key": "conn_int", "label": "更新源接口", "type": "text", "required": False},
        {"key": "enable_net", "label": "启用发布路由", "type": "switch", "required": False},
        {"key": "network", "label": "发布网段(IPv4)", "type": "ip", "required": True, "show_if": {"key": "enable_net", "eq": "on"}},
        {"key": "netmask", "label": "掩码", "type": "mask", "required": True, "show_if": {"key": "enable_net", "eq": "on"}},
        {"key": "vpn4", "label": "启用VPNv4地址族", "type": "switch", "required": False},
        {"key": "vpn4_enable_peer", "label": "VPNv4中启用邻居", "type": "text", "required": False, "show_if": {"key": "vpn4", "eq": "on"}},
        {"key": "vpn4_adv_comm", "label": "VPNv4发送团体属性邻居", "type": "text", "required": False, "show_if": {"key": "vpn4", "eq": "on"}},
        {"key": "vpn4_next_hop_inv", "label": "VPNv4下一跳不变邻居", "type": "text", "required": False, "show_if": {"key": "vpn4", "eq": "on"}},
        {"key": "enable_vpn", "label": "绑定MPLS VPN实例", "type": "switch", "required": False},
        {"key": "vpn", "label": "VPN实例名", "type": "text", "required": True, "show_if": {"key": "enable_vpn", "eq": "on"}},
        {"key": "enable_import", "label": "启用引入路由", "type": "switch", "required": False, "show_if": {"key": "enable_vpn", "eq": "on"}},
        {"key": "vpn_import", "label": "引入路由协议", "type": "text", "required": False, "show_if": {"key": "enable_import", "eq": "on"}},
        {"key": "enable_export", "label": "启用发布路由", "type": "switch", "required": False, "show_if": {"key": "enable_vpn", "eq": "on"}},
        {"key": "vpn_export", "label": "发布路由到", "type": "text", "required": False, "show_if": {"key": "enable_export", "eq": "on"}},
    ]},
    {"code": "router_isis", "fields": [
        {"key": "net", "label": "NET地址", "type": "text", "required": True},
        {"key": "pid", "label": "进程号", "type": "number", "required": False},
        {"key": "rid", "label": "Router ID", "type": "text", "required": False},
        {"key": "level", "label": "级别", "type": "select", "required": True},
        {"key": "enable_vpn", "label": "启用MPLS VPN", "type": "switch", "required": False},
        {"key": "vpn", "label": "VPN实例名", "type": "text", "required": True, "show_if": {"key": "enable_vpn", "eq": "on"}},
    ]},
    {"code": "router_policy", "fields": [
        {"key": "acl", "label": "ACL编号", "type": "number", "required": True},
        {"key": "cname", "label": "流分类名称", "type": "text", "required": False},
        {"key": "bname", "label": "流行为名称", "type": "text", "required": False},
        {"key": "pname", "label": "流策略名称", "type": "text", "required": False},
        {"key": "iface", "label": "应用接口", "type": "text", "required": True},
        {"key": "nh", "label": "指定下一跳", "type": "ip", "required": True},
    ]},
    {"code": "route_policy", "fields": [
        {"key": "name", "label": "路由策略名", "type": "text", "required": True},
        {"key": "seq", "label": "节点序号", "type": "number", "required": False},
        {"key": "enable_prefix", "label": "启用前缀列表", "type": "switch", "required": False},
        {"key": "prefix", "label": "前缀列表名", "type": "text", "required": True, "show_if": {"key": "enable_prefix", "eq": "on"}},
        {"key": "pfx_index", "label": "规则序号", "type": "number", "required": False, "show_if": {"key": "enable_prefix", "eq": "on"}},
        {"key": "pfx_action", "label": "动作", "type": "select", "required": False, "show_if": {"key": "enable_prefix", "eq": "on"}},
        {"key": "pfx_net", "label": "匹配网段", "type": "text", "required": False, "show_if": {"key": "enable_prefix", "eq": "on"}},
        {"key": "pfx_masklen", "label": "掩码长度", "type": "number", "required": False, "show_if": {"key": "enable_prefix", "eq": "on"}},
    ]},
    {"code": "mpls_vpn", "fields": [
        {"key": "rd", "label": "RD", "type": "text", "required": True},
        {"key": "rt_in", "label": "RT导入", "type": "text", "required": True},
        {"key": "rt_out", "label": "RT导出", "type": "text", "required": True},
        {"key": "vpn_name", "label": "VPN实例名", "type": "text", "required": True},
        {"key": "iface", "label": "绑定接口", "type": "text", "required": True},
        {"key": "ip", "label": "接口IP", "type": "text", "required": True},
    ]},
    # ===== 交换机 =====
    {"code": "sw_vlan", "fields": [
        {"key": "vlan", "label": "VLAN号", "type": "text", "required": True},
        {"key": "vlanif", "label": "网关VLAN号", "type": "number", "required": False},
        {"key": "ip", "label": "网关IP/掩码", "type": "network", "required": False},
    ]},
    {"code": "sw_access", "fields": [
        {"key": "iface", "label": "接口", "type": "text", "required": True},
        {"key": "vlan", "label": "PVID", "type": "number", "required": True},
    ]},
    {"code": "sw_trunk", "fields": [
        {"key": "iface", "label": "接口", "type": "text", "required": True},
        {"key": "vlans", "label": "允许VLAN", "type": "text", "required": True},
    ]},
    {"code": "sw_dhcp", "fields": [
        {"key": "pool", "label": "地址池名", "type": "text", "required": True},
        {"key": "net", "label": "网段", "type": "network", "required": True},
        {"key": "gw", "label": "网关", "type": "ip", "required": True},
        {"key": "dns", "label": "DNS", "type": "text", "required": False},
    ]},
    {"code": "sw_stp", "fields": [
        {"key": "mode", "label": "模式", "type": "select", "required": True},
        {"key": "instance", "label": "MSTI实例号", "type": "number", "required": False, "show_if": {"key": "mode", "eq": "mstp"}},
        {"key": "vlan", "label": "VLAN映射", "type": "text", "required": False, "show_if": {"key": "mode", "eq": "mstp"}},
        {"key": "root", "label": "根桥角色", "type": "select", "required": True},
        {"key": "pri", "label": "优先级", "type": "number", "required": False},
    ]},
    # ===== 通用 =====
    {"code": "com_ssh", "fields": [
        {"key": "user", "label": "用户名", "type": "text", "required": True},
        {"key": "pass", "label": "密码", "type": "password", "required": True},
        {"key": "pri", "label": "权限级别", "type": "select", "required": True},
        {"key": "rsabits", "label": "RSA密钥长度", "type": "select", "required": False},
        {"key": "vty", "label": "VTY线路范围", "type": "text", "required": False},
    ]},
    {"code": "com_user", "fields": [
        {"key": "user", "label": "用户名", "type": "text", "required": True},
        {"key": "pass", "label": "密码", "type": "password", "required": True},
        {"key": "pri", "label": "权限级别", "type": "select", "required": True},
    ]},
    {"code": "com_log", "fields": [
        {"key": "logip", "label": "日志服务器IP", "type": "ip", "required": True},
    ]},
    {"code": "com_snmp", "fields": [
        {"key": "ver", "label": "SNMP版本", "type": "select", "required": True},
        {"key": "host", "label": "平台/网管站IP", "type": "ip", "required": True},
        {"key": "community", "label": "团体字", "type": "text", "required": False, "show_if": {"key": "ver", "not_eq": "v3"}},
        {"key": "secname", "label": "用户名(v3)", "type": "text", "required": False, "show_if": {"key": "ver", "eq": "v3"}},
        {"key": "authproto", "label": "认证协议(v3)", "type": "select", "required": False, "show_if": {"key": "ver", "eq": "v3"}},
        {"key": "authpass", "label": "认证密码(v3)", "type": "password", "required": False, "show_if": {"key": "ver", "eq": "v3"}},
        {"key": "privproto", "label": "加密协议(v3)", "type": "select", "required": False, "show_if": {"key": "ver", "eq": "v3"}},
        {"key": "privpass", "label": "加密密码(v3)", "type": "password", "required": False, "show_if": {"key": "ver", "eq": "v3"}},
    ]},
]
