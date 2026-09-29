# Redis 常见数据结构汇总

## 核心类型与底层编码（5 种）

| 类型 | 底层编码与结构 | 特点 | 典型场景 | 常用命令 |
|---|---|---|---|---|
| String 字符串 | 整数值使用 int；短字符串使用 embstr；较长字符串使用 raw（SDS） | 二进制安全，可存文本、数字或序列化对象，最大 512MB | 缓存、计数器、分布式锁、Session | SET GET INCR DECR EXPIRE SETNX |
| List 列表 | quicklist，节点内通常使用 listpack 保存元素 | 有序、可重复，支持两端 O(1) 操作 | 消息队列、时间线、最新列表 | LPUSH RPUSH LPOP RPOP LRANGE BLPOP |
| Hash 哈希 | 小数据使用 listpack；超过阈值后使用 hashtable | 键值对集合，适合存对象，可单字段读写 | 用户/商品信息、对象缓存 | HSET HGET HGETALL HINCRBY HDEL |
| Set 集合 | 小整数集合使用 intset；通用场景使用 hashtable | 无序、唯一，支持交并差集 | 标签、去重、共同好友、抽奖 | SADD SMEMBERS SINTER SUNION SDIFF SRANDMEMBER |
| ZSet 有序集合 | 小数据使用 listpack；超过阈值后使用跳表 + hashtable | 元素唯一并按 score 排序，范围查询快 | 排行榜、热榜、延迟队列、权重路由 | ZADD ZRANGE ZREVRANGE ZSCORE ZINCRBY ZUNIONSTORE |

> 实际编码转换阈值受 Redis 版本和配置影响，可通过 `OBJECT ENCODING key` 查看指定键当前使用的编码。

## 选型速查

| 需求 | 推荐类型 |
|---|---|
| 简单缓存、计数、分布式锁 | String |
| 对象/商品/用户信息 | Hash |
| 队列、栈、时间线 | List |
| 去重、标签、共同好友、抽奖 | Set |
| 排行榜、热榜、延迟队列 | ZSet |

# redisobject
redisObject 是 Redis 里所有 key / value 的统一对象外壳，用来把“用户看到的数据类型”和“内存里的底层结构”解耦。

# string
## 内部实现
String 类型的底层的数据结构实现主要是 int 和 SDS（简单动态字符串）
```c
struct __attribute__((__packed__)) sdshdr8 {
    uint8_t len;     // 已用字节数
    uint8_t alloc;   // 总分配容量（不含头、不含\0）
    unsigned char flags; // 类型标识
    char buf[];      // 实际数据，末尾仍有 \0（兼容 C）
};
```
## 优点：
1. O(1) 取长度：读 len，不用遍历到 \0
2. 二进制安全：靠 len 判断边界，可以存图片、protobuf、带 \0 的数据
3. 防缓冲区溢出：追加前先检查 alloc - len，不够再扩容
### 扩容
```c
新容量 = 原长度 + 追加长度

规则一：小于 1MB —— 翻倍扩容
if (新容量 < 1MB) {
    新容量 = max(新容量, 2 * 原长度)
}

规则二：大于等于 1MB —— 每次只加 1MB
if (新容量 >= 1MB) {
    新容量 = 新容量 + 1MB
}
```


# list
List 类型的底层数据结构是由双向链表或压缩列表实现的
- 如果列表元素小于512个，列表每个元素的值都小于64字节，redis会使用压缩列表作为list类型的底层数据结构。
- 如果列表的元素不满足上面的条件，redis会使用双向链表。

## 版本演进
- Redis < 3.2：小数据用 ziplist，大数据用 linkedlist
- Redis 3.2 ～ 6.x：统一为 quicklist（双向链表 + 每个节点是 ziplist）
- Redis 7.0+：还是 quicklist，但节点内部从 ziplist 换成 listpack

## ziplist
```text
[zlbytes][zltail][zllen][entry1][entry2]...[entryN][zlend]
```
- zlbytes：总字节数
- zltail：尾节点偏移（方便反向找尾）
- zllen：元素个数
- zlend：结束标记 0xFF
- entry
    - [prevlen][encoding][data]
### 致命问题：连锁更新
- 前一个 entry < 254 字节：prevlen 占 1 字节
- 前一个 entry ≥ 254 字节：prevlen 占 5 字节（0xFE + 4字节长度）
假设所有 entry 都是 253 字节，prevlen 各占 1 字节：
[1B][253B] [1B][253B] [1B][253B] ... ← 每个 entry 254 字节
现在修改第一个 entry 为 254 字节 → 第二个 entry 的 prevlen 需要从 1B → 5B：
[1B→5B][254B] [1B→5B][253B] [1B→5B][253B] ...
              ↑ 第二个变 257 Byte → 第三个也需要扩展...
连锁触发！O(N²) 复杂度


## listpack
```text
[total-bytes][num-elements][entry1][entry2]...[entryN][0xFF]
```
- total-bytes 整个 listpack 的总字节数
- num-elements 元素个数
- entry 每个元素
    - [encoding][data][backlen]
- 0xFF 结束标记（和 ziplist 一样）

### 解决了连锁更新的问题
ziplist每个entry存前一个entry的长度，前一个变了，后边就变了。
listpack的每个netry只对自己负责。

## quicklist
```c
// /src/quicklist.h
typedef struct quicklist {
    quicklistNode *head;      // 头节点
    quicklistNode *tail;      // 尾节点
    unsigned long count;      // 所有 entry 总数
    unsigned long len;        // quicklistNode 数量
    int fill : QL_FILL_BITS;  // 填充因子（控制每个节点中 ziplist 的元素数量）
    unsigned int compress : QL_COMP_BITS; // 压缩深度
} quicklist;

typedef struct quicklistNode {
    struct quicklistNode *prev;
    struct quicklistNode *next;
    unsigned char *entry;     // 指向 ziplist/listpack
    size_t sz;                // ziplist/listpack 字节数
    unsigned int count : 16;  // 此节点中 entry 数量
    unsigned int encoding : 2;// RAW==1 or LZF==2
    // ...
} quicklistNode;
```

# hash
#TODO 补充
Hash 类型的底层数据结构是由压缩列表或哈希表实现的：
- 如果哈希元素个数小于512个，所有值小于64字节的话，redis会使用压缩列表作为hash类型的底层数据结构
- 如果哈希类型元素不满足上面条件，redis会使用哈希表作为hash类型的底层数据结构。

# set
#TODO 补充

# zset
#TODO 补充