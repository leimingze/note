```go
func foo() error {
    if 出错 {
        var err *MyError = nil
        // return err  // ❌ 陷阱：返回了 (T=*MyError, V=nil)，err!=nil
        return nil     // ✅ 正确：直接返回 (T=nil, V=nil)
    }
    return nil
}
```