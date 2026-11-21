# 脚本功能：检测视频中的短暂插入、镜头切换、局部变化和渐变叠加，将候选帧与定时帧分批导出。
# 启动命令：python select_frames.py video.mp4 --output-dir out/
import argparse
import json
import re
import subprocess
from dataclasses import asdict, dataclass, fields
from functools import lru_cache
from pathlib import Path

import cv2
import numpy as np

ANALYSIS_LONG, ANALYSIS_SHORT = 160, 90  # 帧差分析用的缩略尺寸，保持横竖方向。
OUTPUT_LONG_SIDE = 768                   # 送审帧的长边上限，控制每帧的视觉 token 数。
TIMED_INTERVAL = 2.0                     # 定时帧间隔（秒）。

PRIORITY = {"brief_insertion": 4, "local_change": 3, "fade_overlay": 2, "scene_change": 1, "timed": 0}
EVASION_TYPES = ("brief_insertion", "local_change", "fade_overlay")


@dataclass
class Thresholds:
    """挑帧阈值，默认值是 M3 在 CH-SV 标定集上网格搜索的结果（见 reports/frame-selection.md）。"""
    cut: float = 0.15          # 相邻帧平均灰度差超过它算画面突变（0–1）。
    back: float = 0.04         # 突变后与突变前一帧的差低于它算"变回原画面"。
    back_ratio: float = 0.6    # 或者：与突变前一帧的差不到"第二次突变幅度"的 0.6，也算变回（容忍画面本身在动）。
    brief_window: float = 0.5  # 变回窗口（秒）：0.5 秒内变回算短暂插入。
    fade: float = 0.02         # 残差减去局部基线后超过它算渐变叠加。
    block: float = 0.15        # 九宫格单块帧差超过它算该块突变。
    quiet: float = 0.06        # 其余块的帧差都低于它，才算"只有个别块突变"。
    max_blocks: int = 1        # 最多几个块同时突变还算局部变化。
    evasion_count: int = 5     # 三类规避帧合计超过它标记 evasion_suspected。
    local_fade_peak: bool = True  # 每条视频最多补选一处尚未覆盖的渐变峰值，关闭后可复现原版。


@lru_cache(maxsize=1)
def _vsync_args() -> tuple:
    """ffmpeg 5.1 起用 -fps_mode，旧版本用 -vsync；两者都表示原样输出每一帧，不补帧也不丢帧。"""
    out = subprocess.run(["ffmpeg", "-version"], capture_output=True, text=True).stdout
    match = re.search(r"ffmpeg version n?(\d+)", out)
    major = int(match.group(1)) if match else 4
    return ("-fps_mode", "passthrough") if major >= 5 else ("-vsync", "passthrough")


def probe_video(video_path: str) -> dict:
    """
    输入：video_path，视频路径。
    输出：宽、高、帧率、时长和视频流数量；不是可解码的视频时抛出 RuntimeError。
    功能：给挑帧和媒体校验提供基础信息；例如 720×1280、30 fps 的竖屏视频返回 width=720。
    """
    cmd = ["ffprobe", "-v", "error", "-show_entries",
           "format=duration:stream=codec_type,width,height,avg_frame_rate,r_frame_rate",
           "-of", "json", video_path]
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        raise RuntimeError(f"ffprobe failed: {result.stderr.strip()[:200]}")
    data = json.loads(result.stdout or "{}")
    videos = [s for s in data.get("streams", []) if s.get("codec_type") == "video"]
    if not videos:
        raise RuntimeError("no video stream")

    def rate(value: str) -> float:
        num, _, den = (value or "0/1").partition("/")
        den_f = float(den or 1)
        return float(num) / den_f if den_f else 0.0

    video = videos[0]
    fps = rate(video.get("avg_frame_rate")) or rate(video.get("r_frame_rate")) or 25.0
    return {
        "width": int(video["width"]), "height": int(video["height"]), "fps": fps,
        "duration": float(data.get("format", {}).get("duration") or 0.0),
        "video_streams": len(videos),
    }


def analysis_size(width: int, height: int) -> tuple:
    """横屏缩到 160×90，竖屏缩到 90×160，保证九宫格对应画面的同一块区域。"""
    return (ANALYSIS_LONG, ANALYSIS_SHORT) if width >= height else (ANALYSIS_SHORT, ANALYSIS_LONG)


