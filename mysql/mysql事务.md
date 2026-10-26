# MySQL 事务

适用范围：MySQL 8.4，InnoDB 存储引擎。

事务特性定义正确性要求，隔离级别规定并发事务允许观察到的行为，MVCC 与锁提供相应的读取和并发控制机制。

## 事务特性（ACID）

| 特性 | 定义 | 主要保障 |
| --- | --- | --- |
| 原子性 | 同一事务的修改全部提交或全部撤销 | undo log 支持回滚 |
| 一致性 | 事务执行前后均满足数据约束与业务规则 | 业务逻辑、数据库约束及其他事务特性共同保证 |
| 隔离性 | 按所选级别限制并发事务之间的相互影响 | MVCC 与锁 |
| 持久性 | 已提交修改在故障后能够恢复 | redo log 与可靠的持久化配置 |

例如，A 有 100 元、B 有 50 元，不计手续费的转账应保持总额 150 元；扣款与入账还必须属于同一事务。

`innodb_flush_log_at_trx_commit=1` 表示每次提交写入并刷新 redo log，仍依赖操作系统与存储设备正确完成刷新。设置为 0 或 2 会改变故障时可能丢失的范围。

## 并发读取异常

| 现象 | 定义 | 关注对象 |
| --- | --- | --- |
| 脏读（Dirty Read） | 读取其他事务尚未提交的修改 | 未提交版本 |
| 不可重复读 | 同一事务重复读取同一行，因其他事务提交修改而读到不同值 | 行值变化 |
| 幻读 | 同一事务重复执行相同条件的查询，因其他事务的写入而得到不同的记录集合 | 结果集合变化 |

幻读应检查记录成员，数量相同也可能存在集合变化。三种现象的业务严重程度没有统一排序。

## 隔离级别

| 级别 | 普通读取方式 | 并发现象 |
| --- | --- | --- |
| 读未提交 RU | 允许读取未提交修改 | 脏读、不可重复读、幻读均可能出现 |
| 读提交 RC | 每次一致性读创建新 Read View | 排除脏读，仍可能不可重复读、幻读 |
| 可重复读 RR（默认） | 首次一致性读创建 Read View，之后复用 | 标准不强制排除幻读；InnoDB 结合快照与范围锁提供保证 |
| 串行化 SERIALIZABLE | 关闭自动提交时，普通 SELECT 转为共享加锁读 | 避免三种现象；冲突操作可能等待 |

隔离保证：串行化 > 可重复读 > 读提交 > 读未提交。该顺序不代表固定的性能排名。

SERIALIZABLE 允许事务并发，锁冲突时才需要等待；自动提交的单条只读 SELECT 可以使用一致性读。

### 读取时序示例

初始余额 100 万。TA 只查询，TB 将余额更新为 200 万。RU、RC、RR 使用同一时序：TB 更新未提交时读取 V1，TB 提交后读取 V2，TA 提交并开启新事务后读取 V3。

| 级别 | V1（万） | V2（万） | V3（万） | 原因 |
| --- | --- | --- | --- | --- |
| RU | 可读到 200 | 200 | 200 | 允许提前读到未提交修改 |
| RC | 100 | 200 | 200 | 后一次查询使用新快照 |
| RR | 100 | 100 | 200 | 同一事务复用快照 |

SERIALIZABLE 使用独立时序：TA 关闭自动提交并先读取持锁，TB 更新等待；TA 再次读取后提交，TB 才能更新并提交。此时 V1=V2=100 万，新事务中的 V3=200 万。若 TB 先持有写锁，TA 首次读取可能等待。

## 快照读与当前读

| 读取方式 | 语义 | 常见操作 |
| --- | --- | --- |
| 快照读（一致性读） | 按 Read View 选择可见版本，通常不对读取记录加锁 | RC、RR 下的普通 SELECT |
| 当前读 | 读取加锁时可见的记录，并约束冲突访问 | SELECT ... FOR UPDATE、SELECT ... FOR SHARE、UPDATE、DELETE |

RC、RR 均能读取本事务此前的修改。当前读不沿用先前普通查询的快照结论，遇到锁冲突时可能等待。

## MVCC

MVCC（多版本并发控制）通过历史版本与可见性规则支持一致性读，减少普通读取与写入之间的锁等待。undo log 提供重建历史版本的信息，Read View 决定版本是否可见。

### 版本链

| 集簇索引隐藏列 | 含义 |
| --- | --- |
| `trx_id`（DB_TRX_ID） | 最后插入或更新记录的事务 ID；删除标记也属于修改 |
| `roll_pointer`（DB_ROLL_PTR） | 指向相关 undo log 信息，用于重建旧版本 |

