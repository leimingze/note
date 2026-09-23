"""
脚本功能：
验证 LangGraph 使用方法 notebook 中无需 API Key 的代码 cell 均可执行，
跳过带 requires-api-key 标签的 cell（需要模型 Key，本机未配置）。

启动命令：
.venv/bin/python -m ipykernel install --user --name langgraph-demo
.venv/bin/python scripts/verify_langgraph_notebook.py
"""

from pathlib import Path

import nbformat
from nbclient import NotebookClient


NOTEBOOK_PATH = Path(__file__).resolve().parent.parent / "demo" / "LangGraph使用方法.ipynb"


def filter_executable_cells(notebook) -> list:
    """
    输入：notebook，nbformat 对象。
    输出：不包含 requires-api-key 标签的 cell 列表。
    功能：过滤出无需模型 API Key 即可执行的 cell。
    """
    return [
        cell
        for cell in notebook.cells
        if "requires-api-key" not in cell.metadata.get("tags", [])
    ]


def main() -> None:
    """
    输入：无。
    输出：执行通过时打印 cell 数量；任一 cell 失败时抛出异常。
    功能：在内存副本中执行无需 API Key 的 cell，验证 notebook 可运行。
    """
    notebook = nbformat.read(NOTEBOOK_PATH, as_version=4)
    filtered = nbformat.v4.new_notebook(metadata=notebook.metadata)
    filtered.cells = filter_executable_cells(notebook)
    client = NotebookClient(filtered, kernel_name="langgraph-demo", timeout=120)
    client.execute()
    print(f"executed {len(filtered.cells)} cells, skipped requires-api-key cells")


if __name__ == "__main__":
    main()
