# go不像java/python一样把方法写到类里。
go实际是这样写的：
```go
type rect struct {
    width, height float64
}

func (r rect) area() float64 {   // ← 写在外面
    return r.width * r.height
}
```
go不要class，故意为之，因为继承多了，没人敢改父类
