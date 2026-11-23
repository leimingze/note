# 脚本功能：从独立正常视频及 v2 E3 原视频提取挑帧器使用的事件数量与强度。
# 启动命令：PYTHONPATH=/root/autodl-tmp/rs-code-v3/worker/framesel python collect_p2.py
import hashlib
import json
from concurrent.futures import ProcessPoolExecutor
from pathlib import Path

from select_frames import EVASION_TYPES, cap_video_events, compute_signals, detect

# 保存历史输入与本轮输出的服务器磁盘。
ROOT = Path("/root/autodl-tmp")
# v2 实验记录用于还原输入版本。
OLD = ROOT / "rs-exp-v3"
# 正常视频的标定数量，与设计 P2 一致。
NORMAL_COUNT = 200


def inputs():
    """
    输入为服务器的 CH-SV 清单、v2 数据划分及 E3 逐条挑帧记录。
    输出为样本分组、视频标识和路径组成的列表。
    功能为排除已使用的视频并复用 E3 原片，例如 hard_1 保留为独立分组。
    """
    splits = json.loads((OLD / "work_e0/splits_materialized.json").read_text())
    used = {row["uid"].split(":", 1)[1] for group in splits.values() for row in group}
    records = sorted((OLD / "work_e3/e3a_frames_meta").glob("*.json"))
    variants = []
    for record in records:
        base, variant = record.stem.removeprefix("synth_").split("_", 1)
        used.add(base)
        if variant == "clean":
            continue
        video = OLD / "work_e3/synth_e3a" / (record.stem + ".mp4")
        if not video.is_file():
            video = ROOT / "rs-fulltest/m3/videos/exp" / base / (variant + ".mp4")
        if not video.is_file():
            raise FileNotFoundError(video)
        variants.append((variant, record.stem, str(video)))
    manifest = (ROOT / "rs-fulltest/chsv_manifest.jsonl").read_text().splitlines()
    candidates = [r for r in map(json.loads, manifest) if r["class"] == "normality"
                  and 0 < r["duration"] <= 20 and r["video_id"] not in used]
    candidates.sort(key=lambda r: hashlib.sha256(r["video_id"].encode()).digest())
    normal = [("normal", r["video_id"], r["path"]) for r in candidates[:NORMAL_COUNT]]
    if len({r[1] for r in normal}) != NORMAL_COUNT:
        raise ValueError("独立正常视频数量不足")
    return normal + variants


def measure(item):
    """
    输入为分组、视频标识和原视频路径。
    输出为视频对应的事件列表，读取或解码失败时报告错误。
    功能为运行 Worker 的挑帧计算，例如保留 local_change 的位置及强度。
    """
    group, uid, path = item
    found = detect(compute_signals(path))  # 从原视频计算生产挑帧器使用的信号。
    events = cap_video_events(found["events"])  # 使用服务默认的最大视频帧数。
    selected = [{"type": e["type"], "frame": int(e["index"]), "score": round(float(e["score"]), 4)}
                for e in events if e["type"] in EVASION_TYPES]
    return {"group": group, "uid": uid, "path": path, "events": selected}


if __name__ == "__main__":
    destination = ROOT / "rs-exp-v4/results/p2_events.jsonl"
    pending = destination.with_suffix(".partial")
    with ProcessPoolExecutor(max_workers=4) as pool, pending.open("w") as output:
        for row in pool.map(measure, inputs()):
            print(json.dumps(row, ensure_ascii=False), file=output, flush=True)
    pending.replace(destination)  # 全批完成后发布结果。
