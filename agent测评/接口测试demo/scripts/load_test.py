"""
脚本功能：
对指定接口做固定时长并发压测，输出 RPS、P50、P95、P99 与错误率。

启动命令：
.venv/bin/python scripts/load_test.py \
  --url http://127.0.0.1:30051/api/orders/90 \
  --header "X-User-Id: 10001" \
  --target-rps 200 --duration 20 --concurrency 10
"""

import argparse
import statistics
import threading
import time
from concurrent.futures import ThreadPoolExecutor

import requests


def parse_args() -> argparse.Namespace:
    """输入：命令行参数。输出：解析后的参数对象。功能：读取压测目标与参数。"""
    parser = argparse.ArgumentParser(description="接口压测与延迟统计")
    parser.add_argument("--url", required=True, help="被测接口完整 URL")
    parser.add_argument("--header", action="append", default=[], help="请求头，可重复，格式 Key: Value")
    parser.add_argument("--target-rps", type=int, default=200, help="目标每秒请求数")
    parser.add_argument("--duration", type=float, default=20.0, help="压测持续时间（秒）")
    parser.add_argument("--concurrency", type=int, default=10, help="并发线程数")
    return parser.parse_args()


def build_headers(header_args: list[str]) -> dict[str, str]:
    """输入：header_args，Key: Value 列表。输出：请求头字典。功能：解析命令行请求头。"""
    headers = {}
    for item in header_args:
        key, _, value = item.partition(":")
        headers[key.strip()] = value.strip()
    return headers


def run_load(url: str, headers: dict[str, str], duration: float, concurrency: int) -> dict:
    """
    输入：url，接口地址；headers，请求头；duration，持续秒数；concurrency，并发数。
    输出：请求数、RPS、延迟分位与错误率统计。
    功能：并发发送请求，按目标速率不限速地尽可能多打，记录每次耗时与结果。
    """
    latencies: list[float] = []
    errors = 0
    lock = threading.Lock()
    deadline = time.monotonic() + duration

    def worker() -> None:
        nonlocal errors
        session = requests.Session()
        while time.monotonic() < deadline:
            start = time.monotonic()
            try:
                response = session.get(url, headers=headers, timeout=5)
                ok = response.status_code < 500
            except requests.RequestException:
                ok = False
            elapsed = (time.monotonic() - start) * 1000
            with lock:
                latencies.append(elapsed)
                if not ok:
                    errors += 1

    with ThreadPoolExecutor(max_workers=concurrency) as pool:
        futures = [pool.submit(worker) for _ in range(concurrency)]
        time.sleep(duration + 1)
        for future in futures:
            future.result()

    elapsed = max(1.0, duration)
    total = len(latencies)
    sorted_latencies = sorted(latencies)
    return {
        "requests": total,
        "rps": total / elapsed,
        "p50_ms": statistics.median(latencies) if total else 0,
        "p95_ms": sorted_latencies[int(total * 0.95) - 1] if total else 0,
        "p99_ms": sorted_latencies[int(total * 0.99) - 1] if total else 0,
        "error_rate": errors / total if total else 1.0,
        "errors": errors,
    }


def main() -> None:
    """输入：无。输出：打印压测统计。功能：命令行入口，执行压测并输出结果。"""
    args = parse_args()
    result = run_load(args.url, build_headers(args.header), args.duration, args.concurrency)
    print(f"请求数: {result['requests']}")
    print(f"RPS: {result['rps']:.1f}")
    print(f"P50: {result['p50_ms']:.1f} ms")
    print(f"P95: {result['p95_ms']:.1f} ms")
    print(f"P99: {result['p99_ms']:.1f} ms")
    print(f"错误率: {result['error_rate']:.4%} ({result['errors']} 个错误)")


if __name__ == "__main__":
    main()
