# 脚本功能：
# 使用 v3 的阈值网格与选优规则运行实验 0 v4，并把调参集和留出集统一为 298 条。
#
# 启动命令：
# PYTHONPATH=/root/autodl-tmp/rs-code-v4/worker \
# /root/miniconda3/bin/python eval/v4/run_exp0_framesel.py \
#   --out-dir /root/autodl-tmp/rs-exp-v4/v4 --workers 8

from __future__ import annotations

import argparse
import itertools
import json
import sys
import time
from collections import defaultdict
from concurrent.futures import ProcessPoolExecutor, as_completed
from dataclasses import asdict
from pathlib import Path
from typing import Any

import numpy as np

REVIEW_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REVIEW_ROOT / "worker"))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from dataset import build_v4_jobs  # noqa: E402
from framesel.select_frames import EVASION_TYPES, Thresholds, compute_signals, detect  # noqa: E402

DEFAULT_V3_ROOT = Path("/root/autodl-tmp/rs-exp-v3")
DEFAULT_SPLITS = DEFAULT_V3_ROOT / "work_e0/splits_materialized.json"
DEFAULT_MANIFEST = Path("/root/autodl-tmp/rs-fulltest/chsv_manifest.jsonl")
DEFAULT_LEGACY_SYNTH_DIR = Path("/root/autodl-tmp/rs-exp-v3/work_e3/synth_e3a")
DEFAULT_OUT = Path("/root/autodl-tmp/rs-exp-v4/v4")
DEFAULT_V3_THRESHOLDS = Path("/root/autodl-tmp/rs-exp-v4/exp0/best_framesel_thresholds.json")
VARIANT_TYPES = {"hard_1": "brief_insertion", "hard_3": "brief_insertion", "hard_10": "brief_insertion",
                 "fade_30": "fade_overlay", "fade_50": "fade_overlay", "pip": "local_change"}
HARD_VARIANTS = ("hard_1", "hard_3", "hard_10")


def cache_signal(job: dict[str, Any], signal_dir: Path, legacy_dir: Path) -> tuple[str, str | None]:
    """
    输入：job，单条视频任务；signal_dir，v4 信号缓存目录；legacy_dir，v3 信号缓存目录。
    输出：成功时返回样本 ID 与缓存路径，失败时返回样本 ID 与错误。
    功能：每条视频只解码一次并保存信号；例如已缓存的同一样本会直接复用。
    """
    path = signal_dir / f"{job['sample_id'].replace(':', '_')}.npz"
    if path.is_file():
        return job["sample_id"], str(path)
    legacy = legacy_dir / path.name
    if legacy.is_file():
        return job["sample_id"], str(legacy)
    try:
        np.savez_compressed(path, **compute_signals(job["video_path"]))
        return job["sample_id"], str(path)
    except Exception as exc:
        return job["sample_id"], str(exc)[:200]


def load_signals(jobs: list[dict[str, Any]], signal_dir: Path, legacy_dir: Path, workers: int):
    """
    输入：jobs，视频任务；signal_dir，v4 缓存；legacy_dir，v3 缓存；workers，并行进程数。
    输出：成功任务、信号映射和失败记录。
    功能：并行计算新增信号并复用 v3 已有信号；例如原有 456 条可直接读取。
    """
    signal_dir.mkdir(parents=True, exist_ok=True)
    paths: dict[str, str] = {}
    failed = []
    with ProcessPoolExecutor(max_workers=workers) as pool:
        futures = {pool.submit(cache_signal, job, signal_dir, legacy_dir): job for job in jobs}
        for future in as_completed(futures):
            sample_id, result = future.result()
            if result and not result.endswith(".npz"):
                failed.append({"sample_id": sample_id, "error": result})
                continue
            paths[sample_id] = result
    ok_jobs = [job for job in jobs if job["sample_id"] in paths]
    signals = {}
    for job in ok_jobs:
        sample_id = job["sample_id"]
        path = Path(paths[sample_id])
        with np.load(path) as data:
            signals[sample_id] = {key: data[key] for key in data.files}
    return ok_jobs, signals, failed


