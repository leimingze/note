#!/usr/bin/env python3
"""
合成插帧视频：把违规帧按 M3 的变体插进正常视频，并记录插入段作为标签。

变体（实现计划 M3）：
- clean：只按统一帧率重新编码，不插入，作为正常视频对照（测误触发，压缩条件与其他变体一致）
- hard_1 / hard_3 / hard_10：硬插 1 / 3 / 10 帧
- fade_30 / fade_50：0.5 秒渐入渐出的半透明整画面叠加，峰值透明度 30% / 50%
- pip：角落画中画，面积约 1/9，持续 1 秒

插帧前统一转成 30 fps 恒定帧率；逐帧流式解码、修改、编码，内存只占一帧；保留原音轨。
帧号从 0 开始，与 select_frames 的解码帧号一致。

命令行：python synth.py base.mp4 violation.jpg --variant hard_3 --position 4.2 --output out.mp4
"""
import argparse
import json
import subprocess
from pathlib import Path

import cv2
import numpy as np

TARGET_FPS = 30
FADE_SECONDS = 0.5
PIP_SECONDS = 1.0
VARIANTS = ("clean", "hard_1", "hard_3", "hard_10", "fade_30", "fade_50", "pip")
CORNERS = ("top_left", "top_right", "bottom_left", "bottom_right")


def probe(path: str) -> dict:
    """读取宽、高和时长；宽高取偶数，满足 yuv420p 编码要求。"""
    out = subprocess.run(
        ["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
         "stream=width,height:format=duration", "-of", "json", path],
        capture_output=True, text=True, check=True).stdout
    data = json.loads(out)
    stream = data["streams"][0]
    return {"width": int(stream["width"]) // 2 * 2, "height": int(stream["height"]) // 2 * 2,
            "duration": float(data["format"]["duration"])}


def insertion_frames(variant: str, fps: int = TARGET_FPS) -> int:
    """变体占用的帧数：硬插按帧数，渐变 0.5 秒，画中画 1 秒，clean 为 0。"""
    if variant.startswith("hard_"):
        return int(variant.split("_")[1])
    if variant.startswith("fade_"):
        return int(round(FADE_SECONDS * fps))
    if variant == "pip":
        return int(round(PIP_SECONDS * fps))
    return 0


def cover_resize(image: np.ndarray, width: int, height: int) -> np.ndarray:
    """等比缩放后居中裁剪到目标尺寸，不拉伸变形，也不留黑边（黑边会让插入画面过于显眼）。"""
    h, w = image.shape[:2]
    scale = max(width / w, height / h)
    resized = cv2.resize(image, (max(width, int(round(w * scale))), max(height, int(round(h * scale)))),
                         interpolation=cv2.INTER_AREA)
    y = (resized.shape[0] - height) // 2
    x = (resized.shape[1] - width) // 2
    return resized[y:y + height, x:x + width]


def fade_alphas(peak: float, count: int) -> np.ndarray:
    """三角形透明度曲线：前一半渐入、后一半渐出，中间帧达到峰值。"""
    j = np.arange(count) + 0.5
    return peak * (1.0 - np.abs(2.0 * j / count - 1.0))


