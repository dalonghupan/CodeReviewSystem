package util

import (
	"github.com/google/uuid"
)

// NewUUID 生成业务主键 UUID（LLD §5.1 review_id 主键UUID）
func NewUUID() string {
	return uuid.NewString()
}

// NewMessageID 生成 MQ 消息唯一ID（幂等校验用）
func NewMessageID() string {
	return uuid.NewString()
}

// IsValidUUID 校验字符串是否为合法 UUID
func IsValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
