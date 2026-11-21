#!/usr/bin/env python3
"""
M3 挑帧实验：在 CH-SV 全部 20 秒以内的正常视频上合成插帧变体，对比"均匀抽帧（每 2 秒一帧）"和 M2 挑帧。

指标（实现计划 M3）：
- 挑帧选中率：插入段里至少有一帧被选中的视频比例，按变体分别报；渐变叠加的插入段取透明度不低于峰值一半的帧。
- 正常视频误触发：clean 变体（同样重新编码、不插入）上的平均选中帧数、evasion_suspected 比例。

步骤：
  synth      对每个正常视频生成 7 个变体（含 clean），同时算好挑帧信号缓存为 npz
  calibrate  在标定集上网格搜索阈值，约束正常视频误触发，写出 thresholds.json
  evaluate   用标定好的阈值在实验集上出报告

python -I experiment.py synth --manifest M --pool P --out-dir D --workers 9
python -I experiment.py calibrate --out-dir D --workers 18
python -I experiment.py evaluate --out-dir D --report report.md
"""
import os

# 每个进程只用 1 个计算线程：numpy/OpenCV 默认按机器核数（208）开线程，多进程叠加会超出容器的线程上限，
# 导致 ffmpeg 或 numpy 建线程失败；实验靠多进程并行，不需要单进程多线程。
for _name in ("OMP_NUM_THREADS", "OPENBLAS_NUM_THREADS", "MKL_NUM_THREADS"):
    os.environ.setdefault(_name, "1")

import argparse
import hashlib
import itertools
import json
import random
import subprocess
import sys
from collections import defaultdict
from concurrent.futures import ProcessPoolExecutor
from dataclasses import asdict, replace
from pathlib import Path

import numpy as np

sys.path.insert(0, str(Path(__file__).resolve().parent))
import select_frames as sf  # noqa: E402
import synth  # noqa: E402

sf.cv2.setNumThreads(1)

VARIANTS = synth.VARIANTS
ATTACKS = [v for v in VARIANTS if v != "clean"]
MAX_EVASION_RATE = 0.05   # 标定约束：正常视频 evasion_suspected 比例不超过 5%。
MAX_CLEAN_FRAMES = 16.0   # 标定约束：正常视频平均选中帧数不超过一批（16 帧）。


def _seed(text: str) -> int:
    return int(hashlib.sha256(text.encode()).hexdigest()[:12], 16)


# ---------- synth ----------

def _decode_errors(path: str) -> str:
    """完整解码一遍，返回 ffmpeg 报的错误；为空表示文件完好。"""
    result = subprocess.run(["ffmpeg", "-v", "error", "-threads", "1", "-i", path, "-f", "null", "-"],
                            capture_output=True, text=True)
    return result.stderr.strip()


def _synth_one(job: dict) -> list:
    """
    对一个正常视频生成全部变体并缓存信号；已存在的文件跳过，可断点续跑。
    先写到带进程号的临时文件，完整解码校验通过后再原子替换到正式路径：曾有残留进程和重跑进程同时写同一个文件，
    ffmpeg 退出码为 0 但视频从中途开始损坏，挑帧只看到前几秒。
    """
    row, pool, out = job["row"], job["pool"], Path(job["out"])
    rng = random.Random(_seed(row["video_id"]))
    frame = rng.choice(pool)
    corner = rng.choice(synth.CORNERS)
    longest = max(synth.insertion_frames(v) for v in ATTACKS) / synth.TARGET_FPS
    room = row["duration"] - 1.0 - longest - 0.2
    position = round(rng.uniform(1.0, max(1.0, room)), 3)
    results = []
    for variant in VARIANTS:
        name = f"{row['video_id']}_{variant}"
        video = out / "videos" / row["split"] / row["video_id"] / f"{variant}.mp4"
        label_path = out / "labels" / f"{name}.json"
        signal_path = out / "signals" / f"{name}.npz"
        if label_path.exists() and signal_path.exists():
            continue
        video.parent.mkdir(parents=True, exist_ok=True)
        temp = video.with_name(f".{variant}.{os.getpid()}.mp4")
        try:
            label = synth.synthesize(row["path"], frame["path"], variant, str(temp), position, corner)
            errors = _decode_errors(str(temp))
            if errors:
                raise RuntimeError(f"synthesized video does not decode cleanly: {errors[:120]}")
            os.replace(temp, video)
            label.update({"output": str(video), "video_id": row["video_id"], "split": row["split"],
                          "duration": row["duration"],
                          "violation_id": frame["id"] if variant != "clean" else "",
                          "violation_class": frame["class"] if variant != "clean" else "",
                          "position": position})
            np.savez_compressed(signal_path, **sf.compute_signals(str(video)))
            label_path.write_text(json.dumps(label, ensure_ascii=False))
            results.append((name, "ok"))
        except Exception as error:  # 单个视频失败记录下来，不中断全量
            temp.unlink(missing_ok=True)
            results.append((name, f"error: {str(error)[:200]}"))
    return results


