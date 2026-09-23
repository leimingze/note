"""
脚本功能：
验证 Redis 阅读型 Notebook 的章节切分与无练习结构。

启动命令：
cd redis && python3 -m unittest scripts/test_build_notebooks.py
"""

import sys
import unittest
from pathlib import Path


SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))
import build_notebooks as builder


FORBIDDEN_TEXT = (
    "练习数据背景",
    "我的作答",
    "参考实现与验证",
    "具体问题",
    "完成标准",
)


class NotebookBuildTest(unittest.TestCase):
    """Redis Notebook 生成链路的结构测试。"""

    def test_all_notebooks_are_markdown_only(self) -> None:
        """
        输入：六个 Redis 主题的 Markdown 源文件。
        输出：每份生成结果只有两个 Markdown 单元格且无练习相关文字。
        功能：防止重新生成时恢复已删除的题目、答案或代码格。
        """
        for doc_file in builder.NOTEBOOKS:
            notebook = builder.build_notebook(doc_file)
            self.assertEqual(len(notebook.cells), 2)
            self.assertTrue(all(cell.cell_type == "markdown" for cell in notebook.cells))
            self.assertNotIn("kernelspec", notebook.metadata)
            combined = "\n".join(cell.source for cell in notebook.cells)
            for marker in FORBIDDEN_TEXT:
                self.assertNotIn(marker, combined, msg=f"{doc_file}: {marker}")
            self.assertIn("## 八股问答", notebook.cells[0].source)
            self.assertIn("## 面试话术", notebook.cells[1].source)
            self.assertIn("## 自查清单", notebook.cells[1].source)

    def test_each_notebook_contains_outline_topics(self) -> None:
        """
        输入：六个主题的源码内容。
        输出：每份包含该模块的主要问题标题。
        功能：检查用户大纲知识点已覆盖。
        """
        expected = {
            "01-基础": ("Redis 和 MySQL 的区别", "Redis 的应用场景"),
            "02-数据结构": ("Redis 有哪些数据类型", "SDS", "哈希冲突", "跳表"),
            "03-持久化": ("AOF 和 RDB", "混合持久化", "执行快照", "key 过大"),
            "04-功能与淘汰": ("过期删除", "内存淘汰", "LRU", "LFU"),
            "05-高可用": ("主从复制", "哨兵", "Cluster", "客户端", "节点出现故障"),
            "06-缓存": ("缓存雪崩", "缓存穿透", "缓存击穿", "延迟双删"),
        }
        for doc_file, markers in expected.items():
            notebook = builder.build_notebook(doc_file)
            combined = "\n".join(cell.source for cell in notebook.cells)
            for marker in markers:
                self.assertIn(marker, combined, msg=f"{doc_file}: 缺少 {marker}")


if __name__ == "__main__":
    unittest.main()
