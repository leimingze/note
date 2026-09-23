"""
脚本功能：
生成十份只包含 MySQL 八股讲解、面试话术和自查清单的阅读型 Notebook。
Markdown 源文件缺失时，从现有 Notebook 恢复讲解内容。

启动命令：
python3 scripts/build_notebooks.py
"""

from pathlib import Path
import re

import nbformat as nbf

ROOT = Path(__file__).resolve().parent.parent
NOTEBOOK_DIR = ROOT / "notebooks"

NOTEBOOKS = [
    "01-基础",
    "02-索引",
    "03-事务",
    "04-锁",
    "05-MVCC",
    "06-日志与主从复制",
    "07-BufferPool与内存",
    "08-存储引擎",
    "09-性能优化与EXPLAIN",
    "10-SQL优化",
]

BASIC_CRUD_SECTION = """\
### 1. 增删改查

查询关键字的逻辑执行顺序：

```text
FROM → JOIN / ON → WHERE → GROUP BY → HAVING → SELECT → DISTINCT → ORDER BY → LIMIT
```

#### LeetCode 题目补充

通过 LeetCode SQL 题补充练习。
"""

BASIC_NORMAL_FORMS_SECTION = """\
### 2. 数据库三大设计范式

#### 第一范式（1NF）：字段不可再分

1NF 要求一个字段只保存一个原子的值，不能把一个字段当成列表或逗号分隔串使用。

反例：`users.phone` 同时保存 `138xxx,139xxx`。这个字段里实际有两个电话号码，
查询其中一个、更新其中一个、对号码做唯一约束都要先拆字符串，容易出错。

如果一位用户最多只有一个手机号，直接保存为单个字段即可，不需要拆表：

```text
users(id, phone)
```

只有当业务确实允许一位用户拥有多个电话号码时，才拆成独立表：

```text
users(id)
user_phones(user_id, phone)
```

拆表不是因为 1NF 要求“一定拆表”，而是因为“一个实体有多个同类值”时，把多个值拆成
多行，才能在每行只放一个号码，方便查询、更新和唯一约束。如果字段只是本身复杂但仍是
一个整体值，应按实际语义判断是否拆，而不是看到“多个值”就机械拆表。

#### 第二范式（2NF）：消除部分依赖

在满足 1NF 的基础上，非主属性必须完全依赖整个候选键，不能只依赖联合主键的一部分。

例如，选课表以 `(student_id, course_id)` 为联合主键，表中同时保存了
`student_name` 和 `course_name`。`student_name` 只依赖 `student_id`，
`course_name` 只依赖 `course_id`，它们都没有完全依赖整个联合主键：

```text
course_selections(student_id, course_id, student_name, course_name, score)
```

应拆成三张表：

```text
students(student_id, student_name)
courses(course_id, course_name)
course_selections(student_id, course_id, score)
```

如果不去掉这两个冗余字段，同一个学生选了 50 门课，`student_name` 就会在 50 行里
重复 50 次。学生改名必须更新所有选课行，容易漏改；新学生还没有选课时，也无法只插入
学生信息而不先造一条选课记录。`course_name` 也有同样问题，只是依赖的是 `course_id`。

#### 第三范式（3NF）：消除传递依赖

在满足 2NF 的基础上，非主属性不能依赖其他非主属性，只能直接依赖候选键。

例如，`users(id, department_id, department_name)` 中，部门名称依赖部门 ID，
应拆成：

```text
users(id, department_id)
departments(department_id, department_name)
```

#### 不满足范式会有什么问题？

- **违反 1NF**：一个字段塞了多个值，查询单一值、更新单一值和唯一约束都很麻烦，
  容易靠字符串拆解处理，还会出现格式不一致和脏数据。
- **违反 2NF**：`student_name` 只依赖 `student_id`、`course_name` 只依赖
  `course_id`，却在每个选课行重复保存；改名要改大量选课行，新学生或新课程在尚无关联
  记录时也难以独立插入。
- **违反 3NF**：`department_name` 只依赖 `department_id` 却重复出现在每个员工行里，
  部门改名要更新很多行；新部门没有员工时，也需要靠占位数据才能插入。

这些问题的共同点是同一个事实被重复保存，导致插入、更新、删除时可能出现数据不一致。
反范式可以为了查询性能保留冗余，但必须明确冗余数据的来源和维护方式。

范式用于减少数据冗余以及插入、更新、删除异常。实际系统可为查询性能适度反范式，
但必须明确冗余字段的数据来源和一致性维护方式。
"""

