// Package errcode 定义全系统统一错误码
// 对应 LLD §7 异常处理与错误码详细设计
// 错误码分层：系统级(10xxx) / 业务级(20xxx) / 第三方(30xxx)
package errcode

import "fmt"

// Error 统一错误结构体
type Error struct {
	Code    int    `json:"code"`    // 错误码
	Message string `json:"message"` // 错误描述（面向开发）
	Detail  string `json:"detail"`  // 错误详情（面向用户提示）
}

func (e *Error) Error() string {
	return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Detail)
}

// New 创建错误实例
func New(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WithDetail 附加详情信息
func (e *Error) WithDetail(detail string) *Error {
	return &Error{Code: e.Code, Message: e.Message, Detail: detail}
}

// ============================================================
// 系统级错误码 10xxx（LLD §7.1 系统级错误）
// ============================================================
var (
	ErrInternal        = New(10001, "系统内部错误")
	ErrDatabase        = New(10002, "数据库操作异常")
	ErrRedis           = New(10003, "Redis操作异常")
	ErrMQProduce       = New(10004, "消息队列发送失败")
	ErrMQConsume       = New(10005, "消息队列消费失败")
	ErrServiceTimeout  = New(10006, "服务调用超时")
	ErrServiceUnavailable = New(10007, "服务不可用")
	ErrParamInvalid    = New(10008, "请求参数校验失败")
	ErrUnauthorized    = New(10009, "未授权，请先登录")
	ErrForbidden       = New(10010, "权限不足，拒绝访问")
	ErrNotFound        = New(10011, "资源不存在")
	ErrDuplicateRequest = New(10012, "重复请求，请勿重复提交")
	ErrRateLimit       = New(10013, "请求频率超限，请稍后重试")
	ErrTokenExpired    = New(10014, "登录令牌已过期，请重新登录")
	ErrFileTooLarge    = New(10015, "文件大小超出限制")
)

// ============================================================
// 业务级错误码 20xxx（LLD §7.1 业务错误）
// ============================================================
var (
	// 评审相关 201xx
	ErrReviewNotFound       = New(20101, "评审单不存在")
	ErrReviewStatusInvalid  = New(20102, "评审单当前状态不允许此操作")
	ErrReviewAlreadyExists  = New(20103, "该MR已存在进行中的评审单")
	ErrReviewDeadlinePassed = New(20104, "评审截止时间已过")
	ErrReviewNotReviewer    = New(20105, "您不是该评审单的评审人")
	ErrReviewNotCreator     = New(20106, "您不是该评审单的创建人")

	// 评论相关 202xx
	ErrCommentNotFound      = New(20201, "评论不存在")
	ErrCommentNoPermission  = New(20202, "无权操作此评论")
	ErrCommentAlreadyReplied = New(20203, "评论已有回复，不可编辑")

	// Sonar门禁 203xx
	ErrSonarGateBlocked     = New(20301, "Sonar质量门禁未通过，存在未修复的高危漏洞")
	ErrSonarDataUnavailable = New(20302, "Sonar扫描数据暂不可用")

	// 租户权限 204xx
	ErrTenantNotFound       = New(20401, "租户不存在或已禁用")
	ErrTenantQuotaExceeded  = New(20402, "租户资源配额已超限")
	ErrCrossTenantAccess    = New(20403, "跨租户访问被禁止")

	// 仓库相关 205xx
	ErrRepoBlacklisted      = New(20501, "该仓库在黑名单中，禁止创建评审")
	ErrRepoNotFound         = New(20502, "仓库未绑定或不存在")
	ErrRepoSensitiveFile    = New(20503, "该文件为敏感文件，内容已屏蔽")

	// 通知相关 206xx
	ErrNotificationDisabled = New(20601, "该通知渠道已被关闭")

	// 报表相关 207xx
	ErrReportNotFound       = New(20701, "报表不存在")
	ErrReportGenerating     = New(20702, "报表生成中，请稍后下载")
	ErrReportGenerateFailed = New(20703, "报表生成失败")

	// 缺陷相关 208xx
	ErrDefectNotFound       = New(20801, "缺陷记录不存在")
)

// ============================================================
// 第三方错误码 30xxx（LLD §7.1 第三方错误）
// ============================================================
var (
	// Git平台 301xx
	ErrGitAuthFailed       = New(30101, "Git平台授权失败")
	ErrGitTokenExpired     = New(30102, "Git授权Token已过期，请重新授权")
	ErrGitAPIError         = New(30103, "Git平台接口调用失败")
	ErrGitRateLimited      = New(30104, "Git平台接口调用频率超限")
	ErrGitMRNotFound       = New(30105, "Git平台MR不存在或已被删除")
	ErrGitDiffParseFailed  = New(30106, "代码Diff解析失败")

	// SonarQube 302xx
	ErrSonarConnectFailed  = New(30201, "SonarQube服务连接失败")
	ErrSonarScanNotFound   = New(30202, "SonarQube未找到对应扫描结果")

	// 企业微信 303xx
	ErrWechatPushFailed    = New(30301, "企业微信消息推送失败")

	// 邮件 304xx
	ErrEmailSendFailed     = New(30401, "邮件发送失败")
	ErrSMTPConnectFailed   = New(30402, "SMTP服务器连接失败")

	// Keycloak 305xx
	ErrKeycloakConnect     = New(30501, "Keycloak服务连接失败")
	ErrKeycloakSyncFailed  = New(30502, "Keycloak账号同步失败")

	// MinIO 306xx
	ErrMinIOUploadFailed   = New(30601, "文件上传至对象存储失败")
	ErrMinIODownloadFailed = New(30602, "文件下载失败")
)

// ============================================================
// HTTP状态码映射（gRPC错误码 → HTTP状态码）
// ============================================================
func ToHTTPStatus(code int) int {
	switch {
	case code == 10009:
		return 401 // Unauthorized
	case code == 10010 || code == 20403:
		return 403 // Forbidden
	case code == 10011 || code >= 20101 && code <= 20101 || code == 20201 || code == 20502 || code == 20701 || code == 20801:
		return 404 // Not Found
	case code == 10008:
		return 400 // Bad Request
	case code == 10012 || code == 20103:
		return 409 // Conflict
	case code == 10013:
		return 429 // Too Many Requests
	case code >= 30000:
		return 502 // Bad Gateway（第三方服务异常）
	default:
		return 500 // Internal Server Error
	}
}
