# 脚本功能：
# 验证 v4 在满足门禁后优先选择硬插帧、画中画和渐变三类最低召回最高的候选。
#
# 启动命令：
# PYTHONPATH=/root/autodl-tmp/rs-code-v4/worker \
# /root/miniconda3/bin/python -m unittest eval.v4.test_selection_metric

from __future__ import annotations

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from run_exp0_framesel import select_best


def candidate(name: str, balanced: float, pip_fade: float, f1: float, fpr: float = 0.1) -> dict:
    """
    输入：name，候选名称；balanced，三类最低召回；pip_fade，画中画与渐变平均召回；f1，总体 F1；fpr，正常误触率。
    输出：包含选优所需最小指标集合的候选字典。
    功能：为选优排序提供可控输入，例如高总体 F1 但低最低召回的候选不应胜出。
    """
    return {
        "name": name,
        "tune": {
            "circuit_breaker_fpr": 0.0,
            "hard_insertion_recall": 1.0,
            "benign_fpr": fpr,
            "balanced_min_recall": balanced,
            "pip_fade_recall": pip_fade,
            "overall_f1": f1,
        },
    }


class SelectionMetricTest(unittest.TestCase):
    """验证均衡召回优先且原门禁继续生效。"""

    def test_balanced_recall_precedes_overall_f1(self) -> None:
        """输入两个合格候选，输出最低召回更高的候选；例如 F1 更低但三类更均衡者胜出。"""
        high_f1 = candidate("high_f1", balanced=0.2, pip_fade=0.3, f1=0.8)
        balanced = candidate("balanced", balanced=0.4, pip_fade=0.5, f1=0.6)
        self.assertEqual(select_best([high_f1, balanced])["name"], "balanced")

    def test_guardrails_exclude_candidate(self) -> None:
        """输入正常误触超过 20% 的候选，输出合格候选；例如 25% 误触不能靠高召回入选。"""
        invalid = candidate("invalid", balanced=0.9, pip_fade=0.9, f1=0.9, fpr=0.25)
        valid = candidate("valid", balanced=0.2, pip_fade=0.3, f1=0.5)
        self.assertEqual(select_best([invalid, valid])["name"], "valid")


if __name__ == "__main__":
    unittest.main()