SECONDARY_INDEX_LEAF_NOTE = """\
#### 二级索引叶子节点一定包含主键吗？

在 InnoDB 中，二级索引的叶子节点一定带有用于回聚簇索引的键。表有显式主键时，
这个键就是主键列；如果是联合索引，叶子节点保存的是“联合索引字段 + 主键字段”。

例如 `idx_user_created(user_id, created_at)` 的叶子节点保存：

```text
user_id, created_at, id
```

所以：

- 查询只需要 `user_id`、`created_at`、`id` 时，可以走覆盖索引，不需要回表；
- 查询还需要其他列时，先用二级索引拿到 `id`，再回聚簇索引取完整行。

如果没有主键，InnoDB 可能选用第一个 `NOT NULL UNIQUE` 索引作为聚簇索引；
如果连合适的唯一键都没有，则使用隐藏行 ID。因此更准确的说法是：二级索引叶子节点
保存的是“聚簇索引键”，通常就是主键。
"""


def heading_matches(line: str, heading: str) -> bool:
    """
    输入：line，单行 Markdown；heading，不含后缀说明的二级标题。
    输出：标题精确匹配或带冒号、括号后缀时返回 True。
    功能：兼容现有 Notebook 中带说明文字的章节标题。
    """
    normalized = line.strip()
    return normalized == heading or normalized.startswith(
        (f"{heading}：", f"{heading}:", f"{heading}（")
    )


def read_section_text(text: str, start: str, end: str | None) -> str:
    """
    输入：text，Markdown 全文；start，起始标题；end，可选结束标题。
    输出：包含起始标题、不包含结束标题的 Markdown 片段。
    功能：从原文中提取八股问答、面试话术或自查清单。
    """
    lines = text.splitlines()
    start_idx = next(i for i, line in enumerate(lines) if heading_matches(line, start))
    end_idx = len(lines)
    if end:
        end_idx = next(
            (i for i, line in enumerate(lines) if heading_matches(line, end)),
            len(lines),
        )
    return "\n".join(lines[start_idx:end_idx]).strip() + "\n"


def replace_basic_crud_section(intro: str) -> str:
    """
    输入：intro，基础篇完整的八股问答 Markdown。
    输出：第 1 节替换为查询执行顺序和 LeetCode 题单后的 Markdown。
    功能：固定基础篇首节内容，避免旧 CRUD 语法说明被重新生成。
    """
    pattern = r"### 1\..*?(?=\n### 2\.)"
    replaced, count = re.subn(pattern, BASIC_CRUD_SECTION.rstrip(), intro, flags=re.DOTALL)
    if count != 1:
        raise ValueError(f"基础篇第 1 节数量异常：{count}")
    return replaced


def add_basic_normal_forms_section(intro: str) -> str:
    """
    输入：intro，已经更新首节的基础篇八股问答 Markdown。
    输出：插入三大设计范式并重新编号后续章节的 Markdown。
    功能：幂等维护基础篇范式章节，避免重复生成或章节编号冲突。
    """
    normal_forms_pattern = (
        r"\n### \d+\. 数据库三大设计范式.*?(?=\n### \d+\.)"
    )
    cleaned = re.sub(normal_forms_pattern, "", intro, flags=re.DOTALL)
    heading_numbers = {
        "SELECT 语句的执行流程？": 3,
        "一行记录在 InnoDB 里是如何存储的？": 4,
        "执行 UPDATE 会发生什么？": 5,
    }
    for heading, number in heading_numbers.items():
        cleaned = re.sub(rf"### \d+\. {re.escape(heading)}", f"### {number}. {heading}", cleaned)
    select_heading = "### 3. SELECT 语句的执行流程？"
    if cleaned.count(select_heading) != 1:
        raise ValueError("基础篇 SELECT 执行流程章节数量异常")
    return cleaned.replace(
        select_heading,
        f"{BASIC_NORMAL_FORMS_SECTION.rstrip()}\n\n{select_heading}",
        1,
    )