def cmd_synth(args) -> None:
    out = Path(args.out_dir)
    for sub in ("labels", "signals"):
        (out / sub).mkdir(parents=True, exist_ok=True)
    rows = [json.loads(line) for line in open(args.manifest, encoding="utf-8")]
    rows = [r for r in rows if r["class"] == "normality" and r["eligible"] and r["duration"] >= 3.5]
    if args.limit:
        rows = rows[:args.limit]
    pool = json.loads(Path(args.pool).read_text(encoding="utf-8"))
    jobs = [{"row": r, "pool": [p for p in pool if p["split"] == r["split"]], "out": str(out)} for r in rows]
    errors = []
    with ProcessPoolExecutor(args.workers) as executor:
        for k, results in enumerate(executor.map(_synth_one, jobs, chunksize=1), start=1):
            errors += [r for r in results if r[1] != "ok"]
            if k % 50 == 0:
                print(f"{k}/{len(jobs)} base videos, errors={len(errors)}", flush=True)
    (out / "synth_errors.json").write_text(json.dumps(errors, ensure_ascii=False, indent=1))
    print(f"done: {len(jobs)} base videos, errors={len(errors)}")


# ---------- shared evaluation ----------

def load_items(out: Path, split: str) -> list:
    items = []
    for label_path in sorted((out / "labels").glob("*.json")):
        label = json.loads(label_path.read_text(encoding="utf-8"))
        if label["split"] != split:
            continue
        signal_path = out / "signals" / f"{label_path.stem}.npz"
        if signal_path.exists():
            items.append((label, str(signal_path)))
    return items


def _hit_range(label: dict) -> tuple:
    return label.get("visible_start", -1), label.get("visible_end", -1)


def evaluate_item(label: dict, signals: dict, th: sf.Thresholds) -> dict:
    """单个视频：均匀抽帧与挑帧各自是否选中插入段、选中帧数、批次和规避标记。"""
    times = signals["times"]
    found = sf.detect(signals, th)
    batches = sf.plan_batches(found["events"], times)
    selected = [e for b in batches for e in b]
    uniform = [e["index"] for e in found["events"] if e["type"] == "timed"]
    lo, hi = _hit_range(label)
    hit_types = sorted({e["type"] for e in selected if lo <= e["index"] <= hi})
    return {
        "variant": label["variant"], "video_id": label["video_id"],
        "violation_class": label.get("violation_class", ""),
        "uniform_hit": any(lo <= i <= hi for i in uniform), "uniform_frames": len(uniform),
        "select_hit": bool(hit_types), "first_batch_hit": any(lo <= e["index"] <= hi for e in batches[0]) if batches else False,
        "hit_types": hit_types, "select_frames": len(selected), "batches": len(batches),
        "evasion": bool(found["evasion_suspected"]), "counts": found["counts"],
    }


