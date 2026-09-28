// Command check 是 durparse 的固定验收程序。
//
// ⚠️ 不要修改本文件。它是判定「题目有没有做对」的依据；
// 修改它只会让判定失效，不会让实现变对。
//
// 10 个场景分四组：
//
//	parse     4：多段与单位严格递减 / 符号作用于整串 / 小数只在最后一段 / 空串与只有单位
//	range     2：超出 int64 报 ErrRange / 中间累加防溢出
//	format    2：最短形式（省略零单位）/ 负值格式
//	roundtrip 2：确定序列往返 / MinInt64 与 MaxInt64 往返
//
// 起点是空壳（方法体全是 panic）时，每个场景都会 panic；固定件必须把 10 个
// 场景全部跑完再收尾。判据全部是硬编码的期望值，不依赖墙钟、随机源或 map 顺序。
//
// 用法：
//
//	go run ./check                 # 跑全部场景
//	go run ./check --only parse    # 只跑一组
//	go run ./check -list           # 列出场景
package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"durparse"
)

type scenario struct {
	group string
	name  string
	run   func() error
}

var scenarios = []scenario{
	{"parse", "P1_multi_segments_decreasing", p1},
	{"parse", "P2_sign_applies_to_whole", p2},
	{"parse", "P3_decimal_only_last_segment", p3},
	{"parse", "P4_empty_sign_and_unit_only", p4},
	{"range", "R1_beyond_int64", r1},
	{"range", "R2_intermediate_accumulation", r2},
	{"format", "F1_shortest_omit_zero_units", f1},
	{"format", "F2_negative_format", f2},
	{"roundtrip", "RT1_sequence_roundtrip", rt1},
	{"roundtrip", "RT2_min_max_int64", rt2},
}

var groups = []string{"parse", "range", "format", "roundtrip"}

const (
	nsUs = int64(time.Microsecond / time.Nanosecond) // 1000
	nsMs = int64(time.Millisecond / time.Nanosecond) // 1e6
	nsS  = int64(time.Second / time.Nanosecond)      // 1e9
	nsM  = int64(time.Minute / time.Nanosecond)      // 6e10
	nsH  = int64(time.Hour / time.Nanosecond)        // 3.6e12
)

func main() {
	only := ""
	list := false
	for i := 1; i < len(os.Args); i++ {
		a := os.Args[i]
		switch {
		case a == "-list":
			list = true
		case a == "--only":
			if i+1 < len(os.Args) {
				only = os.Args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--only="):
			only = strings.TrimPrefix(a, "--only=")
		}
	}

	if list {
		for _, sc := range scenarios {
			fmt.Printf("%-10s %s\n", sc.group, sc.name)
		}
		return
	}

	if only != "" {
		found := false
		for _, g := range groups {
			if g == only {
				found = true
			}
		}
		if !found {
			fmt.Printf("未知分组 %q（可选：parse / range / format / roundtrip）\n", only)
			os.Exit(2)
		}
	}

	passed, total := 0, 0
	for _, sc := range scenarios {
		if only != "" && sc.group != only {
			continue
		}
		total++
		if err := runScenario(sc); err != nil {
			fmt.Printf("FAIL %s/%s  %s\n", sc.group, sc.name, err.Error())
			continue
		}
		passed++
		fmt.Printf("PASS %s/%s\n", sc.group, sc.name)
	}
	fmt.Printf("结果：通过 %d/%d\n", passed, total)
	if passed != total {
		os.Exit(1)
	}
}

func runScenario(sc scenario) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("场景正常结束（不 panic） 实际=panic: " + fmt.Sprint(r))
		}
	}()
	return sc.run()
}

// ------------------------------------------------------------------ 断言

// detailError 的 Error() 恰好是「期望=… 实际=…」。
type detailError struct{ msg string }

func (e *detailError) Error() string { return e.msg }

func expect(want, got string) error {
	return &detailError{msg: "期望=" + want + " 实际=" + got}
}

func errStr(err error) string {
	if err == nil {
		return "nil（没有返回错误）"
	}
	return err.Error()
}

// wantInt 断言 Parse(s) 得到纳秒整数 want。
func wantInt(what, s string, want int64) error {
	got, err := durparse.Parse(s)
	if err != nil {
		return expect(fmt.Sprintf("%s=%d", what, want), "返回错误 "+errStr(err))
	}
	if got != want {
		return expect(fmt.Sprintf("%s=%d", what, want), strconv.FormatInt(got, 10))
	}
	return nil
}

// wantErr 断言 Parse(s) 返回指定的哨兵错误。
func wantErr(what, s string, want error) error {
	_, err := durparse.Parse(s)
	if !errors.Is(err, want) {
		return expect(what+" 返回 "+want.Error(), "得到 "+errStr(err))
	}
	return nil
}

