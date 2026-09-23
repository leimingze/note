专门设计一个类型，用来装出错的信息。
```go
import "errors"

err := errors.New("文件不存在")
fmt.Println(err)  // 输出：文件不存在
```

自定义错误类型：
```go
//第一步 定义结构体
type argError struct {
    arg  int//传进来的参数
    prob string//出错原因
}
//第二步变成error，自定义打印类型
//error接口要求只有一个方法：Error() string
func (e *argError) Error() string {
    return fmt.Sprintf("%d - %s", e.arg, e.prob)
}
//第三步大当error用
func f2(arg int) (int, error) {
    if arg == 42 {
        return -1, &argError{arg, "can't work with it"}
    }
    return arg + 3, nil
}
//第四步打印
r, e := f2(42)
fmt.Println(e)//42 - can't work with it
//第五步从error中取出来
ae, ok := e.(*argError)
//e.(*argError) "e 里面装的是不是 *argError？"
//ae 如果是，ae 就是转出来的 *argError 指针
//ok true 表示转成功了，false 表示转失败了
fmt.Println(ae.arg)    // 42
fmt.Println(ae.prob)  // can't work with it
```
