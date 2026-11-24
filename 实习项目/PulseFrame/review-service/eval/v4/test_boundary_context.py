# 脚本功能：
# 验证切镜边界前帧补采会保留基础事件，并为每个未占用的切镜新增前一帧。
#
# 启动命令：
# PYTHONPATH=/root/autodl-tmp/rs-code-v4/worker \
# /root/miniconda3/bin/python -m unittest eval.v4.test_boundary_context

from __future__ import annotations

import unittest
from unittest.mock import patch

from eval.v4.detect_context import detect_with_boundary_context
from framesel.select_frames import Thresholds


class BoundaryContextTest(unittest.TestCase):
    """验证边界补采的数量和帧号。"""

    @patch("eval.v4.detect_context.detect")
    def test_adds_frame_before_scene_change(self, detect_mock) -> None:
        """输入切镜事件，输出前一帧候选；例如第 115 帧切镜会补采第 114 帧。"""
        detect_mock.return_value = {
            "events": [
                {"index": 115, "score": 0.3, "type": "scene_change"},
                {"index": 50, "score": 0.0, "type": "timed"},
            ],
            "counts": {"scene_change": 1, "timed": 1},
            "evasion_suspected": False,
        }
        result = detect_with_boundary_context({}, Thresholds())
        self.assertIn(114, [event["index"] for event in result["events"]])
        self.assertEqual(result["counts"]["boundary_context"], 1)
        self.assertEqual(result["counts"]["scene_change"], 1)

    @patch("eval.v4.detect_context.detect")
    def test_does_not_duplicate_existing_frame(self, detect_mock) -> None:
        """输入前一帧已经选中，输出不增加重复候选；例如第 99 帧已有定时帧。"""
        detect_mock.return_value = {
            "events": [
                {"index": 100, "score": 0.3, "type": "scene_change"},
                {"index": 99, "score": 0.0, "type": "timed"},
            ],
            "counts": {"scene_change": 1, "timed": 1},
            "evasion_suspected": False,
        }
        result = detect_with_boundary_context({}, Thresholds())
        self.assertEqual(result["counts"]["boundary_context"], 0)
        self.assertEqual(sum(event["index"] == 99 for event in result["events"]), 1)


if __name__ == "__main__":
    unittest.main()
