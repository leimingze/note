"""
脚本功能：
验证 MySQL 阅读型 Notebook 的章节切分、无练习结构与图文内容。

启动命令：
cd mysql && .venv/bin/python -m unittest scripts/test_build_notebooks.py
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
    """阅读型 MySQL Notebook 生成链路的结构测试。"""

    def test_heading_matches_suffix(self) -> None:
        """
        输入：精确标题、带后缀标题和无关标题。
        输出：前两者匹配，无关标题不匹配。
        功能：保证现有带说明的八股问答标题仍能被恢复。
        """
        self.assertTrue(builder.heading_matches("## 八股问答", "## 八股问答"))
        self.assertTrue(
            builder.heading_matches("## 八股问答（阅读后复习）", "## 八股问答")
        )
        self.assertFalse(builder.heading_matches("## 面试话术", "## 八股问答"))

    def test_read_section_stops_at_end_heading(self) -> None:
        """
        输入：包含起止二级标题的 Markdown 文本。
        输出：只返回起始章节，不包含结束章节。
        功能：避免不同复习章节被错误合并。
        """
        source = "## 八股问答\n内容\n## 实操：示例\n不应包含"
        section = builder.read_section_text(source, "## 八股问答", "## 实操")
        self.assertEqual(section, "## 八股问答\n内容\n")

    def test_all_notebooks_are_markdown_only(self) -> None:
        """
        输入：十个主题的现有讲解内容。
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

    def test_sql_optimization_notebook_has_expected_sections(self) -> None:
        """
        输入：10-SQL优化 的 Markdown 源内容。
        输出：生成结果包含插入、主键、排序、分组、分页和更新优化。
        功能：锁定 SQL 优化篇的 6 个主题，避免后续重建时内容缺失。
        """
        notebook = builder.build_notebook("10-SQL优化")
        source = notebook.cells[0].source
        self.assertEqual(len(notebook.cells), 2)
        self.assertIn("### 1. 插入数据怎么优化？", source)
        self.assertIn("### 2. 主键怎么优化？", source)
        self.assertIn("### 3. ORDER BY 怎么优化？", source)
        self.assertIn("#### Using filesort 是什么？", source)
        self.assertIn("ORDER BY user_id ASC, created_at ASC", source)
        self.assertIn("ORDER BY user_id DESC, created_at DESC", source)
        self.assertIn("ORDER BY user_id ASC, created_at DESC", source)
        self.assertIn("反向扫描索引", source)
        self.assertIn("### 4. GROUP BY 怎么优化？", source)
        self.assertIn("idx_demo", source)
        self.assertIn("province, age, status", source)
        self.assertIn("记录不会连续聚在一起", source)
        self.assertIn("### 5. LIMIT 怎么优化？", source)
        self.assertIn("前面 50000 行只从较小的索引结构中读取主键", source)
        self.assertNotIn("### 6. COUNT 怎么优化？", source)
        self.assertIn("### 6. UPDATE 怎么优化？", source)
        self.assertIn("LOAD DATA", source)
        self.assertIn("ON DUPLICATE KEY UPDATE", source)
        self.assertIn("64 位", source)
        self.assertIn("41 位", source)
        self.assertIn("12 位", source)
        self.assertIn("EXPLAIN ANALYZE", source)
        self.assertIn("Using filesort", source)
        self.assertIn("sort_buffer", source)
        self.assertIn("需要创建临时表", source)
        self.assertIn("Using temporary", source)
        self.assertIn("延迟关联", source)
        self.assertIn("游标分页", source)
        self.assertNotIn("## 面试话术", source)
        self.assertIn("## 面试话术", notebook.cells[1].source)
        self.assertIn("## 自查清单", notebook.cells[1].source)
        self.assertEqual(notebook.cells[1].source.count("## 面试话术"), 1)
        self.assertEqual(notebook.cells[1].source.count("## 自查清单"), 1)

    def test_secondary_index_leaf_note_is_present_once(self) -> None:
        """
        输入：02-索引 的现有讲解内容。
        输出：二级索引叶子节点说明只出现一次，并包含聚簇键与无主键场景。
        功能：防止重新生成 Notebook 时丢失二级索引叶子节点知识点。
        """
        notebook = builder.build_notebook("02-索引")
        source = notebook.cells[0].source
        marker = "#### 二级索引叶子节点一定包含主键吗？"
        self.assertEqual(source.count(marker), 1)
        self.assertIn("user_id, created_at, id", source)
        self.assertIn("聚簇索引键", source)
        self.assertIn("NOT NULL UNIQUE", source)
        self.assertIn("隐藏行 ID", source)
        self.assertIn("不满足最左前缀时", source)
        self.assertIn("type=index", source)

    def test_global_lock_explanation_and_image_are_present(self) -> None:
        """
        输入：04-锁 的现有全局锁讲解。
        输出：生成结果保留 FTWRL 阻塞范围、备份一致性解释和配图引用。
        功能：防止重建 Notebook 时丢失用户指定的全局锁图文内容。
        """
        notebook = builder.build_notebook("04-锁")
        source = notebook.cells[0].source
        self.assertIn("`SELECT` 仍可以执行", source)
        self.assertIn("已更新数据事务的\n`COMMIT` 都会被阻塞", source)
        self.assertIn("多张表不属于同一时点", source)
        self.assertIn("assets/global-lock-backup-consistency.png", source)

    def test_table_level_lock_hierarchy_is_present(self) -> None:
        """
        输入：04-锁 的现有表级锁讲解。
        输出：生成结果包含四类表级锁，并区分表共享读锁与表独占写锁。
        功能：锁定表级锁的分类层级和两种表锁模式。
        """
        notebook = builder.build_notebook("04-锁")
        source = notebook.cells[0].source
        self.assertIn("表锁、元数据锁（MDL）、意向锁和自增锁", source)
        self.assertIn("表共享读锁", source)
        self.assertIn("`LOCK TABLES table_name READ`", source)
        self.assertIn("表独占写锁", source)
        self.assertIn("`LOCK TABLES table_name WRITE`", source)

    def test_basic_crud_section_only_contains_query_flow_and_lc(self) -> None:
        """
        输入：基础篇现有八股问答。
        输出：第 1 节只保留查询执行顺序与 LeetCode 题单，不含旧 CRUD 说明。
        功能：锁定用户指定的基础篇首节范围，防止旧内容回流。
        """
        notebook = builder.build_notebook("01-基础")
        first_section = builder.read_section_text(
            notebook.cells[0].source,
            "### 1. 增删改查",
            "### 2. 数据库三大设计范式",
        )
        self.assertIn("FROM → JOIN / ON → WHERE", first_section)
        self.assertIn("LeetCode 题目补充", first_section)
        self.assertIn("通过 LeetCode SQL 题补充练习", first_section)
        self.assertNotIn("leetcode.cn/problems/", first_section)
        self.assertNotIn("基本语法", first_section)
        self.assertNotIn("常见坑", first_section)
        self.assertNotIn("INSERT INTO", first_section)
        self.assertNotIn("面试常问点", first_section)

    def test_basic_normal_forms_section_is_present_once(self) -> None:
        """
        输入：基础篇现有八股问答。
        输出：范式章节出现一次，包含 1NF、2NF、3NF，后续章节编号顺延。
        功能：验证数据库设计内容完整且重复生成不会产生重复章节。
        """
        notebook = builder.build_notebook("01-基础")
        source = notebook.cells[0].source
        self.assertEqual(source.count("### 2. 数据库三大设计范式"), 1)
        self.assertIn("第一范式（1NF）", source)
        self.assertIn("不能把一个字段当成列表或逗号分隔串使用", source)
        self.assertIn("直接保存为单个字段即可，不需要拆表", source)
        self.assertIn("一个实体有多个同类值", source)
        self.assertIn("#### 不满足范式会有什么问题？", source)
        self.assertIn("违反 1NF", source)
        self.assertIn("违反 2NF", source)
        self.assertIn("违反 3NF", source)
        self.assertIn("第二范式（2NF）", source)
        self.assertIn("course_selections", source)
        self.assertIn("student_name", source)
        self.assertIn("course_name", source)
        self.assertIn("同一个学生选了 50 门课", source)
        self.assertIn("第三范式（3NF）", source)
        self.assertIn("### 3. SELECT 语句的执行流程？", source)
        self.assertIn("### 4. 一行记录在 InnoDB 里是如何存储的？", source)
        self.assertIn("### 5. 执行 UPDATE 会发生什么？", source)


if __name__ == "__main__":
    unittest.main()
