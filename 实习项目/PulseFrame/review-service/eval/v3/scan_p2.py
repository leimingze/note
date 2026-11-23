# 脚本功能：穷举事件数量与最大强度的阈值，比较正常视频与 E3 原送审决定。
# 启动命令：python scan_p2.py --results reports/v3
import argparse
import json
from pathlib import Path

# 验收允许 200 条正常视频中至多 20 条因插帧标记送审。
NORMAL_LIMIT = 20
# 直接插帧和画中画的 v2 变体名称。
PRIMARY = {"hard_1", "hard_3", "hard_10", "pip"}


def strength_bound(rows, scores, groups):
    """
    输入为正常视频、指定变体的事件和模型送审结果。
    输出为单调数量与强度规则保全原送审视频时必须送审的正常样本清单。
    功能为检查分类别设阈值是否能满足要求，例如同类事件逐个更强且更多时也会触发。
    """
    kinds = ("brief_insertion", "local_change", "fade_overlay")
    def signature(row):
        """
        输入为一条视频的挑帧事件。
        输出为按类型分别降序排列的强度。
        功能为支持按类型逐事件比较，例如两次局部变化均保留。
        """
        return [sorted((e["score"] for e in row["events"] if e["type"] == kind), reverse=True)
                for kind in kinds]

    required = {r["uid"]: signature(r) for r in rows if r["group"] in groups
                and scores[r["uid"]]["sent_v2"] and not scores[r["uid"]]["above_threshold"]}
    forced = {}
    for row in rows:
        if row["group"] != "normal":
            continue
        normal = signature(row)
        witnesses = [uid for uid, needed in required.items()
                     if all(len(n) >= len(p) and all(a >= b for a, b in zip(n, p))
                            for n, p in zip(normal, needed))]
        if witnesses:
            forced[row["uid"]] = witnesses
    return {"required_positive": len(required), "normal_lower_bound": len(forced), "witnesses": forced}


def scan(rows, scores):
    """
    输入为挑帧事件、同视频的模型送审分数和原送审决定。
    输出为限制正常送审数量时的最佳送审数，以及保全原送审数所需的最少正常送审数。
    功能为枚举数量或强度达标即送审的所有不同决定，例如相同强度的行一同进入。
    """
    counts = [len(row["events"]) for row in rows]
    peaks = [max((e["score"] for e in row["events"]), default=-1) for row in rows]
    normal = {i for i, r in enumerate(rows) if r["group"] == "normal"}
    primary = {i for i, r in enumerate(rows) if r["group"] in PRIMARY}
    extended = set(range(len(rows))) - normal
    model = {i for i in extended if scores[rows[i]["uid"]]["above_threshold"]}
    old = {i for i in extended if scores[rows[i]["uid"]]["sent_v2"]}
    candidates = []
    for count in range(1, max(counts) + 2):
        counted = {i for i, n in enumerate(counts) if n >= count}
        for strength in [*sorted({p for p in peaks if p >= 0}), None]:
            forced = counted | {i for i, p in enumerate(peaks) if strength is not None and p >= strength}
            sent = forced | model
            candidates.append({"count": count, "strength": strength, "normal": len(normal & forced),
                               "primary": len(primary & sent), "all": len(extended & sent)})
    profiles = {}
    for name, group in [("primary", primary), ("all", extended)]:
        old_count = len(group & old)
        within_limit = [c for c in candidates if c["normal"] <= NORMAL_LIMIT]
        retaining = [c for c in candidates if c[name] >= old_count]
        profiles[name] = {"total": len(group), "previously_sent": old_count,
                          "typed_strength_bound": strength_bound(rows, scores, {rows[i]["group"] for i in group}),
                          "best_within_limit": max(within_limit, key=lambda c: (c[name], -c["normal"])),
                          "fewest_normal_at_baseline": min(retaining, key=lambda c: c["normal"]) if retaining else None}
    return {"normal_count": len(normal), "original_normal_forced": sum(counts[i] > 0 for i in normal),
            "candidates": len(candidates), "profiles": profiles}


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--results", type=Path, required=True)
    folder = parser.parse_args().results
    events = [json.loads(line) for line in (folder / "p2_events.jsonl").read_text().splitlines()]
    baseline = {r["uid"]: r for r in json.loads((folder / "p2_baseline.json").read_text())}
    result = scan(events, baseline)
    (folder / "p2_scan.json").write_text(json.dumps(result, indent=2))
