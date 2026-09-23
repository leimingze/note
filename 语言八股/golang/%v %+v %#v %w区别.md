```go
package main

import "fmt"

type User struct {
	ID   int
	Name string
	Age  int8
}

func main() {
	u := &User{ID: 101, Name: "张三", Age: 25}

	fmt.Printf("%%v: %v\n", u)//&{101 张三 25} 最简洁，只打印字段值，不显示字段名（指针带 &）
	fmt.Printf("%%+v: %+v\n", u)//&{ID:101 Name:张三 Age:25} 调试首选，增加了字段名，清晰对应键值
	fmt.Printf("%%#v: %#v\n", u)//&main.User{ID:101, Name:"张三", Age:25}语法级，包含包名、类型、字段名，字符串带引号，可直接当代码

    //如果执行 fmt.Errorf("用户错误: %v", u)，内部只是用上述 %v 的规则（即 {101 张三 25}）拼成字符串。这会产生断链，调用方无法再用 errors.Is 追溯底层错误。若包装错误必须用 %w，而打印值才用这里的 %v 系列。
}
```