def evaluate(jobs: list[dict[str, Any]], signals: dict, thresholds: Thresholds, partition: str) -> dict:
    """
    输入：jobs，全部任务；signals，原始信号；thresholds，检测阈值；partition，分区名。
    输出：正常误触、分类型召回、总体召回、精确率和 F1。
    功能：只把命中真实插入时段且事件类型正确的结果算作召回；例如渐变事件落在画中画区间不会记作命中。
    """
    benign_flags, cb_flags, evasion_counts = [], [], []
    by_variant: dict[str, list[bool]] = defaultdict(list)
    tp = fn = fp = tn = 0
    for job in (item for item in jobs if item["partition"] == partition):
        found = detect(signals[job["sample_id"]], thresholds)
        events = [event for event in found["events"] if event["type"] in EVASION_TYPES]
        count = len(events)
        if job["variant"] == "benign":
            triggered = count > 0
            benign_flags.append(triggered)
            cb_flags.append(bool(found["evasion_suspected"]))
            evasion_counts.append(count)
            fp += int(triggered)
            tn += int(not triggered)
            continue
        if job["variant"] == "clean":
            triggered = count > 0
            by_variant["clean"].append(triggered)
            cb_flags.append(bool(found["evasion_suspected"]))
            evasion_counts.append(count)
            fp += int(triggered)
            tn += int(not triggered)
            continue
        window = job["window"]
        if window is None:
            raise RuntimeError(f"攻击样本缺少帧窗：{job['sample_id']}")
        start, end = window
        expected = VARIANT_TYPES[job["variant"]]
        hit = any(
            event["type"] == expected and start - 1 <= int(event["index"]) <= end + 1
            for event in events
        )
        by_variant[job["variant"]].append(hit)
        tp += int(hit)
        fn += int(not hit)
    recall = tp / max(1, tp + fn)
    precision = tp / max(1, tp + fp)
    f1 = 2 * precision * recall / (precision + recall) if precision + recall else 0.0
    hard = [value for name in HARD_VARIANTS for value in by_variant[name]]
    pip_fade = [value for name in ("pip", "fade_30", "fade_50") for value in by_variant[name]]
    return {
        "partition": partition,
        "benign_count": len(benign_flags),
        "attack_count": tp + fn,
        "benign_fpr": round(float(np.mean(benign_flags)), 4),
        "circuit_breaker_fpr": round(float(np.mean(cb_flags)), 4),
        "benign_max_evasion_count": max(evasion_counts, default=0),
        "benign_p95_evasion_count": round(float(np.percentile(evasion_counts, 95)), 2),
        "hard_insertion_recall": round(float(np.mean(hard)), 4),
        "pip_fade_recall": round(float(np.mean(pip_fade)), 4),
        "overall_attack_recall": round(recall, 4),
        "overall_precision": round(precision, 4),
        "overall_f1": round(f1, 4),
        "variant_rates": {name: round(float(np.mean(values)), 4) for name, values in sorted(by_variant.items())},
        "confusion": {"tp": tp, "fn": fn, "fp": fp, "tn": tn},
    }


def select_best(candidates: list[dict[str, Any]], max_fpr: float = 0.20) -> dict[str, Any]:
    """
    输入：candidates，调参集网格结果；max_fpr，正常误触上限。
    输出：满足硬插召回和熔断约束后按总体 F1 排序的第一组。
    功能：保持 v3 的选优规则不变，确保 v4 只改变分区规模和评测数据。
    """
    eligible = [
        item for item in candidates
        if item["tune"]["circuit_breaker_fpr"] == 0
        and item["tune"]["hard_insertion_recall"] == 1
        and item["tune"]["benign_fpr"] <= max_fpr
    ]
    if not eligible:
        raise RuntimeError("没有阈值同时满足正常误触、熔断和硬插召回约束")
    return sorted(
        eligible,
        key=lambda item: (item["tune"]["overall_f1"], item["tune"]["pip_fade_recall"], -item["tune"]["benign_fpr"]),
        reverse=True,
    )[0]


