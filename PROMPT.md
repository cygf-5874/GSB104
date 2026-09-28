解析出来的值再格式化回去，必须和原文是同一个时长。durparse 是 Go 1.24 的
时长解析与格式化库，只用标准库（无第三方依赖），构建走 go 工具链，
自检走 `scripts/check.sh`（`check/` 是固定验收程序，别改）。

`durparse.go` 现在只有类型和签名，方法体全是 panic；`errors.go` 里两个错误值已经给全。

任务：按 README「对外契约」的 10 条实现，让固定件全过。

验收：
- go build ./... 退出码 0；
- go run ./check 退出码 0，10 个场景全过（parse 4 + range 2 + format 2 + roundtrip 2）。

约束：
1. 不改 `check/`、不改 `errors.go` 里错误值的名字与含义；可以新增文件与类型。
2. 不引入第三方依赖，`go.mod` 里不许出现 `require`。
3. 对外类型与函数签名已定死，不要改。
4. 往返对 int64 全域成立，包括最小值；结果与遍历顺序无关。
