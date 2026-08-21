// Package util 工具函数库
package util

import (
	"os"
	"regexp"
)

var envPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// ExpandEnv 展开字符串中的 ${ENV_VAR} 占位符
// 与 os.ExpandEnv 不同，本函数在变量为空时保留原始占位符（避免误替换为空字符串）
// 如果 strict 为 true，变量为空时返回空字符串（与 os.ExpandEnv 一致）
func ExpandEnv(s string, strict ...bool) string {
	return envPattern.ReplaceAllStringFunc(s, func(match string) string {
		key := match[2 : len(match)-1] // 去掉 ${ 和 }
		val := os.Getenv(key)
		if val == "" && (len(strict) == 0 || !strict[0]) {
			return match // 保留原始占位符
		}
		return val
	})
}