def decode_small(video_path: str, width: int, height: int) -> np.ndarray:
    """
    输入：video_path，视频路径；width、height，缩略尺寸。
    输出：形状为 (帧数, 高, 宽, 3) 的 float32 RGB 数组，取值 0–1。
    功能：一次解码全部帧，供所有检测器共用；例如 20 秒 30 fps 视频返回 600 帧。
    """
    cmd = ["ffmpeg", "-v", "error", "-i", video_path, "-map", "0:v:0", *_vsync_args(),
           "-vf", f"scale={width}:{height}:flags=area", "-f", "rawvideo", "-pix_fmt", "rgb24", "-"]
    raw = subprocess.run(cmd, capture_output=True, check=True).stdout
    frames = np.frombuffer(raw, np.uint8).reshape(-1, height, width, 3)
    return frames.astype(np.float32) / 255.0


def frame_times(video_path: str, count: int, fps: float) -> np.ndarray:
    """
    输入：video_path，视频路径；count，解码得到的帧数；fps，平均帧率。
    输出：每帧相对第一帧的展示时间（秒）。
    功能：从数据包 pts 读取真实时间，兼容可变帧率；数量对不上时退回 i / fps。
    """
    cmd = ["ffprobe", "-v", "error", "-select_streams", "v:0",
           "-show_entries", "packet=pts_time", "-of", "csv=p=0", video_path]
    out = subprocess.run(cmd, capture_output=True, text=True).stdout
    values = sorted(float(v) for v in re.findall(r"-?\d+(?:\.\d+)?", out))
    if len(values) != count:
        return np.arange(count, dtype=np.float64) / fps
    times = np.asarray(values, dtype=np.float64)
    return times - times[0]


def _block_diffs(gray: np.ndarray) -> np.ndarray:
    """相邻帧在 3×3 九宫格上的分块平均差，形状 (帧数, 9)，第 0 帧为 0。"""
    n, h, w = gray.shape
    ys = np.linspace(0, h, 4).astype(int)
    xs = np.linspace(0, w, 4).astype(int)
    diff = np.abs(gray[1:] - gray[:-1])
    blocks = np.zeros((n, 9), dtype=np.float32)
    for r in range(3):
        for c in range(3):
            blocks[1:, r * 3 + c] = diff[:, ys[r]:ys[r + 1], xs[c]:xs[c + 1]].mean(axis=(1, 2))
    return blocks


def _residuals(thumbs: np.ndarray, fps: float) -> np.ndarray:
    """
    每帧减去前后各 1 秒画面的逐像素中位数背景，返回平均残差（附录 A 的渐变叠加信号）。
    在 16×9 彩色缩略图上计算：整画面的半透明叠加会改变每一格的颜色，而镜头平移、人物走动
    对缩略图影响很小；全分辨率的中位数背景在画面运动时本身就有很大残差，会把叠加淹没。
    """
    n = len(thumbs)
    half = max(1, int(round(fps * 1.0)))
    out = np.zeros(n, dtype=np.float32)
    for i in range(n):
        lo, hi = max(0, i - half), min(n, i + half + 1)
        if hi - lo < 5:
            continue
        window = np.concatenate([thumbs[lo:i], thumbs[i + 1:hi]])
        background = np.median(window, axis=0)
        out[i] = float(np.abs(thumbs[i] - background).mean())
    return out


def _rolling_median(values: np.ndarray, half: int) -> np.ndarray:
    """一维滑动中位数，用作残差的局部基线，扣掉持续运动带来的稳定残差。"""
    n = len(values)
    out = np.empty(n, dtype=np.float32)
    for i in range(n):
        out[i] = np.median(values[max(0, i - half):min(n, i + half + 1)])
    return out


