# 当前任务

- 任务编号：20261008-book-access
- 当前方案：保留按章生成的 Markdown，使用 markdown-it-py 解析标题及来源范围，使用 jieba 中文分词与 SQLite FTS5 建立本地检索；agent 按小节编号和字符预算读取正文，不调用大模型或网络服务。
- 输入：`亿级流量系统架构设计与实战 (--) (manongshu.com).pdf`，433 页，332 条书签，13 章。
- 有效文件：转换入口 `pdf_to_md.py` 及 `pdf_content.py`、`pdf_headings.py`；阅读入口 `book_access.py` 及 `book_index.py`、`book_markdown.py`、`book_query.py`；依赖 `requirements.txt`；验证 `test_pdf_to_md.py`、`test_book_access.py`；阅读规则 `AGENTS.md`。
- 有效结果：`亿级流量系统架构设计与实战-md/README.md`、同目录下的 13 个章节文件、书前资料、`assets/`、`conversion-report.json`、`agent-index.md` 和 `sections.sqlite`。
- 输入版本与结果证据：`亿级流量系统架构设计与实战-md/conversion-report.json` 记录来源 SHA-256、433 页的连续章节范围、327 个正文标题和 257 张图片。
- 检索索引证据：`sections.sqlite` 的 `sections` 表记录 314 个小节及 14 个章级或书前资料入口，总计 328 项；`files` 表保存来源 Markdown 与转换报告的 SHA-256 指纹；`metadata` 表保存原 PDF 指纹及索引版本。
- 长度统计：来源 Markdown 共 500359 个字符；移除重复 PDF 链接和独立图片引用后的本节正文合计 407944 个字符，统计使用 `book_access.py index`，字符数不等同于 token 数。
- 生成命令：`python3 pdf_to_md.py '亿级流量系统架构设计与实战 (--) (manongshu.com).pdf' --output '亿级流量系统架构设计与实战-md'`；自动生成阅读索引。更新同一来源的已有结果可增加 `--overwrite`；仅更新索引执行 `python3 book_access.py index`。
- 阅读命令：`python3 book_access.py search 'Snowflake 时钟回拨'`；`python3 book_access.py read 4.3.5 --max-chars 3000`；续读使用返回的 `next_cursor`。默认读取本节，`--scope subtree` 包含下级小节，`--images` 返回图片路径。
- 验证情况：整本转换及自动索引生成成功，原有章节与图片无内容变化；`compileall` 成功；`python3 -m unittest -v test_book_access.py test_pdf_to_md.py` 在 60 秒超时约束下通过全部 12 项验证，覆盖原有转换行为、中英文检索、单字中文检索、阅读范围、续读完整性、输出预算、图片按需查询、索引失效及重新生成、命令行 JSON 输出。
- 已确认限制：原文字层含 OCR 错误，代码符号需要核对；复杂表格保留为等宽文字。第 179、255 页没有文字层，已保留原页图片。证据见转换报告与对应章节文件。
- 未解决问题：无阻碍交付的问题；未进行全文 OCR 纠错。FTS5 使用文字检索，同义表达需调整关键词；续读位置为过滤后正文的字符下标，片段可能处于段落或代码块内部。
- 下一步：无。
- 归档位置：本任务已归档，分词缓存和指纹测试使用的原文副本保留在 `.agent/work/20261008-book-access/`，已加入 Git 忽略规则，不默认读取。
