# 脚本功能：
# 为实验 0 v4 构造同等规模的调参集与留出集，并生成缺失合成视频和准确帧窗。
#
# 启动命令：
# /root/miniconda3/bin/python eval/v4/run_exp0_framesel.py --workers 8

from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any

from synthesis import build_synth_jobs

TARGET_BENIGN_COUNT = 228


def _item_video_id(item: dict[str, Any]) -> str:
    """
    输入：item，v2 数据切分中的单条样本。
    输出：用于去重的 CH-SV video_id；无法解析时返回空字符串。
    功能：兼容 `video_id`、`sample_id` 和带来源前缀的 `uid`，例如 `chsv:abc` 返回 `abc`。
    """
    raw = str(item.get("uid") or item.get("video_id") or item.get("sample_id") or "")
    return raw.split(":", 1)[-1]


def load_manifest(manifest_path: Path) -> tuple[list[dict[str, Any]], dict[str, dict[str, Any]]]:
    """
    输入：manifest_path，CH-SV 全量 JSONL 清单路径。
    输出：按文件原顺序返回的样本列表与 video_id 映射；JSON 无效时抛出异常。
    功能：一次读取清单供正常视频选样和合成底片选择共用，例如同一 video_id 可直接取得原视频路径。
    """
    rows = [
        json.loads(line)
        for line in manifest_path.read_text(encoding="utf-8").splitlines()
        if line.strip()
    ]
    return rows, {str(row["video_id"]): row for row in rows}


def _load_split_ids(splits_path: Path) -> set[str]:
    """
    输入：splits_path，v2 固定数据切分 JSON 路径。
    输出：已用于训练、校准和三份测试集的全部 CH-SV video_id。
    功能：建立数据隔离集合，例如新增留出正常视频不会与已有训练或测试样本重复。
    """
    splits = json.loads(splits_path.read_text(encoding="utf-8"))
    return {
        _item_video_id(item)
        for items in splits.values()
        for item in items
        if item.get("source") == "chsv"
    }


def _benign_job(split_name: str, item: dict[str, Any], manifest: dict[str, dict[str, Any]]) -> dict[str, Any] | None:
    """
    输入：split_name，来源切分；item，CH-SV 正常样本；manifest，按 video_id 索引的清单。
    输出：实验任务；路径不可用时返回 None。
    功能：把已有测试集中的正常样本转换为 v4 任务，例如测试集样本继续作为留出集的一部分。
    """
    video_id = _item_video_id(item)
    row = manifest.get(video_id) or {}
    path = str(row.get("path") or item.get("video_path") or "")
    if not path or not Path(path).is_file():
        return None
    return {
        "sample_id": f"{split_name}:{video_id}",
        "video_path": path,
        "partition": "holdout",
        "group": "benign_holdout",
        "variant": "benign",
        "base": "",
        "window": None,
    }


def select_benign_jobs(
    splits_path: Path,
    manifest: dict[str, dict[str, Any]],
    excluded_ids: set[str],
) -> list[dict[str, Any]]:
    """
    输入：splits_path，固定切分路径；manifest，CH-SV 清单映射；excluded_ids，合成视频占用的底片 ID。
    输出：调参集与留出集各 228 条正常视频任务，数量不符时抛出异常。
    功能：保留 v3 的 228 条调参正常样本和 123 条测试留出样本，再用未进入既有切分的正常视频补足留出集。
    """
    data = json.loads(splits_path.read_text(encoding="utf-8"))
    tune: list[dict[str, Any]] = []
    holdout: list[dict[str, Any]] = []
    seen: set[str] = set()
    for split_name in ("train", "calib"):
        for item in data[split_name]:
            video_id = _item_video_id(item)
            if item.get("source") != "chsv" or item.get("gold_violation") or video_id in seen:
                continue
            row = manifest.get(video_id) or {}
            path = str(row.get("path") or item.get("video_path") or "")
            if path and Path(path).is_file():
                seen.add(video_id)
                tune.append({
                    "sample_id": f"{split_name}:{video_id}",
                    "video_path": path,
                    "partition": "tune",
                    "group": "benign_tune",
                    "variant": "benign",
                    "base": "",
                    "window": None,
                })

    for split_name in ("test_full_240", "test_v1_32", "test_pure_benign_200"):
        for item in data[split_name]:
            video_id = _item_video_id(item)
            if item.get("source") != "chsv" or item.get("gold_violation") or video_id in {j["sample_id"].split(":", 1)[-1] for j in holdout}:
                continue
            job = _benign_job(split_name, item, manifest)
            if job:
                holdout.append(job)

    used_ids = _load_split_ids(splits_path) | excluded_ids
    candidates = []
    for video_id, row in manifest.items():
        duration = float(row.get("duration") or 0.0)
        if (
            video_id not in used_ids
            and row.get("eligible")
            and row.get("class") == "normality"
            and 0 < duration <= 20
            and Path(str(row.get("path") or "")).is_file()
        ):
            candidates.append(row)
    candidates.sort(key=lambda row: hashlib.sha256(f"v4-holdout:{row['video_id']}".encode()).hexdigest())
    for row in candidates[: max(0, TARGET_BENIGN_COUNT - len(holdout))]:
        holdout.append({
            "sample_id": f"test_extra:{row['video_id']}",
            "video_path": str(row["path"]),
            "partition": "holdout",
            "group": "benign_holdout",
            "variant": "benign",
            "base": "",
            "window": None,
        })

    if len(tune) != TARGET_BENIGN_COUNT or len(holdout) != TARGET_BENIGN_COUNT:
        raise RuntimeError(f"正常视频数量不符：tune={len(tune)}, holdout={len(holdout)}")
    return tune + holdout


def build_v4_jobs(
    splits_path: Path,
    manifest_path: Path,
    synth_dir: Path,
    legacy_synth_dir: Path,
) -> list[dict[str, Any]]:
    """
    输入：splits_path，固定数据切分；manifest_path，CH-SV 清单；synth_dir，v4 合成目录；legacy_synth_dir，v3 合成目录。
    输出：调参集与留出集各 298 条，共 596 条任务。
    功能：先确定合成底片以排除同源正常视频，再构造两个等规模分区；例如留出集为 228 条正常加 70 条合成。
    """
    rows, row_map = load_manifest(manifest_path)
    synth_jobs = build_synth_jobs(rows, synth_dir, legacy_synth_dir)
    synth_ids = {job["base"].removeprefix("synth_") for job in synth_jobs}
    benign_jobs = select_benign_jobs(splits_path, row_map, synth_ids)
    jobs = benign_jobs + synth_jobs
    counts = {
        partition: sum(job["partition"] == partition for job in jobs)
        for partition in ("tune", "holdout")
    }
    if counts != {"tune": 298, "holdout": 298}:
        raise RuntimeError(f"v4 分区规模不符：{counts}")
    normal_ids = {_item_video_id({"sample_id": job["sample_id"]}) for job in benign_jobs if job["partition"] == "tune"}
    held_ids = {_item_video_id({"sample_id": job["sample_id"]}) for job in benign_jobs if job["partition"] == "holdout"}
    if normal_ids & held_ids:
        raise RuntimeError("调参集与留出集存在正常视频重叠")
    return jobs
