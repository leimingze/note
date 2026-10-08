# 当前任务

- 任务编号：20261008-pdf-markdown
- 当前方案：使用 PyMuPDF 1.26.7 的文字层、目录书签和图片接口，在本地按章生成 Markdown，不调用大模型或网络服务。
- 输入：`亿级流量系统架构设计与实战 (--) (manongshu.com).pdf`，433 页，332 条书签，13 章。
- 有效文件：`pdf_to_md.py`、`pdf_content.py`、`pdf_headings.py`、`requirements.txt`、`test_pdf_to_md.py`。
- 有效结果：`亿级流量系统架构设计与实战-md/README.md`、同目录下的 13 个章节文件、书前资料、`assets/` 和 `conversion-report.json`。
- 输入版本与结果证据：`亿级流量系统架构设计与实战-md/conversion-report.json` 记录来源 SHA-256、433 页的连续章节范围、327 个正文标题和 257 张图片。
- 生成命令：`python3 pdf_to_md.py '亿级流量系统架构设计与实战 (--) (manongshu.com).pdf' --output '亿级流量系统架构设计与实战-md'`；更新同一来源的已有结果可增加 `--overwrite`。
- 验证情况：整本转换成功；`compileall` 成功；`python3 -m unittest -v test_pdf_to_md.py` 在 60 秒超时约束下通过全部 4 项验证，覆盖页码、书签标题、图片可读取性、原 PDF 链接、来源指纹、列表续行和已有输出保护。
- 已确认限制：原文字层含 OCR 错误，代码符号需要核对；复杂表格保留为等宽文字。第 179、255 页没有文字层，已保留原页图片。证据见转换报告与对应章节文件。
- 未解决问题：无阻碍交付的问题；未进行全文 OCR 纠错。
- 下一步：无。
- 归档位置：本任务已完成，未产生独立过程文件。
