# 脚本功能：
# 为实验 0 v4 选择 20 个合成底片，复用 v3 已生成视频，并补齐缺失的 5 个底片。
#
# 启动命令：
# 由 run_exp0_framesel.py 调用。

from __future__ import annotations

import subprocess
from pathlib import Path
from typing import Any

import cv2
import numpy as np

VARIANTS = ("clean", "hard_1", "hard_3", "hard_10", "fade_30", "fade_50", "pip")
ATTACK_WINDOWS = {
    "hard_1": (0, 0),
    "hard_3": (0, 2),
    "hard_10": (0, 9),
    "fade_30": (0, 11),
    "fade_50": (0, 11),
    "pip": (0, 14),
}
TUNE_SYNTH_BASES = 10
HOLDOUT_SYNTH_BASES = 10
SYNTH_FPS = 25
MAX_SYNTH_SOURCE_FRAMES = 225
MIN_SYNTH_FRAMES = 200


def _source_frame_count(path: str) -> int:
    """
    输入：path，合成底片原视频路径。
    输出：按 v3 生成方式最多读取 225 帧时的帧数。
    功能：还原每条 v3 合成视频的中心插入帧，例如 213 帧原片得到中心帧 106。
    """
    cap = cv2.VideoCapture(path)
    if not cap.isOpened():
        raise RuntimeError(f"无法打开合成底片：{path}")
    decoded = 0
    while decoded < MAX_SYNTH_SOURCE_FRAMES and cap.grab():
        decoded += 1
    cap.release()
    return decoded


def _read_frames(path: str, limit: int, size: tuple[int, int] | None = None) -> list[np.ndarray]:
    """
    输入：path，视频路径；limit，最多读取帧数；size，可选的宽高。
    输出：按原顺序读取的 BGR 帧列表；视频无法解码时抛出异常。
    功能：为合成视频读取正常底片和违规片段，例如读取 225 帧时返回不超过 225 个数组。
    """
    cap = cv2.VideoCapture(path)
    if not cap.isOpened():
        raise RuntimeError(f"无法打开视频：{path}")
    frames = []
    while len(frames) < limit:
        ok, frame = cap.read()
        if not ok:
            break
        frames.append(cv2.resize(frame, size) if size else frame)
    cap.release()
    if not frames:
        raise RuntimeError(f"视频没有可解码帧：{path}")
    return frames


def _transcode_mp4(raw_path: Path, output_path: Path) -> None:
    """
    输入：raw_path，OpenCV 生成的临时视频；output_path，最终 H.264 MP4 路径。
    输出：写入最终视频并删除临时文件；编码失败时抛出异常。
    功能：使用 ffmpeg 统一编码，例如把 mp4v 临时视频转成浏览器可解码的 H.264。
    """
    subprocess.run(
        ["ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", str(raw_path),
         "-c:v", "libx264", "-preset", "veryfast", "-crf", "21", "-pix_fmt", "yuv420p", str(output_path)],
        check=True,
    )
    raw_path.unlink(missing_ok=True)


def _apply_variant(frame: np.ndarray, variant: str, offset: int,
                   violation: list[np.ndarray]) -> np.ndarray:
    """
    输入：frame，正常帧；variant，合成版本；offset，距离中心帧的偏移；violation，违规片段。
    输出：修改后的单帧；不需要修改时返回原帧。
    功能：逐帧插入硬插、渐变或画中画，例如画中画从中心帧开始连续覆盖 15 帧。
    """
    if variant == "hard_1" and offset == 0:
        return violation[0]
    if variant == "hard_3" and offset < 3:
        return violation[offset % len(violation)]
    if variant == "hard_10" and offset < 10:
        return violation[offset % len(violation)]
    if variant in ("fade_30", "fade_50") and offset < 12:
        alpha = 0.30 if variant == "fade_30" else 0.50
        return cv2.addWeighted(frame, 1.0 - alpha, violation[offset % len(violation)], alpha, 0)
    if variant == "pip" and offset < 15:
        output = frame.copy()
        patch_w, patch_h = 720 // 3, 1280 // 3
        patch = cv2.resize(violation[offset % len(violation)], (patch_w, patch_h))
        output[1280 - patch_h - 20:1280 - 20, 720 - patch_w - 20:720 - 20] = patch
        return output
    return frame


