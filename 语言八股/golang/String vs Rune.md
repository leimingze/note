# 什么是 rune 类型？

Go 语言的字符有以下两种：

- uint8 类型，或者叫 byte 型，代表了 ASCII 码的一个字符。

- rune 类型，代表一个 Unicode 码点（code point），当需要处理中文、日文或者其他非 ASCII 字符时，则需要用到 rune 类型。rune 是 int32 的别名（UTF-8 是 Go 字符串的底层编码方式，rune 表示的是 Unicode 标量值，两者概念不同）。
```go
package main
import "fmt"

func main() {
    var str = "hello 你好" //思考下 len(str) 的长度是多少？

    //golang中string底层是通过byte数组实现的，直接求len 实际是在按字节长度计算
    //所以一个汉字占3个字节算了3个长度
    fmt.Println("len(str):", len(str))  // len(str): 12

    //通过rune类型处理unicode字符
    fmt.Println("rune:", len([]rune(str))) //rune: 8
}
```

# 如何高效的拼接字符串
| 拼接方式 | 示例代码 | 性能 | 适用场景 | 优点 | 缺点 |
|---------|---------|------|---------|------|------|
| `+` 运算符 | `s := "Hello, " + "World!"` | 低 | 少量字符串拼接 | 简单直观 | 每次拼接产生新字符串，频繁使用性能差 |
| `fmt.Sprintf` | `s := fmt.Sprintf("%s-%d", "id", 1001)` | 较低 | 混合类型、格式化输出 | 支持任意类型、格式化能力强 | 反射解析，性能一般 |
| `strings.Builder` | `var sb strings.Builder`<br>`sb.WriteString("Hello")`<br>`s := sb.String()` | **高** | 循环、大量拼接 | 内存分配少、性能最好 | 非线程安全 |
| `bytes.Buffer` | `var buf bytes.Buffer`<br>`buf.WriteString("Hello")`<br>`s := buf.String()` | 中高 | 需要 IO 操作的场景 | 实现 `io.Writer` | 比 `strings.Builder` 稍慢 |
| `strings.Join` | `s := strings.Join([]string{"a","b"}, "-")` | 高 | 字符串切片拼接 | 一次内存分配、效率高 | 需先构造切片 |
| 场景 | 推荐方式 |
|------|---------|
| 简单拼接 | `+` |
| 混合类型 / 格式化 | `fmt.Sprintf` |
| 循环 / 大量拼接 | `strings.Builder` |
| 字符串切片拼接 | `strings.Join` |
| 需要 IO 写入 | `bytes.Buffer` |
| 性能排序（由快到慢） |
|---------------------|
| `strings.Builder` > `strings.Join` > `bytes.Buffer` > `+` > `fmt.Sprintf` |