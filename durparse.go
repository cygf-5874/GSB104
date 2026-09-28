// Package durparse 提供时长的解析与格式化。
//
//   - Parse 把 "1h30m"、"2m0.5s" 这样的字符串解析成**纳秒整数**；
//   - Format 把纳秒整数格式化成**最短**的时长字符串。
//
// 语义逐条写在 README 的「对外契约」一节里。
//
// 本文件目前只有类型与签名，方法体全部是 panic，需要按契约实现。
package durparse

// Parse 把时长字符串 s 解析成纳秒整数。
//
// 具体语义（单位集合、多段、符号、小数、边界与溢出）见 README「对外契约」第 1~6 条。
func Parse(s string) (int64, error) {
	panic("not implemented")
}

// Format 把纳秒整数 ns 格式化成最短的时长字符串。
//
// 具体语义（最短形式、负值、往返性质、顺序无关）见 README「对外契约」第 7~10 条。
func Format(ns int64) string {
	panic("not implemented")
}
