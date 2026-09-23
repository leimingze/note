# MySQL 八股学习目录

本目录按“基础 → 索引 → 事务 → 锁 → MVCC → 日志 → 内存 → 存储引擎 → 性能优化
→ SQL 优化”组织 MySQL 面试知识。Notebook 仅包含知识讲解、面试表达和自查清单，
不包含练习题或代码。

## 学习顺序

| 顺序 | 模块 | 对应大纲问题 |
| --- | --- | --- |
| 1 | [基础：SQL 与存储](notebooks/01-基础.ipynb) | 增删改查、SELECT 执行流程、一行记录如何存储、UPDATE 会发生什么 |
| 2 | [索引](notebooks/02-索引.ipynb) | B+ 树、聚簇/二级索引、回表、覆盖索引、最左前缀、索引失效、索引设计 |
| 3 | [事务](notebooks/03-事务.ipynb) | ACID、隔离级别、脏读、不可重复读、幻读、RR 下 UPDATE |
| 4 | [锁](notebooks/04-锁.ipynb) | 全局锁、表锁、行锁、MDL、间隙锁、死锁 |
| 5 | [MVCC](notebooks/05-MVCC.ipynb) | Read View、可见性规则、快照读、当前读、回滚流程、RC/RR 区别 |
| 6 | [日志与主从复制](notebooks/06-日志与主从复制.ipynb) | redo、undo、binlog、两阶段提交、崩溃恢复、主从复制 |
| 7 | [Buffer Pool 与内存](notebooks/07-BufferPool与内存.ipynb) | Buffer Pool、LRU、刷盘、Change Buffer |
| 8 | [存储引擎](notebooks/08-存储引擎.ipynb) | InnoDB/MyISAM 对比、InnoDB 底层架构、引擎选择 |
| 9 | [性能优化与 EXPLAIN](notebooks/09-性能优化与EXPLAIN.ipynb) | 慢查询、EXPLAIN、高效分页、COUNT 优化、I/O 优化 |
| 10 | [SQL 优化](notebooks/10-SQL优化.ipynb) | 插入数据、主键、ORDER BY、GROUP BY、LIMIT、UPDATE |

建议按顺序阅读：先理解 SQL 与存储，再学习索引、事务、锁和 MVCC；之后用日志串起
写入与恢复链路，最后通过内存、存储引擎和 EXPLAIN 建立性能分析框架。

## Notebook 结构

每份 Notebook 固定包含：

- 八股问答：解释概念、原理、场景与取舍。
- 面试话术：按“结论 → 原理 → 场景 → 取舍”组织回答。
- 自查清单：用于阅读后的知识点检查。

这些 Notebook 是纯阅读材料，无需数据库、Jupyter 内核或额外环境。直接用 VS Code、
JupyterLab 或其他支持 `.ipynb` 的工具打开即可。

## 内容维护

Notebook 由 `scripts/build_notebooks.py` 统一生成。当前同名 Markdown 源不存在时，
生成器会从现有 Notebook 的首尾 Markdown 单元格恢复讲解内容。