def synthesize(base_video: str, violation_image: str, variant: str, output_path: str,
               position: float, corner: str = "bottom_right") -> dict:
    """
    输入：base_video，正常视频；violation_image，违规帧图片；variant，变体名；
          output_path，输出路径；position，插入起点（秒）；corner，画中画位置。
    输出：标签字典，包含插入段的起止帧号与时间；渐变变体另给"可见段"（透明度不低于峰值一半）。
    功能：流式生成一个合成视频；例如 hard_3、position=4.2 时第 126–128 帧被替换。
    """
    if variant not in VARIANTS:
        raise ValueError(f"unknown variant: {variant}")
    info = probe(base_video)
    width, height = info["width"], info["height"]
    span = insertion_frames(variant)
    start = int(round(position * TARGET_FPS))
    end = start + span - 1

    patch, pip_box, alphas = None, None, None
    if variant != "clean":
        source = cv2.imread(violation_image)
        if source is None:
            raise ValueError(f"cannot read violation image: {violation_image}")
        if variant == "pip":
            pw, ph = width // 3 // 2 * 2, height // 3 // 2 * 2
            margin = int(width * 0.02)
            x = margin if "left" in corner else width - pw - margin
            y = margin if "top" in corner else height - ph - margin
            pip_box = (x, y, pw, ph)
            patch = cover_resize(source, pw, ph)
        else:
            patch = cover_resize(source, width, height).astype(np.float32)
        if variant.startswith("fade_"):
            alphas = fade_alphas(int(variant.split("_")[1]) / 100.0, span)

    decoder = subprocess.Popen(
        ["ffmpeg", "-v", "error", "-threads", "2", "-i", base_video, "-map", "0:v:0",
         "-vf", f"fps={TARGET_FPS},scale={width}:{height}", "-f", "rawvideo", "-pix_fmt", "bgr24", "-"],
        stdout=subprocess.PIPE)
    encoder = subprocess.Popen(
        ["ffmpeg", "-v", "error", "-y", "-f", "rawvideo", "-pix_fmt", "bgr24", "-s", f"{width}x{height}",
         "-r", str(TARGET_FPS), "-i", "-", "-i", base_video, "-map", "0:v:0", "-map", "1:a:0?",
         "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p", "-threads", "2",
         "-c:a", "aac", "-b:a", "96k", "-movflags", "+faststart", output_path],
        stdin=subprocess.PIPE)

    frame_bytes = width * height * 3
    index = 0
    try:
        while True:
            raw = decoder.stdout.read(frame_bytes)
            if len(raw) < frame_bytes:
                break
            if start <= index <= end and patch is not None:
                frame = np.frombuffer(raw, np.uint8).reshape(height, width, 3).copy()
                if variant == "pip":
                    x, y, pw, ph = pip_box
                    frame[y:y + ph, x:x + pw] = patch
                elif alphas is not None:
                    a = alphas[index - start]
                    frame = (frame.astype(np.float32) * (1 - a) + patch * a).clip(0, 255).astype(np.uint8)
                else:
                    frame = patch.astype(np.uint8)
                raw = frame.tobytes()
            encoder.stdin.write(raw)
            index += 1
    finally:
        encoder.stdin.close()
        decoder.stdout.close()
        decoder.wait()
        encoder.wait()
    if encoder.returncode != 0 or decoder.returncode != 0:
        raise RuntimeError(f"ffmpeg failed for {base_video} ({variant})")
    if span and end >= index:
        raise ValueError(f"insertion [{start}, {end}] exceeds {index} frames")

    label = {"variant": variant, "base_video": base_video, "violation_image": violation_image if span else "",
             "output": output_path, "fps": TARGET_FPS, "width": width, "height": height,
             "total_frames": index, "start_frame": start if span else -1, "end_frame": end if span else -1}
    if span:
        label["start_time"] = round(start / TARGET_FPS, 4)
        label["end_time"] = round(end / TARGET_FPS, 4)
        visible_start, visible_end = start, end
        if alphas is not None:
            visible = np.nonzero(alphas >= alphas.max() / 2)[0]
            visible_start, visible_end = start + int(visible[0]), start + int(visible[-1])
        label["visible_start"], label["visible_end"] = visible_start, visible_end
        if pip_box:
            label["corner"], label["pip_box"] = corner, list(pip_box)
    return label


def main() -> None:
    parser = argparse.ArgumentParser(description="合成插帧视频")
    parser.add_argument("base_video")
    parser.add_argument("violation_image")
    parser.add_argument("--variant", choices=VARIANTS, required=True)
    parser.add_argument("--position", type=float, default=2.0, help="插入起点（秒）")
    parser.add_argument("--corner", choices=CORNERS, default="bottom_right")
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    label = synthesize(args.base_video, args.violation_image, args.variant, args.output,
                       args.position, args.corner)
    print(json.dumps(label, ensure_ascii=False))


if __name__ == "__main__":
    main()