def _evaluate_many(payload) -> list:
    items, th_dict = payload
    th = sf.Thresholds(**th_dict)
    results = []
    for label, signal_path in items:
        with np.load(signal_path) as data:
            signals = {k: data[k] for k in data.files}
        results.append(evaluate_item(label, signals, th))
    return results


def run_parallel(items: list, th: sf.Thresholds, workers: int) -> list:
    chunks = [items[k::workers] for k in range(workers)]
    with ProcessPoolExecutor(workers) as executor:
        parts = executor.map(_evaluate_many, [(c, asdict(th)) for c in chunks if c])
    return [r for part in parts for r in part]


def summarize(results: list) -> dict:
    by_variant = defaultdict(list)
    for r in results:
        by_variant[r["variant"]].append(r)
    summary = {}
    for variant, rows in by_variant.items():
        n = len(rows)
        summary[variant] = {
            "n": n,
            "uniform_hit": sum(r["uniform_hit"] for r in rows) / n,
            "select_hit": sum(r["select_hit"] for r in rows) / n,
            "first_batch_hit": sum(r["first_batch_hit"] for r in rows) / n,
            "uniform_frames": float(np.mean([r["uniform_frames"] for r in rows])),
            "select_frames": float(np.mean([r["select_frames"] for r in rows])),
            "batches2plus": sum(r["batches"] > 1 for r in rows) / n,
            "evasion_rate": sum(r["evasion"] for r in rows) / n,
            "anomaly_events": float(np.mean([r["counts"]["brief_insertion"] + r["counts"]["local_change"]
                                             + r["counts"]["fade_overlay"] for r in rows])),
        }
    return summary


def _objective(summary: dict) -> tuple:
    """约束满足时按攻击变体平均选中率排序；不满足的组合排在后面。"""
    clean = summary.get("clean", {})
    ok = clean.get("evasion_rate", 1) <= MAX_EVASION_RATE and clean.get("select_frames", 99) <= MAX_CLEAN_FRAMES
    attack = np.mean([summary[v]["select_hit"] for v in ATTACKS if v in summary])
    return (ok, attack, -clean.get("select_frames", 99))


# ---------- calibrate ----------

def cmd_calibrate(args) -> None:
    out = Path(args.out_dir)
    items = load_items(out, "calib")
    print(f"calibration videos: {len(items)}")
    base = sf.Thresholds()
    stages = [
        ("cut/back", lambda: [dict(cut=c, back=b, back_ratio=r) for c, b, r in
                              itertools.product([0.08, 0.10, 0.12, 0.15, 0.20], [0.04, 0.06, 0.08], [0.4, 0.5, 0.6])],
         {"clean", "hard_1", "hard_3", "hard_10"}),
        ("fade", lambda: [dict(fade=f) for f in [0.012, 0.015, 0.02, 0.025, 0.03, 0.04, 0.05, 0.06]],
         {"clean", "fade_30", "fade_50"}),
        ("local", lambda: [dict(block=b, quiet=q, max_blocks=m) for b, q, m in
                           itertools.product([0.08, 0.10, 0.12, 0.15, 0.20], [0.02, 0.03, 0.04, 0.06], [1, 2, 3])],
         {"clean", "pip"}),
    ]
    log = []
    for name, grid, variants in stages:
        subset = [it for it in items if it[0]["variant"] in variants]
        scored = []
        for change in grid():
            th = replace(base, **change)
            summary = summarize(run_parallel(subset, th, args.workers))
            scored.append((_objective(summary), change, summary))
        scored.sort(key=lambda x: x[0], reverse=True)
        best = scored[0]
        base = replace(base, **best[1])
        log.append({"stage": name, "best": best[1], "summary": best[2],
                    "top5": [{"change": c, "objective": [bool(o[0]), float(o[1]), float(o[2])]} for o, c, _ in scored[:5]]})
        print(name, "best", best[1], json.dumps(best[2], ensure_ascii=False))

    # 联合检查：三类检测器一起开时正常视频的规避误报会叠加，超标就逐步收紧 evasion_count。
    final = summarize(run_parallel(items, base, args.workers))
    while final["clean"]["evasion_rate"] > MAX_EVASION_RATE and base.evasion_count < 10:
        base = replace(base, evasion_count=base.evasion_count + 1)
        final = summarize(run_parallel(items, base, args.workers))
    (out / "thresholds.json").write_text(json.dumps(asdict(base), indent=1))
    (out / "calibration.json").write_text(json.dumps({"stages": log, "final": final, "thresholds": asdict(base)},
                                                     ensure_ascii=False, indent=1))
    print("final thresholds", asdict(base))
    print(json.dumps(final, ensure_ascii=False, indent=1))


