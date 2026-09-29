// Package durparse 提供时长的解析与格式化。
//
//   - Parse 把 "1h30m"、"2m0.5s" 这样的字符串解析成**纳秒整数**；
//   - Format 把纳秒整数格式化成**最短**的时长字符串。
//
// 语义逐条写在 README 的「对外契约」一节里。
//
// 本文件目前只有类型与签名，方法体全部是 panic，需要按契约实现。
package durparse

import (
	"math/big"
	"strconv"
	"strings"
)

type durationUnit struct {
	name  string
	scale int64
}

type unitAlias struct {
	alias string
	unit  durationUnit
}

var durationUnits = []durationUnit{
	{name: "h", scale: int64(60 * 60 * 1000 * 1000 * 1000)},
	{name: "m", scale: int64(60 * 1000 * 1000 * 1000)},
	{name: "s", scale: 1000 * 1000 * 1000},
	{name: "ms", scale: 1000 * 1000},
	{name: "us", scale: 1000},
	{name: "ns", scale: 1},
}

var unitAliases = []unitAlias{
	{alias: "h", unit: durationUnits[0]},
	{alias: "m", unit: durationUnits[1]},
	{alias: "s", unit: durationUnits[2]},
	{alias: "ms", unit: durationUnits[3]},
	{alias: "us", unit: durationUnits[4]},
	{alias: "µs", unit: durationUnits[4]},
	{alias: "μs", unit: durationUnits[4]},
	{alias: "ns", unit: durationUnits[5]},
}

type durationPart struct {
	integer  string
	fraction string
	unit     durationUnit
}

// Parse 把时长字符串 s 解析成纳秒整数。
//
// 具体语义（单位集合、多段、符号、小数、边界与溢出）见 README「对外契约」第 1~6 条。
func Parse(s string) (int64, error) {
	negative := false
	switch {
	case strings.HasPrefix(s, "-"):
		negative = true
		s = s[1:]
	case strings.HasPrefix(s, "+"):
		s = s[1:]
	}

	if s == "0" {
		return 0, nil
	}
	if s == "" {
		return 0, ErrSyntax
	}

	parts, err := parseDurationParts(s)
	if err != nil {
		return 0, err
	}

	limit := new(big.Int).SetUint64(1<<63 - 1)
	if negative {
		limit.SetUint64(1 << 63)
	}

	total := new(big.Int)
	for _, part := range parts {
		value := durationPartValue(part)
		if value.Cmp(limit) > 0 {
			return 0, ErrRange
		}
		total.Add(total, value)
		if total.Cmp(limit) > 0 {
			return 0, ErrRange
		}
	}

	result := total.Uint64()
	if negative {
		if result == uint64(1)<<63 {
			return -1 << 63, nil
		}
		return -int64(result), nil
	}
	return int64(result), nil
}

// Format 把纳秒整数 ns 格式化成最短的时长字符串。
//
// 具体语义（最短形式、负值、往返性质、顺序无关）见 README「对外契约」第 7~10 条。
func Format(ns int64) string {
	if ns == 0 {
		return "0s"
	}

	var builder strings.Builder
	remaining := uint64(ns)
	if ns < 0 {
		builder.WriteByte('-')
		if ns == -1<<63 {
			remaining = uint64(1) << 63
		} else {
			remaining = uint64(-ns)
		}
	}

	for _, unit := range durationUnits {
		scale := uint64(unit.scale)
		value := remaining / scale
		remaining %= scale
		if value == 0 {
			continue
		}
		builder.WriteString(strconv.FormatUint(value, 10))
		builder.WriteString(unit.name)
	}

	return builder.String()
}

func parseDurationParts(s string) ([]durationPart, error) {
	parts := make([]durationPart, 0, len(durationUnits))
	previousUnit := -1
	decimalAllowed := true

	for s != "" {
		part, next, hasDecimal, err := parseDurationPart(s)
		if err != nil {
			return nil, err
		}
		if hasDecimal && !decimalAllowed {
			return nil, ErrSyntax
		}

		currentUnit := unitRank(part.unit)
		if currentUnit <= previousUnit {
			return nil, ErrSyntax
		}

		parts = append(parts, part)
		previousUnit = currentUnit
		if hasDecimal {
			decimalAllowed = false
		}
		s = next
	}

	return parts, nil
}

func parseDurationPart(s string) (durationPart, string, bool, error) {
	start := 0
	for start < len(s) && s[start] >= '0' && s[start] <= '9' {
		start++
	}

	integerDigits := s[:start]
	hasDecimal := false
	fractionDigits := ""
	if start < len(s) && s[start] == '.' {
		hasDecimal = true
		start++
		fractionStart := start
		for start < len(s) && s[start] >= '0' && s[start] <= '9' {
			start++
		}
		fractionDigits = s[fractionStart:start]
	}

	if integerDigits == "" && fractionDigits == "" {
		return durationPart{}, "", false, ErrSyntax
	}

	unit, unitLength, ok := matchUnit(s[start:])
	if !ok {
		return durationPart{}, "", false, ErrSyntax
	}

	next := start + unitLength
	if next < len(s) {
		nextChar := s[next]
		if nextChar != '.' && (nextChar < '0' || nextChar > '9') {
			return durationPart{}, "", false, ErrSyntax
		}
		if hasDecimal {
			return durationPart{}, "", false, ErrSyntax
		}
	}

	return durationPart{
		integer:  integerDigits,
		fraction: fractionDigits,
		unit:     unit,
	}, s[next:], hasDecimal, nil
}

func matchUnit(s string) (durationUnit, int, bool) {
	unit := durationUnit{}
	length := 0
	for _, candidate := range unitAliases {
		if strings.HasPrefix(s, candidate.alias) && len(candidate.alias) > length {
			unit = candidate.unit
			length = len(candidate.alias)
		}
	}
	return unit, length, length > 0
}

func unitRank(want durationUnit) int {
	for i, unit := range durationUnits {
		if unit.name == want.name {
			return i
		}
	}
	return -1
}

func durationPartValue(part durationPart) *big.Int {
	value, _ := new(big.Int).SetString(part.integer, 10)
	if value == nil {
		value = new(big.Int)
	}
	value.Mul(value, big.NewInt(part.unit.scale))

	if part.fraction != "" {
		fraction, _ := new(big.Int).SetString(part.fraction, 10)
		if fraction != nil {
			denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(part.fraction))), nil)
			fraction.Mul(fraction, big.NewInt(part.unit.scale))
			fraction.Quo(fraction, denominator)
			value.Add(value, fraction)
		}
	}

	return value
}