def compute_signals(video_path: str) -> dict:
    """
    输入：video_path，视频路径。
    输出：检测所需的全部原始信号；可直接 np.savez 缓存，供标定时反复套用不同阈值。
    功能：一次解码同时算出相邻帧差、短暂插入的回落距离、渐变残差和九宫格分块帧差。
    """
    info = probe_video(video_path)
    width, height = analysis_size(info["width"], info["height"])
    frames = decode_small(video_path, width, height)
    n = len(frames)
    if n == 0:
        raise RuntimeError("video has no decodable frames")
    fps = info["fps"]
    times = frame_times(video_path, n, fps)
    gray = frames @ np.array([0.299, 0.587, 0.114], dtype=np.float32)

    adjacent = np.zeros(n, dtype=np.float32)
    if n > 1:
        adjacent[1:] = np.abs(gray[1:] - gray[:-1]).mean(axis=(1, 2))

    # 16×9 彩色缩略图：判断"变回原画面"和渐变残差都用它，对平移和小幅运动不敏感。
    h, w = gray.shape[1:]
    thumbs = frames.reshape(n, h // 10, 10, w // 10, 10, 3).mean(axis=(2, 4))
    thumb_adjacent = np.zeros(n, dtype=np.float32)
    if n > 1:
        thumb_adjacent[1:] = np.abs(thumbs[1:] - thumbs[:-1]).mean(axis=(1, 2, 3))

    # 第 i 帧突变后，窗口内第 i+k+1 帧与突变前一帧（第 i-1 帧）的差：
    # back 用全分辨率灰度（判断"几乎相同"），back_thumb 用缩略图（判断"大致回到原画面"）。
    window = max(1, int(round(fps * 0.5)))
    back = np.full((n, window), np.inf, dtype=np.float32)
    back_thumb = np.full((n, window), np.inf, dtype=np.float32)
    for i in np.nonzero(adjacent > 0.05)[0]:  # 只给有一定突变的帧计算，省时间。
        if i == 0:
            continue
        stop = min(n, i + window + 1)
        if stop > i + 1:
            back[i, :stop - i - 1] = np.abs(gray[i + 1:stop] - gray[i - 1]).mean(axis=(1, 2))
            back_thumb[i, :stop - i - 1] = np.abs(thumbs[i + 1:stop] - thumbs[i - 1]).mean(axis=(1, 2, 3))

    residual = _residuals(thumbs, fps)
    baseline = _rolling_median(residual, max(1, int(round(fps * 2.0))))
    return {
        "fps": np.float64(fps), "times": times, "adjacent": adjacent, "thumb_adjacent": thumb_adjacent,
        "back": back, "back_thumb": back_thumb, "residual": residual, "baseline": baseline,
        "blocks": _block_diffs(gray),
        "width": np.int64(info["width"]), "height": np.int64(info["height"]),
        "duration": np.float64(info["duration"]),
    }


def _merge_events(events: list, gap_frames: int) -> list:
    """相距小于 gap_frames 的同类事件只保留分数最高的一个，避免一次快速运动挑出一串帧。"""
    merged = []
    for event in sorted(events, key=lambda e: e["index"]):
        if merged and event["index"] - merged[-1]["index"] < gap_frames:
            if event["score"] > merged[-1]["score"]:
                merged[-1] = event
            continue
        merged.append(event)
    return merged


def supplemental_fade(signals: dict, existing: list, th: Thresholds) -> list:
    """
    输入：视频信号、已选事件和挑帧配置，时间以秒为单位。
    输出：空列表或一个渐变事件，输入数据保持原样。
    功能：在两个时间尺度寻找先升后降的残差，例如连续移动背景上短暂出现的叠加图。
    """
    times = signals["times"]
    residual = signals["residual"]
    # 前缀计数用于排除跨越镜头切换的比较区间。
    cuts = np.cumsum(signals["adjacent"] > th.cut)
    strength = np.zeros_like(residual)  # 每一帧在不同时间尺度上的最高得分。
    for radius in (th.brief_window / 2, th.brief_window):
        left = np.maximum(0, np.searchsorted(times, times - radius, side="right") - 1)
        right = np.minimum(len(times) - 1, np.searchsorted(times, times + radius))
        valid = (times - radius >= times[0]) & (times + radius <= times[-1]) & (cuts[left] == cuts[right])
        prominence = residual - np.maximum(residual[left], residual[right])  # 两端都低于中心才构成峰。
        np.maximum(strength, prominence, out=strength, where=valid)
    # 已送审的邻近帧已能展示同一次变化，将补选额度留给其余片段。
    for event in existing:
        center = times[event["index"]]
        lo, hi = np.searchsorted(times, [center - th.brief_window / 4, center + th.brief_window / 4])
        strength[lo:hi] = 0
    best = int(np.argmax(strength))  # 取整段中最明显的一处，限制正常视频的额外送审成本。
    if strength[best] <= th.fade:
        return []
    return [{"index": best, "score": float(strength[best]), "type": "fade_overlay"}]


def detect(signals: dict, th: Thresholds = Thresholds()) -> dict:
    """
    输入：signals，compute_signals 的结果；th，阈值。
    输出：各类事件列表（帧号、分数、类型）和规避标记，不读写任何文件。
    功能：按附录 A 的五条规则挑帧；例如插入 1 帧异色画面时得到一个 brief_insertion 事件。
    """
    fps = float(signals["fps"])
    adjacent, back, back_thumb = signals["adjacent"], signals["back"], signals["back_thumb"]
    thumb_adjacent = signals["thumb_adjacent"]
    residual, baseline, blocks = signals["residual"], signals["baseline"], signals["blocks"]
    times = signals["times"]
    n = len(adjacent)
    window = max(1, int(round(fps * th.brief_window)))
    window = min(window, back.shape[1])
    gap = max(1, int(round(fps * 0.5)))

    # 1. 短暂插入与画面切换：突变后 0.5 秒内变回 → 短暂插入，取插入段中间一帧；不变回 → 画面切换。
    #    "变回"有两种判法：与突变前一帧几乎相同；或者那一帧本身又是一次突变，且它离突变前的画面
    #    比离上一帧近得多（插入 10 帧期间原视频还在动，不能要求完全相同）。
    brief, scenes, covered = [], [], np.zeros(n, dtype=bool)
    i = 1
    while i < n:
        if adjacent[i] <= th.cut:
            i += 1
            continue
        ks = np.arange(window)
        js = np.minimum(i + ks + 1, n - 1)
        second_cut = (adjacent[js] > th.cut) & (back_thumb[i, :window] < th.back_ratio * thumb_adjacent[js])
        returned = np.nonzero((back[i, :window] < th.back) | second_cut)[0]
        if len(returned):
            end = i + int(returned[0])          # 插入段最后一帧
            # 取插入段中与插入前画面差异最大的一帧：插入段整段相同时等价于任取一帧；
            # 插入紧挨着视频自带的闪切转场时，避免取到转场帧而漏掉真正插入的画面。
            distance = np.concatenate([[adjacent[i]], back[i, :end - i]])
            pick = i + int(np.argmax(distance))
            brief.append({"index": pick, "score": float(adjacent[i]), "type": "brief_insertion",
                          "start": i, "end": end})
            covered[max(0, i - 1):end + 2] = True
            i = end + 2
            continue
        scenes.append({"index": i, "score": float(adjacent[i]), "type": "scene_change"})
        i += 1
    scenes = _merge_events(scenes, gap)

    # 2. 渐变叠加：残差扣掉局部基线后超过阈值的时段取最大的一帧；画面切换前后 1 秒背景失效，不判。
    excess = residual - baseline
    near_cut = np.zeros(n, dtype=bool)
    one_sec = max(1, int(round(fps)))
    for event in scenes:
        near_cut[max(0, event["index"] - one_sec):event["index"] + one_sec + 1] = True
    hot = (excess > th.fade) & ~near_cut & ~covered
    fades, start = [], None
    for idx in range(n + 1):
        if idx < n and hot[idx]:
            start = idx if start is None else start
            continue
        if start is not None:
            seg = np.arange(start, idx)
            best = int(seg[np.argmax(excess[seg])])
            fades.append({"index": best, "score": float(excess[best]), "type": "fade_overlay"})
            start = None
    fades = _merge_events(fades, int(round(fps * 2.0)))

    # 3. 局部变化：只有 1–max_blocks 个块突变、其余块平稳；同一组块在 2 秒内先变后复原时取中间帧。
    locals_raw = []
    for idx in range(1, n):
        if covered[idx]:
            continue
        changed = np.nonzero(blocks[idx] > th.block)[0]
        if not 1 <= len(changed) <= th.max_blocks:
            continue
        others = np.delete(blocks[idx], changed)
        if len(others) and others.max() >= th.quiet:
            continue
        locals_raw.append({"index": idx, "score": float(blocks[idx][changed].max()),
                           "type": "local_change", "blocks": [int(b) for b in changed]})
    local_events, used = [], set()
    pair_gap = int(round(fps * 2.0))
    for k, event in enumerate(locals_raw):
        if k in used:
            continue
        partner = None
        for m in range(k + 1, len(locals_raw)):
            other = locals_raw[m]
            if other["index"] - event["index"] > pair_gap:
                break
            if other["blocks"] == event["blocks"]:
                partner = m
                break
        if partner is not None:
            used.add(partner)
            event = dict(event, index=(event["index"] + locals_raw[partner]["index"] - 1) // 2)
        local_events.append(event)
    local_events = _merge_events(local_events, gap)

    # 4. 定时帧：每 2 秒一帧。
    timed = []
    for k in range(int(times[-1] // TIMED_INTERVAL) + 1):
        idx = int(np.searchsorted(times, k * TIMED_INTERVAL))
        if idx < n:
            timed.append({"index": idx, "score": 0.0, "type": "timed"})

    if th.local_fade_peak:
        fades.extend(supplemental_fade(signals, brief + local_events + fades + scenes + timed, th))
    evasion_events = len(brief) + len(local_events) + len(fades)
    return {
        "events": brief + local_events + fades + scenes + timed,
        "counts": {"brief_insertion": len(brief), "local_change": len(local_events),
                   "fade_overlay": len(fades), "scene_change": len(scenes), "timed": len(timed)},
        "evasion_suspected": evasion_events > th.evasion_count,
    }


def plan_batches(events: list, times: np.ndarray, max_per_batch: int = 16) -> list:
    """
    输入：events，detect 得到的事件；times，每帧时间；max_per_batch，每批帧数上限。
    输出：批次列表，每批内按时间排序；同一帧被多个检测器选中时只保留优先级最高的类型。
    功能：按原视频帧号升序分批，例如 25 个不同帧依次分成 16 帧和 9 帧。
    """
    best = {}
    for event in events:
        old = best.get(event["index"])
        if old is None or (PRIORITY[event["type"]], event["score"]) > (PRIORITY[old["type"]], old["score"]):
            best[event["index"]] = event
    ordered = sorted(best.values(), key=lambda e: e["index"])
    batches = []
    for start in range(0, len(ordered), max_per_batch):
        chunk = ordered[start:start + max_per_batch]
        batches.append([dict(e, timestamp=round(float(times[e["index"]]), 3)) for e in chunk])
    return batches


def _resize_long(image: np.ndarray, long_side: int) -> np.ndarray:
    """长边超过 long_side 时等比缩小，用于控制送审图片大小。"""
    h, w = image.shape[:2]
    scale = long_side / max(h, w)
    if scale >= 1:
        return image
    return cv2.resize(image, (int(round(w * scale)), int(round(h * scale))), interpolation=cv2.INTER_AREA)


def _block_box(blocks: list, width: int, height: int) -> tuple:
    """把九宫格块号换算成原分辨率下的裁剪框 (x1, y1, x2, y2)，四周多留 5% 余量。"""
    xs = np.linspace(0, width, 4).astype(int)
    ys = np.linspace(0, height, 4).astype(int)
    rows = [b // 3 for b in blocks]
    cols = [b % 3 for b in blocks]
    x1, x2 = xs[min(cols)], xs[max(cols) + 1]
    y1, y2 = ys[min(rows)], ys[max(rows) + 1]
    pad_x, pad_y = int(width * 0.05), int(height * 0.05)
    return max(0, x1 - pad_x), max(0, y1 - pad_y), min(width, x2 + pad_x), min(height, y2 + pad_y)


def extract_frames(video_path: str, indices: list, width: int, height: int) -> dict:
    """
    输入：video_path，视频；indices，要取的帧号；width、height，原分辨率。
    输出：帧号 → BGR 原分辨率图像。
    功能：按帧号精确取帧（不按时间 seek），保证插入 1 帧的画面不会取成相邻帧。
    """
    wanted = sorted(set(indices))
    if not wanted:
        return {}
    expr = "+".join(f"eq(n\\,{i})" for i in wanted)
    cmd = ["ffmpeg", "-v", "error", "-i", video_path, "-map", "0:v:0", *_vsync_args(),
           "-vf", f"select={expr}", "-f", "rawvideo", "-pix_fmt", "bgr24", "-"]
    raw = subprocess.run(cmd, capture_output=True, check=True).stdout
    size = width * height * 3
    images = [np.frombuffer(raw[k * size:(k + 1) * size], np.uint8).reshape(height, width, 3)
              for k in range(len(raw) // size)]
    return dict(zip(wanted, images))


def select_frames(video_path: str, output_dir: Path, max_per_batch: int = 16,
                  th: Thresholds = Thresholds(), signals: dict | None = None) -> dict:
    """
    输入：video_path，视频；output_dir，输出目录；max_per_batch，每批帧数；th，阈值。
    输出：批次、规避标记、各类计数等；选中帧和局部放大图写入 output_dir。
    功能：线上流程使用的完整挑帧；例如角落画中画会额外输出一张变化区域的放大图。
    """
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    signals = signals or compute_signals(video_path)
    found = detect(signals, th)
    batches = plan_batches(found["events"], signals["times"], max_per_batch)
    width, height = int(signals["width"]), int(signals["height"])
    images = extract_frames(video_path, [e["index"] for b in batches for e in b], width, height)

    out_batches = []
    for b, batch in enumerate(batches):
        out = []
        for event in batch:
            image = images.get(event["index"])
            if image is None:
                continue
            path = output_dir / f"b{b}_f{event['index']:05d}.jpg"
            cv2.imwrite(str(path), _resize_long(image, OUTPUT_LONG_SIDE), [cv2.IMWRITE_JPEG_QUALITY, 90])
            frame = {"index": event["index"], "timestamp": event["timestamp"], "frame_type": event["type"],
                     "score": round(event["score"], 4), "frame_path": str(path)}
            if event["type"] == "local_change":
                x1, y1, x2, y2 = _block_box(event["blocks"], width, height)
                crop = image[y1:y2, x1:x2]
                zoom = OUTPUT_LONG_SIDE / max(crop.shape[:2])
                crop = cv2.resize(crop, None, fx=zoom, fy=zoom, interpolation=cv2.INTER_CUBIC)
                crop_path = output_dir / f"b{b}_f{event['index']:05d}_crop.jpg"
                cv2.imwrite(str(crop_path), crop, [cv2.IMWRITE_JPEG_QUALITY, 90])
                frame["crop_path"] = str(crop_path)
            out.append(frame)
        out_batches.append(out)

    return {
        "batches": out_batches,
        "evasion_suspected": bool(found["evasion_suspected"]),
        "counts": found["counts"],
        "fps": round(float(signals["fps"]), 3),
        "duration": round(float(signals["duration"]), 3),
        "total_frames": int(len(signals["adjacent"])),
        "thresholds": asdict(th),
    }


def thresholds_from_json(text: str | None) -> Thresholds:
    """命令行或接口传入的阈值 JSON 覆盖默认值，未知字段报错。"""
    if not text:
        return Thresholds()
    data = json.loads(text)
    names = {f.name for f in fields(Thresholds)}
    unknown = set(data) - names
    if unknown:
        raise ValueError(f"unknown thresholds: {sorted(unknown)}")
    return Thresholds(**data)


def main() -> None:
    parser = argparse.ArgumentParser(description="视频挑帧模块")
    parser.add_argument("video", help="输入视频路径")
    parser.add_argument("--output-dir", default="output", help="输出目录")
    parser.add_argument("--max-per-batch", type=int, default=16, help="每批最多帧数")
    parser.add_argument("--thresholds", help="覆盖默认阈值的 JSON，例如 '{\"cut\": 0.15}'")
    args = parser.parse_args()

    output_dir = Path(args.output_dir)
    result = select_frames(args.video, output_dir, args.max_per_batch, thresholds_from_json(args.thresholds))
    text = json.dumps(result, ensure_ascii=False)
    (output_dir / "result.json").write_text(text, encoding="utf-8")
    print(text)


if __name__ == "__main__":
    main()
