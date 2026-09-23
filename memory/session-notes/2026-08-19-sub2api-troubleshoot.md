# Sub2API 服务器排查会话（2026-08-19）

## 背景
用户要求按 `notes/sub2api检测.md` 对 47.237.8.6（2核/1.6G/无swap）上的 Sub2API 中转服务做系统性只读排查：TTFT 高、偶发 502/499。

## 关键结论（详见 notes/sub2api检测报告.md）

1. **服务器卡死根因**：1.6G 内存 + 无 swap + 高峰期并发大请求（body 达 10.6MB）→ 内存耗尽。8/19 19:29 容器卡死（连定时日志都停），19:37:45 journald 报 "Under memory pressure" 后系统冻结，所有端口握手成功但零数据。21:25 用户重启恢复。
2. **TTFT 高根因**：OpenAI Codex 上游高峰期排队/过载。xhigh 长推理请求（100-650s）占满单账号 16 并发槽，后续请求排队；502 全天 136 次集中在 14-17 点，failover 92 次但单账号无效；499 = 上游 502 → failover → 客户端断开（非客户端网络问题）。
3. **已排除**：本地网络（TTFB 52-267ms）、HTTP2（GOAWAY 4条）、conntrack（80/65536）、CPU/磁盘、compaction（0条）。
4. **架构实际**：Caddy（宿主机）→ sub2api:8080，非文档所述"无 Nginx"；Redis 连接池 4096/min idle 256、Postgres 256 上限配置过激。

## 服务器操作经验
- SSH: `ssh root@47.237.8.6`（密钥 ~/.ssh/id_ed25519），root 免 sudo
- 源码在服务器 `/opt/sub2api/src/`（后端 Go：backend/internal/）
- 容器日志用 `docker logs sub2api --since ...`（历史日志保留，服务器重启不丢）
- usage_logs 表：duration_ms / first_token_ms / reasoning_effort / cache_read_tokens 字段
- 8/19 后已给 sub2api 加内存监控建议（P0 未执行，待用户确认）

## 未完成
- P0-P2 优化方案未执行（需用户确认，涉及改配置/重启）

## 已执行变更（2026-08-19 22:14）
- **首输出超时已生效**：docker-compose.yml 增加 `GATEWAY_OPENAI_FIRST_OUTPUT_TIMEOUT_SECONDS=180`（默认值，可用 .env 覆盖），sub2api 容器已重建生效。效果：上游 180s 无首个语义输出 → failover/报错，不再无限挂。备份：`/opt/sub2api/src/deploy/docker-compose.yml.bak-*`。
- **effort 限制：用户决定不限制**（保持 group 2 max_reasoning_effort 为空）。注意：Sub2API 支持 group 级 `max_reasoning_effort` / `reasoning_effort_mappings`（改库即时生效），可强制改写客户端请求体里的 effort，是降 TTFT 最有效手段（xhigh→medium 可降约 75%），用户随时可再启用。
- **swap：已执行**（用户确认）——2G swapfile（/swapfile，/etc/fstab 已配置开机自启），swapon 已启用。现在可用内存 842MB + swap 2G 应急空间，防再次整机冻结。

## 复检结论（2026-08-19 23:10，详见 sub2api检测报告.md 第 8-14 节）

- 资源健康（可用内存 829Mi、swap 未用、CPU idle 97%）；网络正常（容器内 TTFB 32ms）。
- **新发现**：24h 内 routing 阶段 503 共 **516 次**，同一分钟密集爆发（09:54/14:18/16:27），原因=上游 502/超时 → account+model 连续失败冷却 → 选号失败 503（源码 openai_account_runtime_block_fastpath.go / no_account_error.go）。429（队列满）为 0，排除排队溢出。
- 上游 502 88 次、客户端 499 52 次，仍集中在 14-17h；xhigh/max 的 P50 已降到 ~5s（180s 超时生效），但 P90 长尾 19-42s 仍在。
- **放大器确认**：客户端断开后 drain 计费期间并发槽不释放（openai_gateway_handler.go:526-560 defer 在 Forward 内）。
- **用户贴的"config.json timeout/cache_ttl/concurrency=100/host网络"建议全部核验为不适用于本项目**（无 config.json；真实配置=环境变量+DB 账号字段+OpenAI prompt cache）。
- 数据库字段注意：usage_logs **无 status 列**（错误在 ops_error_logs）；accounts 表含 concurrency/overload_until/temp_unschedulable_until/schedulable。psql 查询用 base64 传 SQL 避开引号问题（$$ 会被本地 shell 吞）。
- 未执行项（需用户确认）：加账号≥2、并发 16→10、断开即释放槽位（需源码改动）、RESPONSE_HEADER_TIMEOUT 180s、503 告警。

## 超时调整（2026-08-19 23:22 已生效，用户确认）

- **`GATEWAY_OPENAI_RESPONSE_HEADER_TIMEOUT=20`**（.env:287，原 0=无超时）——合法范围 ≥0。
- **`GATEWAY_OPENAI_FIRST_OUTPUT_TIMEOUT_SECONDS=30`**（.env:461，原 180）——**下限 30s**：设 20 时容器校验失败 `must be 0 or between 30-600 seconds` 反复重启（RestartCount=8），已回退到最小合法值 30。
- 备份：`/opt/sub2api/src/deploy/.env.bak-20260819-232147`（root 600）。容器已重建 healthy，请求正常。
- **预期影响（快速失败策略）**：高峰 14-17h TTFT P50≈15-17s、P90≈60s，20/30s 会掐掉约一半请求，客户端需自行重试；非高峰无感。观察 24h 后决定是否回调。
- 校验源码：`backend/internal/config/config.go:3165-3175`（response_header ≥0；first_output 0 或 30-600；另有 high_effort 变体 30-1800）。
