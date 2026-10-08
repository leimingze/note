# 事务
MULTI（开启事物） → 命令入队(不执行) → EXEC（执行事务） → 一次性全部执行
         ↑
      DISCARD（取消事务） → 清空队列

## 特性
| 特性 | SQL 数据库（如 MySQL） | Redis 事务 | 说明 |
|------|----------------------|-----------|------|
| **原子性** | ✅ 全成功 / 全回滚 | ⚠️ 部分支持 | 入队阶段报错（语法错误）：`EXEC` 不执行，全部不执行；运行时错误（如对字符串执行 `INCR`）：仅当前命令失败，其余继续执行，**不回滚** |
| **隔离性** | ✅ 多种隔离级别（RC / RR / Serializable） | ✅ 天然隔离 | Redis 单线程执行，事务期间不被其他客户端打断，等价于 **Serializable** 级别 |
| **一致性** | ✅ 事务前后数据一致 | ⚠️ 需应用层保证 | Redis 只保证命令按顺序执行，不校验业务约束，一致性由业务代码或 Lua 脚本维护 |
| **持久性** | ✅ 提交后持久化（WAL / Redo Log） | ❌ 取决于持久化配置 | 未开启 RDB/AOF 或 AOF 未配置 `always` 刷盘时，仍可能丢失数据 |

## 事务的错误处理
```bash
# 情况 1：入队时语法错误 → 整个事务不执行
127.0.0.1:6379> MULTI
OK
127.0.0.1:6379> SET k1 v1
QUEUED
127.0.0.1:6379> SET k2          # ❌ 语法错误
(error) ERR wrong number of arguments for 'set' command
127.0.0.1:6379> EXEC
(error) EXECABORT Transaction discarded because of previous errors.
# k1 也不会被设置 ✅

# 情况 2：运行时错误 → 正确命令照常执行，错误的不影响
127.0.0.1:6379> MULTI
OK
127.0.0.1:6379> SET k1 v1
QUEUED
127.0.0.1:6379> LPUSH k1 value   # k1 是 string，不能 LPUSH
QUEUED
127.0.0.1:6379> EXEC
1) OK
2) (error) WRONGTYPE Operation against a key holding the wrong kind of value
# SET k1 成功了，LPUSH 失败了！⚠️
```

## WATCH 乐观锁（CAS）
```bash
# 转账场景：从 account:A 转 100 给 account:B
127.0.0.1:6379> WATCH account:A account:B
OK
127.0.0.1:6379> GET account:A
"500"
127.0.0.1:6379> MULTI
OK
127.0.0.1:6379> DECRBY account:A 100
QUEUED
127.0.0.1:6379> INCRBY account:B 100
QUEUED
127.0.0.1:6379> EXEC
# 如果 EXEC 之前 account:A 或 account:B 被其他客户端修改了 → 返回 nil
# 本事务不会执行
```

# lua脚本
lua是redis的事务替代方案，提供了真正的原子性


# pipeline
pipeline不是原子操作，只是一个批量发送的优化，优化网络返回次数
```bash
# 不使用 Pipeline（N 次 RTT）
SET k1 v1  →  OK
SET k2 v2  →  OK
SET k3 v3  →  OK
# 3 次网络往返

# 使用 Pipeline（1 次 RTT）
(printf "SET k1 v1\r\nSET k2 v2\r\nSET k3 v3\r\n"; sleep 1) | redis-cli --pipe
# All data transferred. Waiting for the last reply...
# Last reply received from server.
# 仅 1 次网络往返！
```

# Redis 三种批量操作方式对比

| 对比维度 | MULTI/EXEC 事务 | Pipeline 管道 | Lua 脚本 |
|---|---|---|---|
| **原子性** | ⚠️ 执行阶段不插入其他命令 | ❌ 不保证 | ✅ 完全原子 |
| **回滚** | ❌ 不支持 | ❌ 不支持 | ❌ 不支持 |
| **减少 RTT** | ✅ 打包发送 | ✅ 打包发送 | ✅ 一次 RTT |
| **逻辑能力** | ❌ 纯命令执行 | ❌ 纯命令执行 | ✅ 条件/循环/计算 |
| **可复用** | ❌ | ❌ | ✅ EVALSHA 复用 |
| **集群兼容** | ⚠️ 同一 node | ✅ 无限制 | ⚠️ 同一 slot |
| **性能** | ⭐⭐ | ⭐⭐⭐ | ⭐⭐ |
| **使用场景** | 简单原子操作 | 批量读写 | 复杂原子逻辑 |

# 场景选择
```bash
场景 1：转账（读 → 计算 → 写，需要原子性）
  → Lua 脚本（原子性最强）

场景 2：批量 SET 1000 个 key（不需要原子性）
  → Pipeline（性能最优）

场景 3：用 WATCH 实现乐观锁转账
  → MULTI/EXEC + WATCH（复杂但灵活）

场景 4：抢红包（需要 CAS 比较 + 原子操作）
  → Lua 脚本（一条命令搞定）
```