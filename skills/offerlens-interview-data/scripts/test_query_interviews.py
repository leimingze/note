#!/usr/bin/env python3
import importlib.util
from pathlib import Path


path = Path(__file__).with_name("query_interviews.py")
spec = importlib.util.spec_from_file_location("query_interviews", path)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

payload = {
    "data": [{"company": "字节跳动", "post": "后端", "title": "面经", "content": "完整原文", "edit_time": "2026-08-22"}],
    "total": 23,
    "access": "trial",
    "trial_remaining": 9,
    "window_days": 90,
}
output = module.markdown(payload)
assert "匹配样本：23" in output
assert "完整原文" in output
assert "免费剩余：9 次" in output
assert module.parser().parse_args(["--adaptive-recent"]).adaptive_recent
print("ok")
