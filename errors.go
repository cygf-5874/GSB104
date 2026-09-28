package durparse

import "errors"

// 本包所有失败路径都返回下面两个哨兵错误之一。调用方用 errors.Is 判定，
// 不要比较错误字符串。
var (
	// ErrSyntax 表示字符串不符合时长语法。
	ErrSyntax = errors.New("durparse: invalid duration syntax")

	// ErrRange 表示结果超出了 int64 纳秒的可表示范围。
	ErrRange = errors.New("durparse: duration out of range")
)