def add_secondary_index_leaf_note(intro: str) -> str:
    """
    输入：intro，索引篇现有的八股问答 Markdown。
    输出：在覆盖索引章节前插入二级索引叶子节点说明，内容已存在时保持原样。
    功能：确保 InnoDB 二级索引携带聚簇索引键的知识不会因重建 Notebook 丢失。
    """
    marker = "#### 二级索引叶子节点一定包含主键吗？"
    anchor = "### 6. 覆盖索引是什么？"
    if marker in intro:
        return intro
    if intro.count(anchor) != 1:
        raise ValueError("索引篇覆盖索引章节数量异常")
    return intro.replace(
        anchor,
        f"{SECONDARY_INDEX_LEAF_NOTE.rstrip()}\n\n{anchor}",
        1,
    )


def load_learning_sections(doc_file: str) -> tuple[str, str, str, str]:
    """
    输入：doc_file，不含扩展名的文档名称。
    输出：标题、八股问答、面试话术、自查清单四段干净文本。
    功能：优先读取 Markdown；不存在时从当前 Notebook 的首尾单元格恢复。
    """
    markdown_path = ROOT / f"{doc_file}.md"
    if markdown_path.exists():
        source = markdown_path.read_text(encoding="utf-8")
        ending_source = source
    else:
        notebook = nbf.read(NOTEBOOK_DIR / f"{doc_file}.ipynb", as_version=4)
        source = notebook.cells[0].source
        ending_source = notebook.cells[-1].source
    title = source.splitlines()[0].lstrip("# ")
    has_practice_section = any(
        heading_matches(line, "## 实操") for line in source.splitlines()
    )
    intro_end = "## 实操" if has_practice_section else "## 面试话术"
    intro = read_section_text(source, "## 八股问答", intro_end)
    intro_lines = intro.splitlines()
    intro_lines[0] = "## 八股问答"
    intro = "\n".join(intro_lines).strip() + "\n"
    if doc_file == "01-基础":
        intro = replace_basic_crud_section(intro)
        intro = add_basic_normal_forms_section(intro)
    if doc_file == "02-索引":
        intro = add_secondary_index_leaf_note(intro)
    talk = read_section_text(ending_source, "## 面试话术", "## 自查清单")
    checklist = read_section_text(ending_source, "## 自查清单", None)
    return title, intro, talk, checklist


def build_notebook(doc_file: str) -> nbf.NotebookNode:
    """
    输入：doc_file，不含扩展名的文档名称。
    输出：只包含两个 Markdown 单元格的 NotebookNode。
    功能：组装知识讲解与面试复习内容，不生成任何练习或代码单元格。
    """
    title, intro, talk, checklist = load_learning_sections(doc_file)
    return nbf.v4.new_notebook(
        cells=[
            nbf.v4.new_markdown_cell(f"# {title}\n\n{intro}"),
            nbf.v4.new_markdown_cell(f"{talk}\n\n{checklist}"),
        ]
    )


def main() -> None:
    """
    输入：无。
    输出：覆盖生成 notebooks/ 下十份阅读型 .ipynb 文件。
    功能：遍历全部主题并移除历史练习单元格和执行输出。
    """
    for doc_file in NOTEBOOKS:
        notebook = build_notebook(doc_file)
        target = NOTEBOOK_DIR / f"{doc_file}.ipynb"
        nbf.write(notebook, target)
        print(f"生成 {target.name}（{len(notebook.cells)} 个 Markdown 单元格）")


if __name__ == "__main__":
    main()
