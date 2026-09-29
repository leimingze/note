# 什么是io多路复用
- 本质：一个线程同时监听多个文件描述符（fd），当其中某些 fd 就绪时再进行处理。
- 解决的核心问题：大量连接、少量活跃​ 场景下的高并发问题。
- 经典场景：网络服务器（Web Server、Redis、Nginx），大量客户端连接，但大部分时间都在“等”


| 多路复用 | 数据结构 | 获取就绪事件的方式 | 复杂度 |
| :--- | :--- | :--- | :--- |
| select | 位图（fd_set） | 全量扫描 | O(n) |
| poll | 链表（pollfd） | 全量扫描 | O(n) |
| epoll | 红黑树 + 就绪链表 | 直接返回就绪事件 | O(1) |

```c
// 1. 创建一个 epoll 句柄，后面所有操作都靠它
int epfd = epoll_create(1024);

// 2. 告诉 epoll：帮我盯着这个 fd，它可读的时候通知我
struct epoll_event ev;
ev.events = EPOLLIN;
ev.data.fd = client_sockfd;
epoll_ctl(epfd, EPOLL_CTL_ADD, client_sockfd, &ev);

// 3. 等着，谁就绪了就返回（没就绪就一直睡）
struct epoll_event events[MAX_EVENTS];
int nfds = epoll_wait(epfd, events, MAX_EVENTS, timeout);

// 4. 挨个处理就绪的 fd（只有就绪的才会出现在这里）
for (int i = 0; i < nfds; i++) {
    handle_event(events[i].data.fd);
}
```

epoll 为什么快？ 因为它把"等待事件"和"轮询就绪 fd"分开了。epoll_wait 直接返回已经发生事件的 fd 列表，不用像 select 那样遍历所有 fd。