package util

import "time"

// ParseDurationOr 解析时长字符串，空串或解析失败时返回默认值
// 配置文件中时长统一使用字符串（如 "5s"、"30m"），避免 yaml 解析歧义
func ParseDurationOr(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}
