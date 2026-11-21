"""
挑帧与合成插帧的单元测试：现场生成一段缓慢平移的纹理视频作为"正常视频"，用 synth.py 插入
1/3/10 帧、0.5 秒半透明叠加、角落画中画，断言每类都被挑中、选中帧确实是插入画面、正常视频不误报。

运行：cd worker/framesel && python -m pytest -q test_select_frames.py
"""
import subprocess
import sys
from pathlib import Path

import cv2
import numpy as np
import pytest

sys.path.insert(0, str(Path(__file__).resolve().parent))
import select_frames  # noqa: E402
import synth  # noqa: E402

W, H, FPS, SECONDS = 360, 640, 30, 8


def _texture(seed: int, width: int, height: int) -> np.ndarray:
    """平滑的随机纹理，模拟有内容的画面。"""
    rng = np.random.default_rng(seed)
    noise = rng.random((height // 8, width // 8, 3)).astype(np.float32)
    return (cv2.resize(noise, (width, height), interpolation=cv2.INTER_CUBIC) * 200 + 20).clip(0, 255).astype(np.uint8)


def _write_video(path: Path, frames) -> None:
    proc = subprocess.Popen(
        ["ffmpeg", "-v", "error", "-y", "-f", "rawvideo", "-pix_fmt", "bgr24", "-s", f"{W}x{H}",
         "-r", str(FPS), "-i", "-", "-c:v", "libx264", "-preset", "veryfast", "-crf", "20",
         "-pix_fmt", "yuv420p", str(path)], stdin=subprocess.PIPE)
    for frame in frames:
        proc.stdin.write(frame.tobytes())
    proc.stdin.close()
    assert proc.wait() == 0


@pytest.fixture(scope="module")
def media(tmp_path_factory):
    root = tmp_path_factory.mktemp("framesel")
    big = _texture(1, W + FPS * SECONDS + 10, H)
    base = root / "base.mp4"
    # 每帧向右平移 1 像素，相邻帧差小但不为零，接近真实视频的缓慢运动。
    _write_video(base, (big[:, k:k + W] for k in range(FPS * SECONDS)))
    violation = root / "violation.jpg"
    red = np.zeros((H, W, 3), np.uint8)
    red[:, :, 2] = 230
    red[H // 4:H // 2, W // 4:W // 2] = (40, 200, 40)
    cv2.imwrite(str(violation), red)
    scene = root / "scene.mp4"
    other = 255 - _texture(7, W + FPS * SECONDS + 10, H)  # 反色，保证是明显的画面切换
    _write_video(scene, (big[:, k:k + W] if k < 4 * FPS else other[:, k:k + W] for k in range(FPS * SECONDS)))
    return {"root": root, "base": base, "violation": violation, "scene": scene}


def _selected_indices(result: dict) -> list:
    return [f["index"] for batch in result["batches"] for f in batch]


def _types(result: dict) -> dict:
    return {f["index"]: f["frame_type"] for batch in result["batches"] for f in batch}


@pytest.mark.parametrize("variant", ["hard_1", "hard_3", "hard_10"])
def test_hard_insertion_is_selected(media, variant):
    out = media["root"] / f"{variant}.mp4"
    label = synth.synthesize(str(media["base"]), str(media["violation"]), variant, str(out), position=3.0)
    result = select_frames.select_frames(str(out), media["root"] / f"{variant}_frames")
    hits = [i for i in _selected_indices(result) if label["start_frame"] <= i <= label["end_frame"]]
    assert hits, f"{variant}: no selected frame inside {label['start_frame']}..{label['end_frame']}"
    assert _types(result)[hits[0]] == "brief_insertion"
    # 选中的帧必须是插入的红色画面，不能是相邻的正常帧。
    frame = next(f for b in result["batches"] for f in b if f["index"] == hits[0])
    image = cv2.imread(frame["frame_path"])
    assert image[..., 2].mean() > 150 and image[..., 0].mean() < 100


@pytest.mark.parametrize("variant", ["fade_30", "fade_50"])
def test_fade_overlay_is_selected(media, variant):
    out = media["root"] / f"{variant}.mp4"
    label = synth.synthesize(str(media["base"]), str(media["violation"]), variant, str(out), position=3.0)
    result = select_frames.select_frames(str(out), media["root"] / f"{variant}_frames")
    hits = [i for i in _selected_indices(result) if label["visible_start"] <= i <= label["visible_end"]]
    assert hits, f"{variant}: counts={result['counts']}"
    assert _types(result)[hits[0]] == "fade_overlay"


def test_pip_is_selected_with_crop(media):
    out = media["root"] / "pip.mp4"
    label = synth.synthesize(str(media["base"]), str(media["violation"]), "pip", str(out), position=3.0)
    result = select_frames.select_frames(str(out), media["root"] / "pip_frames")
    frames = [f for b in result["batches"] for f in b
              if label["start_frame"] <= f["index"] <= label["end_frame"] and f["frame_type"] == "local_change"]
    assert frames, f"counts={result['counts']}"
    assert Path(frames[0]["crop_path"]).is_file()


def test_clean_video_has_no_evasion_frames(media):
    out = media["root"] / "clean.mp4"
    synth.synthesize(str(media["base"]), str(media["violation"]), "clean", str(out), position=3.0)
    result = select_frames.select_frames(str(out), media["root"] / "clean_frames")
    counts = result["counts"]
    assert counts["brief_insertion"] == counts["local_change"] == counts["fade_overlay"] == 0, counts
    assert not result["evasion_suspected"]
    assert counts["timed"] == SECONDS // 2


def test_scene_change_is_selected(media):
    result = select_frames.select_frames(str(media["scene"]), media["root"] / "scene_frames")
    types = _types(result)
    assert any(t == "scene_change" and abs(i - 4 * FPS) <= 1 for i, t in types.items()), types
    assert result["counts"]["brief_insertion"] == 0


def test_batches_keep_all_frames_and_unique_files(media):
    out = media["root"] / "hard_3_batch.mp4"
    synth.synthesize(str(media["base"]), str(media["violation"]), "hard_3", str(out), position=3.0)
    result = select_frames.select_frames(str(out), media["root"] / "batch_frames", max_per_batch=2)
    assert len(result["batches"]) >= 2
    paths = [f["frame_path"] for b in result["batches"] for f in b]
    assert len(paths) == len(set(paths)) and all(Path(p).is_file() for p in paths)
    # 第一批装优先级最高的短暂插入帧；每批内按时间排序。
    assert any(f["frame_type"] == "brief_insertion" for f in result["batches"][0])
    for batch in result["batches"]:
        times = [f["timestamp"] for f in batch]
        assert times == sorted(times)


def test_synth_labels_and_audio_free_base(media):
    out = media["root"] / "label_check.mp4"
    label = synth.synthesize(str(media["base"]), str(media["violation"]), "fade_50", str(out), position=2.0)
    assert label["start_frame"] == 60 and label["end_frame"] == 60 + 15 - 1
    assert label["start_frame"] <= label["visible_start"] <= label["visible_end"] <= label["end_frame"]
    assert label["total_frames"] == FPS * SECONDS
