# -*- coding: utf-8 -*-
"""pyservice.report —— 巡检报告/周报 HTML 渲染（Python 智能外围）。
Go 端收集结构化数据（MySQL/TSDB/各采集器快照）后转发本模块渲染 HTML，
失败时 Go 自动回退原生渲染实现。"""
