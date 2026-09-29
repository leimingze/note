"""
脚本功能：
提供一个通过 Claude Code CLI 审查文本的 MCP 工具。

启动命令：
python server.py
"""

import subprocess

from mcp.server.mcpserver import MCPServer


# ==================== 常量与配置 ====================

CLAUDE_COMMAND = "claude"
CLAUDE_TIMEOUT_SECONDS = 120


# ==================== MCP 服务 ====================

server = MCPServer("claude-review")


@server.tool()
def review_text(text: str) -> str:
    """
    输入：text，需要审查的非空文本。
    输出：Claude 返回的审查意见；输入为空或 CLI 调用失败时抛出异常。
    功能：通过本机 Claude Code CLI 审查文本，并将结果返回给 MCP 客户端。
    """
    if not text.strip():
        raise ValueError("待审查文本不能为空")

    prompt = f"""请审查下面的文本，指出主要问题并给出具体修改建议：

{text}
"""

    result = subprocess.run(
        [CLAUDE_COMMAND, "--print", "--tools", "", "--no-session-persistence"],
        input=prompt,
        capture_output=True,
        text=True,
        check=False,
        timeout=CLAUDE_TIMEOUT_SECONDS,
    )

    if result.returncode != 0:
        raise RuntimeError(f"Claude CLI 调用失败：{result.stderr.strip()}")

    return result.stdout.strip()


# ==================== 命令行入口 ====================

def main() -> None:
    """
    输入：无，使用当前进程的标准输入输出。
    输出：启动 stdio MCP Server 并持续等待请求；启动失败时抛出异常。
    功能：运行 claude-review MCP Server，供 Codex 等 MCP 客户端调用。
    """
    server.run(transport="stdio")


if __name__ == "__main__":
    main()
