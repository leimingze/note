tag	谁读	干嘛用
json:"name"	encoding/json	序列化 JSON key
db:"user_name"	ORM（如 GORM）	映射数据库列名
validate:"required"	go-playground/validator	校验字段不能为空
type User struct {
	Name string
	Age  int
}

u := User{Name: "Alice", Age: 18}
b, _ := json.Marshal(u)
fmt.Println(string(b))
//{"Name":"Alice","Age":18}
```
有tag的情况
```go
type User struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}
u := User{Name: "Alice", Age: 18}
b, _ := json.Marshal(u)
fmt.Println(string(b))
//{"name":"Alice","age":18}
```