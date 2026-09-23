#!/usr/bin/env python3
"""Query OfferLens interview records without sending resume data."""

from __future__ import annotations

import argparse
import json
import os
import sys
from datetime import date, timedelta
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen


DEFAULT_API_BASE = "https://skill.mnls.cloud"


def fetch(base_url: str, api_key: str, params: dict[str, str | int]) -> dict:
    url = f"{base_url.rstrip('/')}/api/v1/interviews?{urlencode(params)}"
    request = Request(url, headers={"X-API-Key": api_key, "User-Agent": "offerlens-interview-data/1.0"})
    try:
        with urlopen(request, timeout=30) as response:
            return json.load(response)
    except HTTPError as error:
        try:
            payload = json.load(error)
        except (json.JSONDecodeError, UnicodeDecodeError):
            payload = {"detail": {"message": error.reason}}
        detail = payload.get("detail", payload)
        message = detail.get("message", str(detail)) if isinstance(detail, dict) else str(detail)
        if isinstance(detail, dict) and detail.get("purchase_url"):
            message = f"{message}\n{detail['purchase_url']}"
        raise RuntimeError(message) from error
    except URLError as error:
        raise RuntimeError(f"无法连接 OfferLens API：{error.reason}") from error


def markdown(payload: dict) -> str:
    lines = [
        f"数据窗口：{payload.get('window_days', '自定义')} 天",
        f"匹配样本：{payload['total']}，本次返回：{len(payload['data'])}",
        f"访问类型：{payload['access']}",
    ]
    if payload.get("trial_remaining") is not None:
        lines.append(f"免费剩余：{payload['trial_remaining']} 次")
    for index, item in enumerate(payload["data"], 1):
        meta = " · ".join(filter(None, [item.get("company"), item.get("post"), item.get("edit_time")]))
        lines.extend((f"\n## {index}. {item.get('title') or '面经'}", meta, "", item.get("content") or ""))
    return "\n".join(lines)


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description="查询 OfferLens 真实面经原文")
    for name, help_text in (
        ("company", "公司名称"), ("post", "岗位名称"), ("role-group", "岗位大类"),
        ("role-family", "岗位方向"), ("keyword", "标题、正文或岗位关键词"),
        ("date-from", "开始日期 YYYY-MM-DD"), ("date-to", "结束日期 YYYY-MM-DD"),
    ):
        result.add_argument(f"--{name}", help=help_text)
    recent = result.add_mutually_exclusive_group()
    recent.add_argument("--days", type=int, help="查询最近 N 天")
    recent.add_argument("--adaptive-recent", action="store_true", help="按 90/180/365 天扩窗，直到至少 20 个匹配样本")
    result.add_argument("--limit", type=int, default=10, choices=range(1, 101), metavar="1-100")
    result.add_argument("--offset", type=int, default=0)
    result.add_argument("--format", choices=("json", "markdown"), default="json")
    result.add_argument("--api-base", default=os.getenv("OFFERLENS_API_BASE", DEFAULT_API_BASE))
    return result


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    params = {
        key: value for key, value in vars(args).items()
        if key in {"company", "post", "role_group", "role_family", "keyword", "date_from", "date_to"} and value
    }
    params.update(limit=args.limit, offset=args.offset)
    windows = (90, 180, 365) if args.adaptive_recent else (args.days,) if args.days else (None,)
    api_key = os.getenv("OFFERLENS_API_KEY", "offerlens")
    payload = None
    try:
        for days in windows:
            query = dict(params)
            if days:
                query["date_from"] = (date.today() - timedelta(days=days)).isoformat()
            payload = fetch(args.api_base, api_key, query)
            payload["window_days"] = days
            if not args.adaptive_recent or payload["total"] >= 20:
                break
    except RuntimeError as error:
        print(str(error), file=sys.stderr)
        return 2
    print(markdown(payload) if args.format == "markdown" else json.dumps(payload, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