def _generate_variant(normal_path: str, violation: list[np.ndarray], variant: str,
                      middle: int, output_path: Path) -> None:
    """
    输入：normal_path，正常底片；violation，违规片段；variant，合成版本；middle，插入中心帧；output_path，输出路径。
    输出：生成单个合成视频；源视频无法读取时抛出异常。
    功能：逐帧读取并写入视频，避免把 720×1280 的整段视频同时保存在内存中。
    """
    cap = cv2.VideoCapture(normal_path)
    if not cap.isOpened():
        raise RuntimeError(f"无法打开合成底片：{normal_path}")
    output_path.parent.mkdir(parents=True, exist_ok=True)
    raw_path = output_path.with_suffix(".raw.mp4")
    writer = cv2.VideoWriter(str(raw_path), cv2.VideoWriter_fourcc(*"mp4v"), SYNTH_FPS, (720, 1280))
    if not writer.isOpened():
        raise RuntimeError(f"无法创建视频：{raw_path}")
    index = 0
    last = None
    while index < MAX_SYNTH_SOURCE_FRAMES:
        ok, frame = cap.read()
        if not ok:
            break
        last = cv2.resize(frame, (720, 1280))
        writer.write(_apply_variant(last, variant, index - middle, violation))
        index += 1
    cap.release()
    if last is None:
        writer.release()
        raw_path.unlink(missing_ok=True)
        raise RuntimeError(f"底片没有可解码帧：{normal_path}")
    while index < MIN_SYNTH_FRAMES:
        writer.write(_apply_variant(last, variant, index - middle, violation))
        index += 1
    writer.release()
    _transcode_mp4(raw_path, output_path)


def _generate_group(normal_row: dict[str, Any], violation_row: dict[str, Any], output_dir: Path) -> None:
    """
    输入：normal_row，正常底片清单行；violation_row，违规片段清单行；output_dir，合成视频目录。
    输出：生成 7 个变体视频；源文件不足时抛出异常。
    功能：沿用 v3 E3 的 200–225 帧、中心插入方式，例如画中画持续 15 帧。
    """
    violation = _read_frames(str(violation_row["path"]), 15)
    violation = [cv2.resize(frame, (720, 1280)) for frame in violation]
    middle = max(_source_frame_count(str(normal_row["path"])), MIN_SYNTH_FRAMES) // 2
    base_id = f"synth_{normal_row['video_id']}"
    for variant in VARIANTS:
        _generate_variant(
            str(normal_row["path"]), violation, variant, middle,
            output_dir / f"{base_id}_{variant}.mp4",
        )


def build_synth_jobs(
    manifest_rows: list[dict[str, Any]],
    synth_dir: Path,
    legacy_synth_dir: Path,
) -> list[dict[str, Any]]:
    """
    输入：manifest_rows，CH-SV 清单原序列；synth_dir，v4 合成目录；legacy_synth_dir，v3 合成目录。
    输出：20 个底片 × 7 变体的任务，并补齐缺失视频；底片数量不足时抛出异常。
    功能：保留 v3 前 15 个底片，再追加 5 个同规则底片，使两个分区各有 10 个合成底片。
    """
    eligible = [
        row for row in manifest_rows
        if row.get("eligible")
        and 7.0 <= float(row.get("duration") or 0.0) <= 15.0
        and Path(str(row.get("path") or "")).is_file()
    ]
    normals = [row for row in eligible if row.get("class") == "normality"]
    violations = [row for row in eligible if row.get("class") in ("violence", "vulgarity")]
    total = TUNE_SYNTH_BASES + HOLDOUT_SYNTH_BASES
    if len(normals) < total or len(violations) < total:
        raise RuntimeError(f"合成底片不足：正常={len(normals)}, 违规={len(violations)}, 需要={total}")

    jobs = []
    for index in range(total):
        normal_row = normals[index]
        violation_row = violations[index]
        base_id = f"synth_{normal_row['video_id']}"
        legacy_paths = {variant: legacy_synth_dir / f"{base_id}_{variant}.mp4" for variant in VARIANTS}
        v4_paths = {variant: synth_dir / f"{base_id}_{variant}.mp4" for variant in VARIANTS}
        if all(path.is_file() for path in legacy_paths.values()):
            video_paths = legacy_paths
        else:
            if not all(path.is_file() for path in v4_paths.values()):
                _generate_group(normal_row, violation_row, synth_dir)
            video_paths = v4_paths
        middle = max(_source_frame_count(str(normal_row["path"])), MIN_SYNTH_FRAMES) // 2
        partition = "tune" if index < TUNE_SYNTH_BASES else "holdout"
        for variant in VARIANTS:
            start, end = ATTACK_WINDOWS.get(variant, (-1, -1))
            jobs.append({
                "sample_id": f"{base_id}_{variant}",
                "video_path": str(video_paths[variant]),
                "partition": partition,
                "group": "synthetic",
                "variant": variant,
                "base": base_id,
                "window": None if variant == "clean" else [middle + start, middle + end],
            })
    return jobs
