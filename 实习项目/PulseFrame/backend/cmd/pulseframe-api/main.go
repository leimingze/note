// 程序功能：读取配置，创建 MySQL 和 Redis 客户端，并启动 PulseFrame 核心 API。
// 启动命令：在 backend 目录执行 go run ./cmd/pulseframe-api；环境变量见 backend/README.md。
package main

import (
	"context"
	"log/slog"
	"os"

	"pulseframe/internal/app"
)

// main 是 PulseFrame 核心 API 的进程入口。
// 输入：进程环境变量和操作系统退出信号。
// 输出：启动 HTTP 服务；初始化或运行失败时记录错误并以非零状态退出。
// 功能：把进程环境交给应用启动函数，启动或运行失败时记录错误并退出。
// 例如数据库配置有效且端口可用时启动服务；数据库无法连接时记录错误并以状态码1退出。
func main() {
	// 入口无法向调用方返回错误，因此记录根因并用非零状态通知进程管理器。
	if err := app.Run(context.Background(), os.LookupEnv); err != nil {
		slog.Error("pulseframe api stopped with error", "error", err)
		os.Exit(1)
	}
}