# ---------- evaluate ----------

def cmd_evaluate(args) -> None:
    out = Path(args.out_dir)
    th = sf.Thresholds(**json.loads((out / "thresholds.json").read_text()))
    items = load_items(out, "exp")
    results = run_parallel(items, th, args.workers)
    (out / "exp_results.jsonl").write_text("\n".join(json.dumps(r, ensure_ascii=False) for r in results))
    summary = summarize(results)
    calib = json.loads((out / "calibration.json").read_text())["final"] if (out / "calibration.json").exists() else {}
    (out / "exp_summary.json").write_text(json.dumps({"exp": summary, "calib": calib, "thresholds": asdict(th)},
                                                     ensure_ascii=False, indent=1))

    def pct(x):
        return f"{100 * x:.1f}%"

    lines = ["| 变体 | 视频数 | 均匀抽帧选中率 | M2 挑帧选中率 | 其中第一批即选中 | 均匀抽帧帧数 | M2 平均帧数 | 需要第二批 | evasion_suspected |",
             "|---|---:|---:|---:|---:|---:|---:|---:|---:|"]
    for variant in VARIANTS:
        s = summary.get(variant)
        if not s:
            continue
        lines.append(f"| {variant} | {s['n']} | {pct(s['uniform_hit']) if variant != 'clean' else '—'} | "
                     f"{pct(s['select_hit']) if variant != 'clean' else '—'} | "
                     f"{pct(s['first_batch_hit']) if variant != 'clean' else '—'} | {s['uniform_frames']:.1f} | "
                     f"{s['select_frames']:.1f} | {pct(s['batches2plus'])} | {pct(s['evasion_rate'])} |")
    hit_by_type = defaultdict(lambda: defaultdict(int))
    for r in results:
        for t in r["hit_types"]:
            hit_by_type[r["variant"]][t] += 1
    attribution = ["| 变体 | " + " | ".join(sf.PRIORITY) + " |", "|---|" + "---:|" * len(sf.PRIORITY)]
    for variant in ATTACKS:
        n = summary.get(variant, {}).get("n", 0) or 1
        attribution.append(f"| {variant} | " + " | ".join(pct(hit_by_type[variant][t] / n) for t in sf.PRIORITY) + " |")
    Path(args.report).write_text("\n".join(["## 实验集结果", "", *lines, "", "## 选中插入段的检测器（可多选）", "",
                                             *attribution, ""]), encoding="utf-8")
    print("\n".join(lines))
    print("\n".join(attribution))


def main() -> None:
    parser = argparse.ArgumentParser(description="M3 挑帧实验")
    sub = parser.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("synth")
    p.add_argument("--manifest", required=True)
    p.add_argument("--pool", required=True)
    p.add_argument("--out-dir", required=True)
    p.add_argument("--workers", type=int, default=9)
    p.add_argument("--limit", type=int, default=0)
    p = sub.add_parser("calibrate")
    p.add_argument("--out-dir", required=True)
    p.add_argument("--workers", type=int, default=18)
    p = sub.add_parser("evaluate")
    p.add_argument("--out-dir", required=True)
    p.add_argument("--report", required=True)
    p.add_argument("--workers", type=int, default=18)
    args = parser.parse_args()
    {"synth": cmd_synth, "calibrate": cmd_calibrate, "evaluate": cmd_evaluate}[args.cmd](args)


if __name__ == "__main__":
    main()
