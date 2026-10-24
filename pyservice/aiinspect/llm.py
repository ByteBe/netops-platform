# -*- coding: utf-8 -*-
"""LLM 调用：以 openai 兼容 chat/completions 接口调用（deepseek/qwen/openai/ollama 等）。
失败返回 None，由调用方降级为统计报告。"""
from typing import Any, Dict, Optional


def chat(cfg: Dict[str, Any], prompt: str, timeout: int = 90) -> Optional[str]:
    """调用 /chat/completions 生成回复。"""
    import requests

    base = cfg["base_url"]
    if not base:
        return None
    url = f"{base}/chat/completions"
    headers = {"Content-Type": "application/json"}
    if cfg.get("api_key"):
        headers["Authorization"] = f"Bearer {cfg['api_key']}"
    payload = {
        "model": cfg.get("model") or "gpt-3.5-turbo",
        "temperature": cfg.get("temperature", 0.7),
        "max_tokens": 2048,
        "messages": [
            {"role": "system", "content": "你是资深网络运维专家，根据提供的系统数据生成简洁专业的中文巡检报告。用 Markdown 格式。"},
            {"role": "user", "content": prompt},
        ],
    }
    try:
        resp = requests.post(url, json=payload, headers=headers, timeout=timeout)
        if resp.status_code != 200:
            return None
        j = resp.json()
        content = j.get("choices") and j["choices"][0].get("message", {}).get("content")
        return content or None
    except Exception:
        return None
