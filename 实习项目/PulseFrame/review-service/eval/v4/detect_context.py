# 脚本功能：
# 在基础挑帧事件上补充切镜边界前一帧，改善首个突变不明显的硬插帧覆盖率。
#
# 启动命令：
# 由 run_exp0_framesel.py 调用。

from __future__ import annotations

from framesel.select_frames import Thresholds, detect


def detect_with_boundary_context(signals: dict, thresholds: Thresholds) -> dict:
    """
    输入：signals，原始视频信号；thresholds，检测阈值。
    输出：基础事件、切镜边界前帧和更新后的计数；不修改输入对象。
    功能：当结束边界被识别为普通切镜时补采边界前帧；例如插入 3 帧时增加切镜前一帧作为候选。
    """
    found = detect(signals, thresholds)
    events = [dict(event) for event in found["events"]]
    existing = {int(event["index"]) for event in events}
    context = []
    for event in found["events"]:
        if event["type"] != "scene_change":
            continue
        index = int(event["index"]) - 1
        if index < 0 or index in existing:
            continue
        context.append({
            "index": index,
            "score": float(event["score"]),
            "type": "boundary_context",
            "boundary_index": int(event["index"]),
        })
        existing.add(index)
    counts = dict(found["counts"])
    counts["boundary_context"] = len(context)
    return {**found, "events": events + context, "counts": counts}
