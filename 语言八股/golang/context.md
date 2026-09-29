# context是什么
context.Context 是go标准库的一个接口，用来在goroutine之间传递：
- 取消信号
- 超时控制 
- 截止时间
- 请求作用域的共享数据

# 为什么需要context？
go里大量使用goroutine，没有context时，只能通过通信来通知退出。
我希望：请求取消 / 超时 → 所有相关 goroutine 立刻停止

# 底层

```go
type Context interface {
    Deadline() (deadline time.Time, ok bool)//返回截止时间（如果有）
    Done() <-chan struct{}//返回一个只读 channel，关闭表示“该退出了”
    Err() error//返回取消原因（Canceled / DeadlineExceeded）
    Value(key any) any//存/取请求作用域的数据
}
```