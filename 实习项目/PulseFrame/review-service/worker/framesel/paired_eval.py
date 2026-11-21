# 脚本功能：在固定原视频上配对评估新旧挑帧，核验旧版结果，输出逐视频记录。
# 启动命令：python paired_eval.py /root/autodl-tmp/rs-fulltest paired.jsonl
import argparse
import hashlib
import json
from dataclasses import replace
from pathlib import Path

import numpy as np

import experiment

# 按原视频抽样，一个原视频的全部变体共用分组，种子与检测参数无关。
SAMPLE_SIZE = 100
SAMPLE_SEED = "m2-paired-20261121"


def coverage_hit(label: dict, signals, old) -> bool:
    """
    输入：插入区间标注、解码信号和旧版阈值。
    输出：在距离已有选帧最远处补一帧后，该帧是否命中。
    功能：对照等量增加送审帧的效果，例如优先补选长时间没有选帧的位置。
    """
    events = experiment.sf.detect(signals, old)["events"]  # 读取旧版选帧位置。
    times = signals["times"]
    selected = times[sorted({event["index"] for event in events})]
    right = np.searchsorted(selected, times)  # 找到每帧左右最近的已选时刻。
    before = selected[np.maximum(right - 1, 0)]
    after = selected[np.minimum(right, len(selected) - 1)]
    index = np.argmax(np.minimum(np.abs(times - before), np.abs(after - times)))
    return bool(label["visible_start"] <= index <= label["visible_end"])


def source_ids(root: Path) -> list[str]:
    """
    输入：root，含原评估集及三轮审核记录的目录。
    输出：按固定哈希顺序选出的原视频标识，数量不足时抛出异常。
    功能：排除标定、审核调试及教学示例，例如同一底片的六种变体只能一起入选。
    """
    excluded = {"3x22uyzejnpiuk4"}  # 已展示并查看过的教学示例。
    for run in ("e2e", "e2e2", "e2e3"):
        for line in (root / run / "cases.jsonl").read_text().splitlines():
            row = json.loads(line)  # 从各轮送审记录读取原视频标识。
            if row["source"] == "synthetic":
                excluded.add(row["base_video"])
    ids = {p.name for p in (root / "m3/videos/exp").iterdir() if p.is_dir()} - excluded
    ordered = sorted(ids, key=lambda name: hashlib.sha256(f"{SAMPLE_SEED}/{name}".encode()).digest())
    if len(ordered) < SAMPLE_SIZE:
        raise ValueError(f"only {len(ordered)} eligible sources")
    return ordered[:SAMPLE_SIZE]


def main() -> None:
    """
    输入：命令行指定数据根目录和输出路径。
    输出：生成逐视频 JSONL，缺少变体或旧版结果与历史记录不符时抛出错误。
    功能：对同一组解码信号运行两种规则，例如每条正常视频同时记录两版送审帧数。
    """
    parser = argparse.ArgumentParser()
    parser.add_argument("root", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    dataset = args.root / "m3"
    old = experiment.sf.Thresholds(**json.loads((dataset / "thresholds.json").read_text()), local_fade_peak=False)
    new = replace(old, local_fade_peak=True)  # 仅启用新增规则，沿用原有阈值。
    history = [json.loads(s) for s in (dataset / "exp_results.jsonl").read_text().splitlines()]
    reference = {(r["video_id"], r["variant"]): r for r in history}
    rows = []
    for name in source_ids(args.root):
        for variant in experiment.VARIANTS:
            stem = f"{name}_{variant}"
            label = json.loads((dataset / "labels" / f"{stem}.json").read_text())
            with np.load(dataset / "signals" / f"{stem}.npz") as signals:
                baseline = experiment.evaluate_item(label, signals, old)
                candidate = experiment.evaluate_item(label, signals, new)
                control = baseline["select_hit"]
                if not control and candidate["select_frames"] > baseline["select_frames"] and variant != "clean":
                    control = coverage_hit(label, signals, old)  # 同一条视频使用相同补选额度。
            if baseline != reference[name, variant]:
                raise ValueError(f"baseline mismatch: {stem}")
            rows.append({"id": name, "variant": variant, "old": baseline, "new": candidate, "coverage_hit": control})
    args.output.write_text("".join(json.dumps(row) + "\n" for row in rows), encoding="utf-8")


if __name__ == "__main__":
    main()
