// Package durparse 提供时长的解析与格式化。
//
//   - Parse 把 "1h30m"、"2m0.5s" 这样的字符串解析成**纳秒整数**；
//   - Format 把纳秒整数格式化成**最短**的时长字符串。
//
// 语义逐条写在 README 的「对外契约」一节里。
package durparse

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// unit 描述一个时长单位。scale 是该单位对应的纳秒数。
type unit struct {
	name  string
	scale int64
}

// formatUnits 按固定顺序（从大到小）排列，供 Format 拼接与 Parse 判递减使用。
// 用切片而不是 map，保证遍历顺序与运行无关。
var formatUnits = []unit{
	{"h", 3_600_000_000_000},
	{"m", 60_000_000_000},
	{"s", 1_000_000_000},
	{"ms", 1_000_000},
	{"us", 1_000},
	{"ns", 1},
}

// unitScale 按名字精确匹配单位；µs（U+00B5）与 μs（U+03BC）都算微秒。
func unitScale(name string) (int64, bool) {
	switch name {
	case "ns":
		return 1, true
	case "us", "µs", "μs":
		return 1_000, true
	case "ms":
		return 1_000_000, true
	case "s":
		return 1_000_000_000, true
	case "m":
		return 60_000_000_000, true
	case "h":
		return 3_600_000_000_000, true
	}
	return 0, false
}

// Parse 把时长字符串 s 解析成纳秒整数。
//
// 具体语义（单位集合、多段、符号、小数、边界与溢出）见 README「对外契约」第 1~6 条。
func Parse(s string) (int64, error) {
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	// 唯一允许的无单位形式是 "0"。
	if s == "0" {
		return 0, nil
	}
	if s == "" {
		return 0, ErrSyntax
	}

	// limit 是允许累加到的最大幅值：负数可以到 2^63（MinInt64），正数到 MaxInt64。
	limit := uint64(math.MaxInt64)
	if neg {
		limit = 1 << 63
	}

	var total uint64
	prevScale := int64(math.MaxInt64) // 上一段的单位尺度，必须严格递减

	for s != "" {
		// 整数部分
		i := 0
		var intPart uint64
		hasInt := false
		for i < len(s) && '0' <= s[i] && s[i] <= '9' {
			d := uint64(s[i] - '0')
			if intPart > (limit+1)/10+1 { // 提前防 uint64 溢出；超过 limit 必然 ErrRange
				return 0, ErrRange
			}
			intPart = intPart*10 + d
			hasInt = true
			i++
		}
		// 小数部分（只记录位置，等拿到单位尺度后再精确截断）
		fracStart, fracEnd := -1, -1
		if i < len(s) && s[i] == '.' {
			i++
			fracStart = i
			for i < len(s) && '0' <= s[i] && s[i] <= '9' {
				i++
			}
			fracEnd = i
		}
		if !hasInt && fracStart < 0 {
			return 0, ErrSyntax // 既没有整数位也没有小数位
		}
		if fracStart >= 0 && fracStart == fracEnd && !hasInt {
			return 0, ErrSyntax // 只有一个孤零零的小数点
		}

		// 单位：一直到下一个数字或小数点为止
		j := i
		for j < len(s) && s[j] != '.' && !('0' <= s[j] && s[j] <= '9') {
			j++
		}
		scale, ok := unitScale(s[i:j])
		if !ok {
			return 0, ErrSyntax
		}
		if scale >= prevScale {
			return 0, ErrSyntax // 单位必须严格递减
		}
		prevScale = scale

		// 小数只允许出现在最后一段。
		if fracStart >= 0 && j < len(s) {
			return 0, ErrSyntax
		}

		// 本段纳秒数 = intPart*scale + floor(frac*scale)。
		us := uint64(scale)
		if intPart > limit/us {
			return 0, ErrRange
		}
		seg := intPart * us
		if fracStart >= 0 && fracEnd > fracStart {
			seg += fracNanos(s[fracStart:fracEnd], us)
		}
		if seg > limit || total > limit-seg {
			return 0, ErrRange
		}
		total += seg
		s = s[j:]
	}

	if neg {
		if total == 1<<63 {
			return math.MinInt64, nil
		}
		return -int64(total), nil
	}
	return int64(total), nil
}

// fracNanos 计算 floor(0.digits * scale)，对绝对值向下截断到纳秒。
// 用 big.Int 做精确运算，避免非 10 幂尺度（如 m、h）逐位截断丢精度。
func fracNanos(digits string, scale uint64) uint64 {
	num := new(big.Int)
	if _, ok := num.SetString(digits, 10); !ok {
		return 0
	}
	num.Mul(num, new(big.Int).SetUint64(scale))
	den := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(digits))), nil)
	return num.Quo(num, den).Uint64()
}

// Format 把纳秒整数 ns 格式化成最短的时长字符串。
//
// 具体语义（最短形式、负值、往返性质、顺序无关）见 README「对外契约」第 7~10 条。
func Format(ns int64) string {
	if ns == 0 {
		return "0s"
	}
	// 用无符号幅值，MinInt64 也能正确处理。
	u := uint64(ns)
	neg := ns < 0
	if neg {
		u = uint64(-(ns + 1)) + 1
	}
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for _, un := range formatUnits {
		scale := uint64(un.scale)
		if u >= scale {
			b.WriteString(strconv.FormatUint(u/scale, 10))
			b.WriteString(un.name)
			u %= scale
		}
	}
	return b.String()
}