// wantInt64 断言某个 int64 值 n 的 Format 结果是 want。
func wantFormat(what string, n int64, want string) error {
	got := durparse.Format(n)
	if got != want {
		return expect(what+"="+strconv.Quote(want), strconv.Quote(got))
	}
	return nil
}

// ------------------------------------------------------------------ parse

func p1() error {
	// 多段拼接，单位严格递减。
	cases := []struct {
		s    string
		want int64
	}{
		{"1h30m", nsH + 30*nsM},
		{"2m0.5s", 2*nsM + nsS/2},
		{"1h2m3s", nsH + 2*nsM + 3*nsS},
		{"1h2m3s4ms5us6ns", nsH + 2*nsM + 3*nsS + 4*nsMs + 5*nsUs + 6},
		{"1s500ms", nsS + 500*nsMs},
		{"1h", nsH},
		{"5ns", 5},
	}
	for _, c := range cases {
		if err := wantInt("Parse("+strconv.Quote(c.s)+")", c.s, c.want); err != nil {
			return err
		}
	}
	// 单位必须严格递减：相等或递增都是 ErrSyntax。
	for _, s := range []string{"1h1h", "30m1h", "1s1s", "1m1h", "1us1ns2ns"} {
		if err := wantErr("Parse("+strconv.Quote(s)+")", s, durparse.ErrSyntax); err != nil {
			return err
		}
	}
	return nil
}

func p2() error {
	cases := []struct {
		s    string
		want int64
	}{
		{"-1h30m", -(nsH + 30*nsM)},
		{"+1h30m", nsH + 30*nsM},
		{"-2m0.5s", -(2*nsM + nsS/2)},
		{"-1h", -nsH},
		{"-0s", 0},
		{"-0", 0},
		{"+0", 0},
	}
	for _, c := range cases {
		if err := wantInt("Parse("+strconv.Quote(c.s)+")", c.s, c.want); err != nil {
			return err
		}
	}
	// 符号作用于整串：-1h30m == -(1h + 30m)。
	whole, err := durparse.Parse("-1h30m")
	if err != nil {
		return expect("Parse(\"-1h30m\") 成功", errStr(err))
	}
	partA, _ := durparse.Parse("-1h")
	partB, _ := durparse.Parse("-30m")
	if whole != partA+partB {
		return expect(fmt.Sprintf("-1h30m = %d + %d = %d", partA, partB, partA+partB), strconv.FormatInt(whole, 10))
	}
	return nil
}

func p3() error {
	// 小数只允许出现在最后一段。
	ok := []struct {
		s    string
		want int64
	}{
		{"1.5h", nsH + nsH/2},
		{"2m0.5s", 2*nsM + nsS/2},
		{".5s", nsS / 2},
		{"0.5h", nsH / 2},
		{"1.25s", nsS + nsS/4},
	}
	for _, c := range ok {
		if err := wantInt("Parse("+strconv.Quote(c.s)+")", c.s, c.want); err != nil {
			return err
		}
	}
	bad := []string{"1.5h30m", "1.5h1.5m", "1.5h1m", "2m0.5s1ms"}
	for _, s := range bad {
		if err := wantErr("Parse("+strconv.Quote(s)+")", s, durparse.ErrSyntax); err != nil {
			return err
		}
	}
	// 小数截断到 ns（向下取整的绝对值）。
	if err := wantInt("Parse(\"1.0000000009s\")", "1.0000000009s", nsS); err != nil {
		return err
	}
	if err := wantInt("Parse(\"0.0000000009s\")", "0.0000000009s", 0); err != nil {
		return err
	}
	return nil
}

func p4() error {
	// 空串、只有符号、只有单位：都是 ErrSyntax。
	for _, s := range []string{"", "-", "+", "s", "ns", "h", "ms", "-s", "+ns", "1h30", "1", "h30m"} {
		if err := wantErr("Parse("+strconv.Quote(s)+")", s, durparse.ErrSyntax); err != nil {
			return err
		}
	}
	// "0" 无单位合法；"0s"、"00h" 合法。
	for _, c := range []struct {
		s    string
		want int64
	}{{"0", 0}, {"0s", 0}, {"00h", 0}, {"000ns", 0}} {
		if err := wantInt("Parse("+strconv.Quote(c.s)+")", c.s, c.want); err != nil {
			return err
		}
	}
	return nil
}

// ------------------------------------------------------------------ range

func r1() error {
	if err := wantInt("Parse(\"9223372036854775807ns\")", "9223372036854775807ns", math.MaxInt64); err != nil {
		return err
	}
	if err := wantInt("Parse(\"-9223372036854775808ns\")", "-9223372036854775808ns", math.MinInt64); err != nil {
		return err
	}
	for _, s := range []string{
		"9223372036854775808ns",  // MaxInt64 + 1
		"-9223372036854775809ns", // MinInt64 - 1
		"2562048h",               // 远超 int64
		"9223372036854775807s",
	} {
		if err := wantErr("Parse("+strconv.Quote(s)+")", s, durparse.ErrRange); err != nil {
			return err
		}
	}
	return nil
}