def main() -> None:
    """
    输入：命令行参数，包括数据路径、输出目录和并行进程数。
    输出：v4 数据清单、信号缓存、阈值扫描结果和汇总 JSON。
    功能：完成等规模调参与留出评测；例如两个分区各输出 298 条的完整指标。
    """
    parser = argparse.ArgumentParser(description="实验 0 v4：等规模调参与留出评测")
    parser.add_argument("--splits", type=Path, default=DEFAULT_SPLITS)
    parser.add_argument("--manifest", type=Path, default=DEFAULT_MANIFEST)
    parser.add_argument("--synth-dir", type=Path)
    parser.add_argument("--legacy-synth-dir", type=Path, default=DEFAULT_LEGACY_SYNTH_DIR)
    parser.add_argument("--out-dir", type=Path, default=DEFAULT_OUT)
    parser.add_argument("--legacy-signals", type=Path, default=DEFAULT_OUT.parent / "exp0/signals")
    parser.add_argument("--v3-thresholds", type=Path, default=DEFAULT_V3_THRESHOLDS)
    parser.add_argument("--workers", type=int, default=8)
    args = parser.parse_args()

    started = time.time()
    args.out_dir.mkdir(parents=True, exist_ok=True)
    synth_dir = args.synth_dir or args.out_dir / "synth"
    jobs = build_v4_jobs(args.splits, args.manifest, synth_dir, args.legacy_synth_dir)
    (args.out_dir / "dataset_manifest.json").write_text(
        json.dumps(jobs, ensure_ascii=False, indent=2), encoding="utf-8",
    )
    ok_jobs, signals, failed = load_signals(jobs, args.out_dir / "signals", args.legacy_signals, args.workers)
    if failed:
        raise RuntimeError(f"信号计算失败 {len(failed)} 条：{failed[:3]}")

    grid = itertools.product(
        (0.02, 0.04, 0.06, 0.08, 0.10, 0.15),
        (True, False),
        (0.15, 0.20),
        (0.12, 0.15, 0.18),
        (0.06, 0.08, 0.10),
        (1, 2),
    )
    candidates = []
    for fade, local_peak, cut, block, quiet, max_blocks in grid:
        thresholds = Thresholds(
            cut=cut, fade=fade, block=block, quiet=quiet, max_blocks=max_blocks,
            evasion_count=15, local_fade_peak=local_peak,
        )
        candidates.append({
            "thresholds": asdict(thresholds),
            "tune": evaluate(ok_jobs, signals, thresholds, "tune"),
        })
    best = select_best(candidates)
    best_thresholds = Thresholds(**best["thresholds"])
    selected = {
        "thresholds": best["thresholds"],
        "tune": best["tune"],
        "holdout": evaluate(ok_jobs, signals, best_thresholds, "holdout"),
    }
    fixed = None
    if args.v3_thresholds.is_file():
        v3_thresholds = Thresholds(**json.loads(args.v3_thresholds.read_text(encoding="utf-8")))
        fixed = {
            "thresholds": asdict(v3_thresholds),
            "tune": evaluate(ok_jobs, signals, v3_thresholds, "tune"),
            "holdout": evaluate(ok_jobs, signals, v3_thresholds, "holdout"),
        }
    report = {
        "version": "v4",
        "elapsed_sec": round(time.time() - started, 1),
        "dataset_counts": {
            "tune": sum(job["partition"] == "tune" for job in jobs),
            "holdout": sum(job["partition"] == "holdout" for job in jobs),
        },
        "grid_size": len(candidates),
        "selected": selected,
        "v3_thresholds_replayed": fixed,
        "top10_tune": sorted(
            candidates,
            key=lambda item: (item["tune"]["overall_f1"], item["tune"]["pip_fade_recall"], -item["tune"]["benign_fpr"]),
            reverse=True,
        )[:10],
    }
    (args.out_dir / "exp0_v4_results.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8",
    )
    print(json.dumps(selected, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
