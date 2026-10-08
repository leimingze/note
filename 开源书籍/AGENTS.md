# 书籍按需阅读

本目录的书籍资料用于按问题查询。入口为 `亿级流量系统架构设计与实战-md/agent-index.md`，包含章节标题和原书关键词。

## 查询命令

在本目录执行：

```bash
python3 book_access.py search 'Snowflake 时钟回拨' --limit 5
python3 book_access.py sections 4
python3 book_access.py read 4.3.5 --max-chars 3000
```

- 按问题搜索，再根据返回的小节编号读取正文。
- `read` 默认只读取本节正文；`--scope subtree` 包含下级小节。
- 输出中的 `next_cursor` 非空表示还有正文，使用 `--cursor` 继续读取；字符预算可按任务需要调整，不等同于 token 数。
- 搜索摘录用于选择阅读位置，回答书中观点时读取相关原文。
- `--images` 仅返回图片路径；需要图表时再加载对应图片。
- 正文输出省去长 PDF 链接，来源文件与 PDF 页码保留在元数据中。
- 索引指纹不符时运行 `python3 book_access.py index` 更新索引。
- 原文字层有 OCR 错误，代码符号和复杂表格需要结合 PDF 核对。

## 有效文件

- 项目任务与验证记录：`.agent/STATE.md`。
- 转换入口：`pdf_to_md.py`。
- 查询入口：`book_access.py`。
- 书籍原文、图片、转换报告及检索数据库：`亿级流量系统架构设计与实战-md/`。