func r2() error {
	// 各段单独看都不越界，但累加起来才是边界值。朴素实现若先溢出再判会漏掉。
	const (
		maxStr = "2562047h47m16s854ms775us807ns" // == MaxInt64
		minStr = "2562047h47m16s854ms775us808ns" // == 2^63
	)
	if err := wantInt("Parse("+strconv.Quote(maxStr)+")", maxStr, math.MaxInt64); err != nil {
		return err
	}
	if err := wantErr("Parse("+strconv.Quote(minStr)+")", minStr, durparse.ErrRange); err != nil {
		return err
	}
	// 同一个字符串加负号后恰好等于 MinInt64。
	if err := wantInt("Parse(\"-\"+minStr)", "-"+minStr, math.MinInt64); err != nil {
		return err
	}
	// 每一段单独看都在范围内，累加起来才越界：2562047h + 59m = 9223372740000000000 > MaxInt64。
	if err := wantErr("Parse(\"2562047h59m\")", "2562047h59m", durparse.ErrRange); err != nil {
		return err
	}
	return nil
}

// ----------------------------------------------------------------- format

func f1() error {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0s"},
		{nsH, "1h"},
		{nsM, "1m"},
		{nsS, "1s"},
		{nsMs, "1ms"},
		{nsUs, "1us"},
		{1, "1ns"},
		{1500, "1us500ns"},
		{90 * nsS, "1m30s"},
		{nsH + 30*nsM, "1h30m"},
		{nsH + 1, "1h1ns"},
		{nsH + 2*nsM + 3*nsS + 4*nsMs + 5*nsUs + 6, "1h2m3s4ms5us6ns"},
		{nsS + 500*nsMs, "1s500ms"},
	}
	for _, c := range cases {
		if err := wantFormat("Format("+strconv.FormatInt(c.n, 10)+")", c.n, c.want); err != nil {
			return err
		}
	}
	return nil
}

func f2() error {
	cases := []struct {
		n    int64
		want string
	}{
		{-1, "-1ns"},
		{-(90 * nsS), "-1m30s"},
		{-(nsH + 30*nsM), "-1h30m"},
		{-nsH, "-1h"},
		{math.MinInt64, "-2562047h47m16s854ms775us808ns"},
		{math.MaxInt64, "2562047h47m16s854ms775us807ns"},
	}
	for _, c := range cases {
		if err := wantFormat("Format("+strconv.FormatInt(c.n, 10)+")", c.n, c.want); err != nil {
			return err
		}
	}
	return nil
}

// -------------------------------------------------------------- roundtrip

// lcg 是确定性的伪随机序列（不依赖 math/rand 的版本或种子语义）。
func lcgNext(x *uint64) int64 {
	*x = *x*6364136223846793005 + 1442695040888963407
	return int64(*x)
}

func rt1() error {
	seed := uint64(0x9E3779B97F4A7C15)
	for i := 0; i < 20000; i++ {
		v := lcgNext(&seed)
		s := durparse.Format(v)
		got, err := durparse.Parse(s)
		if err != nil {
			return expect(fmt.Sprintf("Parse(Format(%d)) 成功（Format=%q）", v, s), "返回错误 "+errStr(err))
		}
		if got != v {
			return expect(fmt.Sprintf("Parse(Format(%d))==%d（Format=%q）", v, v, s), strconv.FormatInt(got, 10))
		}
	}
	// 边界附近的确定性取值。
	for _, v := range []int64{0, 1, -1, nsUs - 1, nsUs, nsMs, nsS, nsH, math.MaxInt64, math.MinInt64,
		math.MaxInt64 - 1, math.MinInt64 + 1} {
		s := durparse.Format(v)
		got, err := durparse.Parse(s)
		if err != nil {
			return expect(fmt.Sprintf("Parse(Format(%d)) 成功（Format=%q）", v, s), "返回错误 "+errStr(err))
		}
		if got != v {
			return expect(fmt.Sprintf("Parse(Format(%d))==%d（Format=%q）", v, v, s), strconv.FormatInt(got, 10))
		}
	}
	return nil
}

func rt2() error {
	for _, v := range []int64{math.MinInt64, math.MaxInt64} {
		s := durparse.Format(v)
		if strings.Contains(s, ".") {
			return expect(fmt.Sprintf("Format(%d) 不含小数（以便精确往返）", v), strconv.Quote(s))
		}
		got, err := durparse.Parse(s)
		if err != nil {
			return expect(fmt.Sprintf("Parse(Format(%d)) 成功（Format=%q）", v, s), "返回错误 "+errStr(err))
		}
		if got != v {
			return expect(fmt.Sprintf("Parse(Format(%d))==%d（Format=%q）", v, v, s), strconv.FormatInt(got, 10))
		}
	}
	return nil
}
