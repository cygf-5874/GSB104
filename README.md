# durparse

时长的**解析与格式化**库（Go 1.24，仅标准库，`go.mod` 无 `require`）。

`Parse` 把 `"1h30m"`、`"2m0.5s"` 这样的字符串解析成**纳秒整数**；
`Format` 把纳秒整数格式化成**最短**的时长字符串。两者互为逆运算。

```go
ns, err := durparse.Parse("-1h30m") // -5400000000000
s := durparse.Format(ns)            // "-1h30m"
```

## 怎么跑

```bash
go build ./...            # 编译
go run ./check            # 固定验收程序（10 个场景）
go run ./check -list      # 列出全部场景
go run ./check --only parse
bash scripts/check.sh     # 薄封装：go run ./check "$@"
```

前提：Go 1.24。`check/`、`scripts/` 是固定验收的一部分，**请勿修改**。

## 对外契约

下面 10 条是 `Parse` 与 `Format` 的契约。**它们是契约，不是「当前行为」的转述。**

1. **`Parse(s string) (int64, error)`**：返回**纳秒整数**。支持的单位：
   `ns`、`us`、`µs`、`μs`（后两个是不同码位的微秒符号）、`ms`、`s`、`m`、`h`。
2. **多段**：一个字符串里可以含多段，如 `1h30m`、`2m0.5s`、`1h2m3s4ms5us6ns`。
   每段的**单位必须严格递减**（`h > m > s > ms > us > ns`，相等也不允许），
   否则返回 `ErrSyntax`。
3. **符号**：允许前导 `+` / `-`；`-` 作用于**整串**（`-1h30m` = 负一个半小时）。
   空串、只有符号（`-`、`+`）、只有单位（`s`、`ns`）都返回 `ErrSyntax`。
4. **小数**：小数点只允许出现在**最后一段**（`1.5h` 合法，`1.5h30m` 非法，`1.5h1.5m` 非法）。
   小数部分**截断到 ns**（对绝对值向下取整，`1.0000000009s` → `1s`）。
5. **零**：`0`（无单位）合法，返回 `0`；`0s`、`00h`、`000ns` 也合法。其它无单位的形式非法。
6. **溢出**：结果超出 `int64` 纳秒范围返回 `ErrRange`；**中间累加也要防溢出**
   （不能先溢出回绕、再拿回绕后的值去判界）。
7. **`Format(ns int64) string`**：输出**最短**形式 —— 系数都是整数，**省略系数为 0 的单位**，
   单位按 `h`、`m`、`s`、`ms`、`us`、`ns` 的固定顺序拼接，不输出小数。`Format(0)` 为 `"0s"`。
8. **往返性质**：`Parse(Format(ns)) == ns` 对**任意** `int64` 成立（含 `math.MinInt64`）。
9. **负值**：`Format` 对负值输出前导 `-`，且**单位序列**与对应正值相同（`-1h30m` → `-1h30m`）。
10. **不依赖 `map` 迭代顺序**：单位表必须按固定顺序遍历，同一输入多次调用结果逐字节一致。

## 公开接口

```go
var ErrSyntax error
var ErrRange error

func Parse(s string) (int64, error)
func Format(ns int64) string
```

`errors.go` 里两个哨兵错误的名字与含义已定死，不要改；可以新增文件与类型。

## 固定件场景（10 个）

| 分组 | 场景 | 覆盖 |
| --- | --- | --- |
| `parse` | `P1_multi_segments_decreasing` | 契约 1、2：多段、单位严格递减 |
| `parse` | `P2_sign_applies_to_whole` | 契约 3：符号作用于整串 |
| `parse` | `P3_decimal_only_last_segment` | 契约 4：小数只在最后一段、截断到 ns |
| `parse` | `P4_empty_sign_and_unit_only` | 契约 3、5：空串 / 只有符号 / 只有单位 / 零 |
| `range` | `R1_beyond_int64` | 契约 6：超出 int64 报 `ErrRange` |
| `range` | `R2_intermediate_accumulation` | 契约 6：中间累加防溢出、边界恰好可取 |
| `format` | `F1_shortest_omit_zero_units` | 契约 7：最短形式、省略零单位 |
| `format` | `F2_negative_format` | 契约 9：负值格式、`MinInt64` / `MaxInt64` |
| `roundtrip` | `RT1_sequence_roundtrip` | 契约 8：确定序列往返 |
| `roundtrip` | `RT2_min_max_int64` | 契约 8：`MinInt64` / `MaxInt64` 往返 |

`--only <组名>`（`parse` / `range` / `format` / `roundtrip`）可以只跑一组。
10 个场景都在同一个进程里跑，每个场景的 panic 会被单独捕获并记成一条 `FAIL`，
**一个场景挂掉不会遮蔽其余场景**。判据全部是硬编码的期望值，不依赖墙钟、机器速度或随机源。

## 目录

```
.
├── .gitattributes
├── .gitignore
├── go.mod               module durparse
├── errors.go            ErrSyntax / ErrRange（已给）
├── durparse.go          类型与签名（方法体是 panic，待实现）
├── PROMPT.md            本题的 User Prompt（逐字）
├── check/main.go        固定验收程序（勿改）
├── scripts/check.sh     薄封装：go run ./check "$@"
└── README.md
```
