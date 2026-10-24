# -*- coding: utf-8 -*-
"""数据库连接配置：从环境变量读取，默认 53 上的 MySQL / TDengine 地址。"""
import os

MYSQL = {
    "host": os.environ.get("MYSQL_HOST", "127.0.0.1"),
    "port": int(os.environ.get("MYSQL_PORT", "3306")),
    "user": os.environ.get("MYSQL_USER", "root"),
    "password": os.environ.get("MYSQL_PASSWORD", "tssbs123"),
    "database": os.environ.get("MYSQL_DATABASE", "netops"),
}

TDENGINE = {
    "host": os.environ.get("TD_HOST", "127.0.0.1"),
    "port": int(os.environ.get("TD_PORT", "6041")),
    "user": os.environ.get("TD_USER", "root"),
    "password": os.environ.get("TD_PASSWORD", "taosdata"),
    "database": os.environ.get("TD_DATABASE", "data"),
}

# 服务监听端口
PY_PORT = int(os.environ.get("PY_PORT", "8000"))

_LEVEL_ZH = {"critical": "严重", "error": "错误", "warning": "警告", "info": "信息"}


def level_zh(level: str) -> str:
    """日志/告警级别英文转中文。"""
    return _LEVEL_ZH.get(level, level)
