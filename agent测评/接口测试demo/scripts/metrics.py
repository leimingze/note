"""
脚本功能：
读取测试结果 JSON，计算覆盖、执行、缺陷与性能指标，并输出发布门禁结论。

启动命令：
python scripts/metrics.py scripts/test_results.example.json
"""

import json
import sys
from collections import Counter


# ==================== 常量与配置 ====================

VALID_RESULTS = frozenset({"passed", "failed", "blocked", "not_run"})
BLOCKING_SEVERITIES = frozenset({"blocker", "critical"})
PERF_GATE = {"target_rps": 200, "p95_limit_ms": 500, "error_rate_limit": 0.001}
RELIABILITY_TARGETS = frozenset({"G2", "G3", "G5"})


# ==================== 输入输出、加载与保存 ====================


def load_results(path: str) -> dict:
    """
    输入：path，测试结果 JSON 文件路径。
    输出：解析后的结果字典；文件不存在或格式错误时抛出异常。
    功能：读取用例、缺陷、性能与可运营性数据，作为指标计算输入。
    """
    with open(path, encoding="utf-8") as f:
        return json.load(f)


# ==================== 核心逻辑 ====================


def compute_execution(cases: list[dict]) -> dict:
    """
    输入：cases，用例字典列表，每项含 result 与 core 字段。
    输出：执行率、通过率、阻塞率及核心用例分层的指标字典。
    功能：按指南口径计算执行指标，核心与非核心分开统计。
    """
    counts = Counter(case["result"] for case in cases)
    planned = len(cases)
    executed = counts["passed"] + counts["failed"]
    core = [case for case in cases if case.get("core")]
    core_counts = Counter(case["result"] for case in core)
    core_executed = core_counts["passed"] + core_counts["failed"]
    return {
        "planned": planned,
        "executed": executed,
        "passed": counts["passed"],
        "failed": counts["failed"],
        "blocked": counts["blocked"],
        "not_run": counts["not_run"],
        "execution_rate": executed / planned if planned else 0,
        "pass_rate": counts["passed"] / executed if executed else 0,
        "blocked_rate": counts["blocked"] / planned if planned else 0,
        "core_planned": len(core),
        "core_executed": core_executed,
        "core_failed": core_counts["failed"],
        "core_pass_rate": core_counts["passed"] / core_executed if core_executed else 0,
    }


def compute_coverage(cases: list[dict], targets: list[str], has_performance: bool) -> dict:
    """
    输入：cases，用例列表；targets，计划测试目标列表；has_performance，是否有性能证据。
    输出：已覆盖目标集合与覆盖率。
    功能：已执行用例的目标计入覆盖，G7 非功能目标由性能证据承接。
    """
    covered = {
        case["target"]
        for case in cases
        if case["result"] in ("passed", "failed") and case.get("target")
    }
    if has_performance and "G7" in targets:
        covered.add("G7")
    target_set = set(targets)
    return {
        "covered": sorted(covered & target_set),
        "uncovered": sorted(target_set - covered),
        "coverage_rate": len(covered & target_set) / len(target_set) if target_set else 0,
    }


def compute_defects(defects: list[dict]) -> dict:
    """
    输入：defects，缺陷字典列表，每项含 severity 与 status 字段。
    输出：严重程度分布与未关闭阻断级缺陷数量。
    功能：按严重程度统计缺陷，识别是否还有阻断发布的未关闭缺陷。
    """
    severity_counts = Counter(defect["severity"] for defect in defects)
    open_blocking = sum(
        1
        for defect in defects
        if defect["severity"] in BLOCKING_SEVERITIES and defect["status"] == "open"
    )
    return {"severity_counts": dict(severity_counts), "open_blocking": open_blocking}


