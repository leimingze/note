# 脚本功能：核对 50 条验证样本的离线分数与 Worker 输出，记录 E3 送审基准。
# 启动命令：在服务器执行 PYTHONPATH=/root/autodl-tmp/rs-code-v3/eval/v2 python check_gate.py
import hashlib
import json
from pathlib import Path

import torch
from common_v2 import has_injection, post_worker
from run_v2_e0 import extract_gate_feature
from risk_gate.models import MILMaxGuardedGate, predict_samples
from risk_gate.service import EVASION_FRAME_TYPES
import sys

# 与 Worker 使用同一版文字规则表。
sys.path.insert(0, "/root/autodl-tmp/rs-code-v3/worker/media_text")
from text_rules import check_text

# v2 缓存、权重及 v3 输出位置。
OLD = Path("/root/autodl-tmp/rs-exp-v3")
RESULTS = Path("/root/autodl-tmp/rs-exp-v4/results")
# P3 设计规定的抽样规模与最大允许分差。
SAMPLE_COUNT = 50
TOLERANCE = 0.001


def comparison(item, model, threshold):
    """
    输入为已提取帧和文字的样本、离线模型及放行阈值。
    输出为离线和在线的分数与决定，读取缓存并访问 Worker。
    功能为比较同一输入的两条计算路径，例如记录某行是否同时放行。
    """
    cache = OLD / "work_e0/gate_npz"
    cache_file = cache / (item["uid"].replace(":", "_") + ".npz")
    if not cache_file.is_file():
        raise FileNotFoundError(cache_file)
    sample, _ = extract_gate_feature(None, item, cache)  # 复用 v2 已保存的同一输入特征。
    offline = predict_samples(model, [sample], device="cpu")[0]["p_risk"]
    parts = [item[k] for k in ("title", "description", "ocr", "asr")]
    hits = check_text("\n".join(parts))
    injection = has_injection(parts)
    payload = {"gate_mode": "trained", "frames": item["all_frames"], "title": parts[0],
               "description": parts[1], "ocr_text": parts[2], "asr_text": parts[3],
               "audio_embed_b64": item.get("audio_embed_b64", ""), "text_hits": hits,
               "audio_risk_score": item.get("audio_risk_score", 0), "injection_hits": injection,
               "aud_bypass": item.get("aud_bypass", False)}
    online = post_worker("/risk_gate", payload)  # 调用同一检查点的在线门控。
    if online["gate_mode"] != "trained":
        raise ValueError("Worker 未加载训练模型")
    bypass = (any(f["type"] in EVASION_FRAME_TYPES for f in item["all_frames"])
              or bool(payload["aud_bypass"] or injection or hits.get("should_reject")
                      or hits.get("redline_hits") or hits.get("contact_hits")))
    return {"uid": item["uid"], "source": item["source"], "offline": offline,
            "online": online["p_risk"], "error": abs(online["p_risk"] - offline),
            "offline_pass": offline < threshold and not bypass, "online_pass": online["early_exit"]}


def verify_scores():
    """
    输入为 v2 验证清单及 M3 检查点。
    输出为包含逐条误差和通过情况的 P3 文件，访问在线服务。
    功能为固定抽取 50 个视频标识，例如复跑时使用相同样本。
    """
    splits = json.loads((OLD / "work_e0/splits_materialized.json").read_text())
    rows = sorted(splits["calib"], key=lambda r: hashlib.sha256(r["uid"].encode()).digest())[:SAMPLE_COUNT]
    state = torch.load(OLD / "checkpoints/m3_mil_gate.pt", map_location="cpu", weights_only=True)
    model = MILMaxGuardedGate(**state["config"], dropout=0)
    model.load_state_dict(state["state_dict"])  # 加载服务正在使用的权重。
    threshold = json.loads((OLD / "checkpoints/gate_calibration.json").read_text())["trained"]["tau_95"]
    records = [comparison(row, model, threshold) for row in rows]
    passed = len(records) == SAMPLE_COUNT and all(
        r["error"] < TOLERANCE and r["offline_pass"] == r["online_pass"] for r in records)
    (RESULTS / "p3_comparison.json").write_text(json.dumps({"passed": passed, "records": records}, indent=2))


def score_e3():
    """
    输入为 v2 E3 的实际送审清单和全量候选帧。
    输出为每条视频的模型分数、分数是否达到阈值以及原送审决定。
    功能为单独衡量规则变化的影响，例如模型已达阈值的视频仍然送审。
    """
    work = OLD / "work_e3"
    sent = {json.loads(line)["sample_id"] for line in (work / "e3a_9b_review_in.jsonl").read_text().splitlines()}
    scores = []
    for path in sorted((work / "e3a_frames_meta").glob("*.json")):
        frames = json.loads(path.read_text())["frames"]
        gate = post_worker("/risk_gate", {"frames": frames, "title": "日常视频分享", "mode": "trained"})
        if gate["gate_mode"] != "trained":
            raise ValueError("Worker 未加载训练模型")
        scores.append({"uid": path.stem, "sent_v2": path.stem in sent, "score": gate["p_risk"],
                       "above_threshold": gate["p_risk"] >= gate["threshold"], "sent_replay": not gate["early_exit"]})
    (RESULTS / "p2_baseline.json").write_text(json.dumps(scores, indent=2))


if __name__ == "__main__":
    score_e3()
    verify_scores()