当前版本不可见时，沿 undo 信息重建旧版本并重新判断；找到可见版本后停止，没有可见历史则不返回该行。二级索引记录不直接保存上述两列。

### Read View

Read View 记录创建时的事务状态，用于判断版本可见性，无需复制整张数据表。

| 字段 | 含义 |
| --- | --- |
| `m_ids` | 创建时其他活跃读写事务的 ID 集合，排除创建者 |
| `min_trx_id` | `m_ids` 的最小值；集合为空时等于 `max_trx_id` |
| `max_trx_id` | 创建时下一个待分配的读写事务 ID 边界 |
| `creator_trx_id` | 创建者事务 ID，用于识别自身修改 |

源码对应：`min_trx_id` 为 `m_up_limit_id`，`max_trx_id` 为 `m_low_limit_id`，`creator_trx_id` 为 `m_creator_trx_id`。

RR 通常在首次一致性读时创建 Read View；`START TRANSACTION WITH CONSISTENT SNAPSHOT` 可在开始时建立快照。复用期间，其他事务提交不会更新已有视图。

### 可见性判断

先判断自身修改，再对其他事务的版本比较上下界和活跃事务集合。

| 判断顺序 | 条件 | 结果 |
| --- | --- | --- |
| 1 | `trx_id == creator_trx_id` | 自身修改可见 |
| 2 | 其他事务，`trx_id < min_trx_id` | 可见 |
| 3 | 其他事务，`trx_id >= max_trx_id` | 不可见，继续找旧版本 |
| 4 | `min_trx_id <= trx_id < max_trx_id`，且在 `m_ids` 中 | 当时未提交，不可见 |
| 5 | `min_trx_id <= trx_id < max_trx_id`，且不在 `m_ids` 中 | 当时已提交，可见 |

事务 ID 的分配顺序不代表提交顺序，因此中间范围必须检查 `m_ids`。快照建立后插入的记录，可能被上界排除，也可能被创建时的活跃事务集合排除。

## 锁与幻读

快照读通过版本可见性保持历史视图，锁通过约束冲突写入保护访问范围。

### 锁的类型

以下设 `id` 为主键，索引已有 1、3、5，采用 RR 显式事务。

| 锁 | 保护位置 | 例子 |
| --- | --- | --- |
| 记录锁 | 已有索引记录 | 完整唯一条件 `WHERE id=3 FOR UPDATE` 命中记录，只锁 id=3，不阻止插入 id=2 |
| 间隙锁 | 记录之间的插入位置 | 间隙为 `(-∞,1)`、`(1,3)`、`(3,5)`、`(5,+∞)`；保护 `(1,3)` 会阻止其他事务插入 id=2 |
| next-key lock | 一条记录及其前方间隙 | 保护记录和对应插入位置，具体范围取决于索引扫描 |

### RR 的幻读处理边界

- **快照读**：相同条件下复用 Read View，本事务未修改相关记录时，其他事务的新提交不会改变历史结果；此机制不阻止插入。
- **当前读**：范围查询按索引扫描取得必要的记录锁、间隙锁或 next-key lock，阻止其他事务在已保护的范围内插入；相关锁通常保留到事务结束。
- **结果变化**：混用快照读与当前读，或本事务修改相关记录，后续查询的结果集合仍可能改变。

RC 的常规搜索与索引扫描不提供 RR 同样的间隙保护；外键检查和重复键检查仍可能使用间隙锁。

## 配图与参考资料

配图：[脏读](assets/image6.png)、[不可重复读](assets/image7.png)、[幻读](assets/image8.png)、[隔离级别时序](assets/image9.png)。

- [MySQL 8.4：一致性非锁定读](https://dev.mysql.com/doc/refman/8.4/en/innodb-consistent-read.html)
- [MySQL 8.4：事务隔离级别](https://dev.mysql.com/doc/refman/8.4/en/innodb-transaction-isolation-levels.html)
- [MySQL 8.4：多版本机制](https://dev.mysql.com/doc/refman/8.4/en/innodb-multi-versioning.html)
- [MySQL 8.4：InnoDB 锁](https://dev.mysql.com/doc/refman/8.4/en/innodb-locking.html)
- [MySQL 8.4：InnoDB 与 ACID](https://dev.mysql.com/doc/refman/8.4/en/mysql-acid.html)
- [MySQL 8.4.0：Read View 可见性源码](https://raw.githubusercontent.com/mysql/mysql-server/mysql-8.4.0/storage/innobase/include/read0types.h)
- [MySQL 8.4.0：Read View 创建源码](https://raw.githubusercontent.com/mysql/mysql-server/mysql-8.4.0/storage/innobase/read/read0read.cc)
