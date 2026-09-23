# Redis 八股学习目录

本目录按“基础 → 数据结构 → 持久化 → 功能与淘汰 → 高可用 → 缓存”组织 Redis 面试知识。
Notebook 仅包含知识讲解、面试表达和自查清单，不包含练习题或代码。

## 学习顺序

| 顺序 | 模块 | 对应大纲问题 |
| --- | --- | --- |
| 1 | [基础](notebooks/01-基础.ipynb) | Redis 与 MySQL 区别、为什么快、应用场景 |
| 2 | [数据结构](notebooks/02-数据结构.ipynb) | 数据类型、底层结构、SDS、哈希冲突、跳表 |
| 3 | [持久化](notebooks/03-持久化.ipynb) | RDB、AOF、混合持久化、快照、大 Key |
| 4 | [功能与淘汰](notebooks/04-功能与淘汰.ipynb) | 过期删除、内存淘汰、LRU、LFU |
| 5 | [高可用](notebooks/05-高可用.ipynb) | 主从、哨兵、Cluster、故障与一致性 |
| 6 | [缓存](notebooks/06-缓存.ipynb) | 雪崩、穿透、击穿、缓存一致性、延迟双删 |

## Notebook 结构

每份 Notebook 固定包含两个 Markdown 单元格：

- 八股问答：解释概念、原理、场景与取舍。
- 面试话术：按“结论 → 原理 → 场景 → 取舍”组织回答。
- 自查清单：用于阅读后的知识点检查。

这些 Notebook 是纯阅读材料，无需 Redis 服务、Jupyter 内核或额外环境。直接用 VS Code、
JupyterLab 或其他支持 `.ipynb` 的工具打开即可。

## 内容维护

Markdown 源文件位于 `sources/`，由 `scripts/build_notebooks.py` 统一生成
`notebooks/` 下的六份阅读型 Notebook。

```bash
cd redis
python3 scripts/build_notebooks.py
```
