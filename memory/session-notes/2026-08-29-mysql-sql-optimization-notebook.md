# 会话记录：MySQL SQL 优化 Notebook 补充

日期：2026-08-29

## 背景

用户希望在 `mysql/notebooks/` 补充 SQL 优化知识点，并明确第 1 节“优化前先看什么”参考
`09-性能优化与EXPLAIN.ipynb` 的排查工具与顺序。

## 结果

- 新增 Markdown 源：`mysql/10-SQL优化.md`。
- 生成阅读型 Notebook：`mysql/notebooks/10-SQL优化.ipynb`。
- 内容覆盖 7 个主题：插入数据（批量 INSERT、ON DUPLICATE、LOAD DATA）、主键设计、
  ORDER BY、GROUP BY、LIMIT、COUNT、UPDATE；主键设计部分补充了 InnoDB 页分裂与页合并
  说明。
- `mysql/README.md` 已增加第 10 项并更新学习顺序。
- `mysql/scripts/build_notebooks.py` 已加入 `10-SQL优化`，并修复 Markdown 源没有
  `## 实操` 时讲解与面试/自查重叠的问题。
- `mysql/scripts/test_build_notebooks.py` 新增 SQL 优化结构与二级索引叶子节点回归测试，7 项测试通过。
- 修正 `01-基础.ipynb` 第一范式解释：1NF 要求字段原子，单手机号不需要拆表，只有一实体
  有多个同类值时才拆子表；同时更新 `build_notebooks.py` 中的固定范式章节和回归断言。
- 在三大范式后补充“不满足范式会有什么问题”：1NF 带来查询/更新/唯一约束问题，2NF 造成
  重复事实与更新、插入异常，3NF 造成传递依赖与更新、插入、删除异常。
- 2NF 示例从订单明细改为学生/课程/选课表：`course_selections` 以
  `(student_id, course_id)` 为联合主键，`student_name` 与 `course_name` 分别只依赖主键
  的一部分，说明部分依赖更直观；并在例子后直接说明不改会造成姓名/课程名重复、批量改名、
  新实体无关联记录时难以插入等问题。
- `10-SQL优化.md` ORDER BY 小节把 `Using filesort` 解释提前到示例前，并压缩为两句话：
  Extra 提示、需要额外排序、可能用 sort buffer 或临时文件，是否优化看扫描行数和耗时。
- ORDER BY 示例改为联合索引 `(user_id, created_at)` 的三种排序：同向、整体反向、
  混向通常 filesort。
- `02-索引.ipynb` 最左前缀原则补充：不满足最左前缀时优化器仍可能选择索引，但通常只能
  走 `type=index` 的全索引扫描或覆盖扫描，而不是高效的 `ref`/`range`。
- `10-SQL优化.md` GROUP BY 示例改为 `(province, age, status)` 联合索引，并说明
  `GROUP BY status` 因相同值不连续而产生 `Using temporary`。
- LIMIT 延迟关联补充说明：主要收益是跳过数据时只读取较小的索引主键，最后只对目标
  20 行回表，不是单纯优化回表。
- 用户要求删除 COUNT 优化节，`UPDATE` 重新编号为第 6 节，README、测试和记忆同步更新。

## 验证

- 重新生成后 `10-SQL优化.ipynb` 只有 2 个 Markdown 单元格，无 kernelspec、代码格或练习内容。
- 第 1 格只包含标题与八股问答；第 2 格只包含面试话术和自查清单。
- 生成器测试命令：`cd mysql && .venv/bin/python -m unittest scripts/test_build_notebooks.py -v`。