def check_gates(metrics: dict) -> list[dict]:
    """
    输入：metrics，包含执行、覆盖、缺陷、性能与可运营性指标的字典。
    输出：六项发布门禁的通过与不通过列表及原因。
    功能：按指南的门禁维度生成发布判断依据。
    """
    execution = metrics["execution"]
    coverage = metrics["coverage"]
    defects = metrics["defects"]
    performance = metrics["performance"]
    reliability_cases = [
        case
        for case in metrics["cases"]
        if case.get("target") in RELIABILITY_TARGETS
    ]
    reliability_passed = all(
        case["result"] == "passed" for case in reliability_cases
    )
    perf_ok = (
        performance is not None
        and performance["rps"] >= PERF_GATE["target_rps"]
        and performance["p95_ms"] <= PERF_GATE["p95_limit_ms"]
        and performance["error_rate"] < PERF_GATE["error_rate_limit"]
    )
    core_complete = (
        execution["core_executed"] == execution["core_planned"]
        and execution["core_failed"] == 0
        and execution["blocked"] == 0
    )
    return [
        {"name": "覆盖", "passed": coverage["coverage_rate"] == 1.0,
         "reason": "G1-G7 均有证据" if coverage["coverage_rate"] == 1.0
         else f"未覆盖: {', '.join(coverage['uncovered'])}"},
        {"name": "执行", "passed": core_complete,
         "reason": "核心用例全部执行并通过" if core_complete else "核心用例未全部执行或存在失败/阻塞"},
        {"name": "缺陷", "passed": defects["open_blocking"] == 0,
         "reason": "无未关闭阻断/严重缺陷" if defects["open_blocking"] == 0
         else f"存在 {defects['open_blocking']} 个未关闭阻断/严重缺陷"},
        {"name": "性能", "passed": perf_ok,
         "reason": "A2 达标" if perf_ok else "200 RPS 下 P95 或错误率未达标/未提供性能证据"},
        {"name": "可靠性", "passed": reliability_passed,
         "reason": "事务、MQ 与跨系统用例全部通过" if reliability_passed else "存在可靠性用例失败"},
        {"name": "可运营性", "passed": bool(metrics.get("operations_ready")),
         "reason": "监控、灰度与回滚条件已准备" if metrics.get("operations_ready")
         else "监控/灰度/回滚条件未确认"},
    ]


def render_report(metrics: dict, gates: list[dict]) -> str:
    """
    输入：metrics，全部指标字典；gates，门禁结果列表。
    输出：可直接粘贴到测试报告的多行文本。
    功能：把指标与门禁格式化为可复核的报告正文。
    """
    execution = metrics["execution"]
    coverage = metrics["coverage"]
    defects = metrics["defects"]
    lines = [
        f"版本: {metrics.get('version', '未知')}  环境: {metrics.get('environment', '未知')}",
        f"执行率: {execution['execution_rate']:.1%} ({execution['executed']}/{execution['planned']})",
        f"通过率: {execution['pass_rate']:.1%} ({execution['passed']}/{execution['executed']})",
        f"阻塞率: {execution['blocked_rate']:.1%} ({execution['blocked']}/{execution['planned']})",
        f"需求覆盖率: {coverage['coverage_rate']:.1%} ({len(coverage['covered'])}/{len(coverage['covered']) + len(coverage['uncovered'])})",
        f"严重程度分布: {defects['severity_counts'] or '无缺陷'}",
        "",
        "发布门禁:",
    ]
    for gate in gates:
        mark = "通过" if gate["passed"] else "不通过"
        lines.append(f"  [{mark}] {gate['name']}: {gate['reason']}")
    lines.append("")
    lines.append("发布建议: " + ("通过" if all(g["passed"] for g in gates) else "不通过或带风险通过"))
    return "\n".join(lines)


# ==================== 命令行入口 ====================


def main(argv: list[str]) -> int:
    """
    输入：argv，命令行参数，第一个为测试结果 JSON 路径。
    输出：打印指标报告；参数缺失或数据非法时返回非零状态码。
    功能：加载结果、计算指标、检查门禁并输出报告。
    """
    if len(argv) != 1:
        print("用法: python scripts/metrics.py <results.json>")
        return 1
    data = load_results(argv[0])
    for case in data["cases"]:
        if case["result"] not in VALID_RESULTS:
            print(f"非法用例状态: {case.get('id')} -> {case['result']}")
            return 1
    performance = data.get("performance")
    metrics = {
        "version": data.get("version"),
        "environment": data.get("environment"),
        "cases": data["cases"],
        "execution": compute_execution(data["cases"]),
        "coverage": compute_coverage(data["cases"], data.get("targets", []), performance is not None),
        "defects": compute_defects(data.get("defects", [])),
        "performance": performance,
        "operations_ready": data.get("operations_ready", False),
    }
    print(render_report(metrics, check_gates(metrics)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
