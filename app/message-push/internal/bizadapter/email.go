package bizadapter

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"

	"cr-system/pkg/errcode"
)

// EmailClient SMTP 邮件客户端
// 支持 StartTLS（端口 587）和 SSL（端口 465）
type EmailClient struct {
	timeout time.Duration
}

// EmailConfig 邮件发送配置
type EmailConfig struct {
	SMTPHost string
	SMTPPort int
	Username string
	Password string // SMTP 授权码
}

// EmailMessage 邮件消息
type EmailMessage struct {
	From    string
	To      []string
	Subject string
	Body    string   // HTML 正文
	CC      []string // 抄送
}

// NewEmailClient 创建邮件客户端
func NewEmailClient(timeout time.Duration) *EmailClient {
	return &EmailClient{timeout: timeout}
}

// SendEmail 发送邮件
func (c *EmailClient) SendEmail(ctx context.Context, cfg EmailConfig, msg EmailMessage) error {
	if cfg.SMTPHost == "" || cfg.SMTPPort == 0 || cfg.Username == "" {
		return errcode.ErrSMTPConnectFailed.WithDetail("SMTP 配置不完整")
	}

	from := msg.From
	if from == "" {
		from = cfg.Username
	}

	// 构建邮件头
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = joinAddresses(msg.To)
	headers["Subject"] = encodeSubject(msg.Subject)
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"
	if len(msg.CC) > 0 {
		headers["Cc"] = joinAddresses(msg.CC)
	}

	var header string
	for k, v := range headers {
		header += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	header += "\r\n"

	// 合并收件人列表（To + CC）
	recipients := append(msg.To, msg.CC...)
	if len(recipients) == 0 {
		return errcode.ErrParamInvalid.WithDetail("收件人列表为空")
	}

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	// 根据端口选择连接方式
	if cfg.SMTPPort == 465 {
		return c.sendSSL(ctx, addr, cfg, recipients, header+msg.Body)
	}
	return c.sendStartTLS(ctx, addr, cfg, recipients, header+msg.Body)
}

// sendStartTLS 端口 587：STARTTLS
func (c *EmailClient) sendStartTLS(ctx context.Context, addr string, cfg EmailConfig, recipients []string, body string) error {
	conn, err := net.DialTimeout("tcp", addr, c.timeout)
	if err != nil {
		return errcode.ErrSMTPConnectFailed.WithDetail("连接 SMTP 服务器失败: " + err.Error())
	}

	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return errcode.ErrSMTPConnectFailed.WithDetail("创建 SMTP 客户端失败: " + err.Error())
	}
	defer client.Close()

	if err := client.StartTLS(&tls.Config{ServerName: cfg.SMTPHost}); err != nil {
		return errcode.ErrSMTPConnectFailed.WithDetail("STARTTLS 失败: " + err.Error())
	}

	return c.authAndSend(client, cfg, recipients, body)
}

// sendSSL 端口 465：SSL/TLS 直连
func (c *EmailClient) sendSSL(ctx context.Context, addr string, cfg EmailConfig, recipients []string, body string) error {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: c.timeout}, "tcp", addr, &tls.Config{ServerName: cfg.SMTPHost})
	if err != nil {
		return errcode.ErrSMTPConnectFailed.WithDetail("SSL 连接 SMTP 失败: " + err.Error())
	}

	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return errcode.ErrSMTPConnectFailed.WithDetail("创建 SSL SMTP 客户端失败: " + err.Error())
	}
	defer client.Close()

	return c.authAndSend(client, cfg, recipients, body)
}

// authAndSend 认证并发送
func (c *EmailClient) authAndSend(client *smtp.Client, cfg EmailConfig, recipients []string, body string) error {
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	if err := client.Auth(auth); err != nil {
		return errcode.ErrSMTPConnectFailed.WithDetail("SMTP 认证失败: " + err.Error())
	}

	// 发件人
	if err := client.Mail(cfg.Username); err != nil {
		return errcode.ErrEmailSendFailed.WithDetail("设置发件人失败: " + err.Error())
	}

	// 收件人
	for _, rec := range recipients {
		if err := client.Rcpt(rec); err != nil {
			return errcode.ErrEmailSendFailed.WithDetail("添加收件人失败: " + err.Error())
		}
	}

	// 发送内容
	w, err := client.Data()
	if err != nil {
		return errcode.ErrEmailSendFailed.WithDetail("打开数据连接失败: " + err.Error())
	}
	if _, err := fmt.Fprint(w, body); err != nil {
		return errcode.ErrEmailSendFailed.WithDetail("写入邮件内容失败: " + err.Error())
	}
	if err := w.Close(); err != nil {
		return errcode.ErrEmailSendFailed.WithDetail("关闭数据连接失败: " + err.Error())
	}

	return nil
}

// joinAddresses 合并邮件地址列表为字符串
func joinAddresses(addrs []string) string {
	result := ""
	for i, a := range addrs {
		if i > 0 {
			result += ", "
		}
		result += a
	}
	return result
}

// encodeSubject 编码邮件主题（防止中文乱码）
func encodeSubject(subject string) string {
	return fmt.Sprintf("=?UTF-8?B?%s?=", base64Encode([]byte(subject)))
}

// base64Encode 简易 Base64 编码（避免引入 encoding/base64 包名冲突）
func base64Encode(data []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="
	result := make([]byte, 0, ((len(data)+2)/3)*4)
	for i := 0; i < len(data); i += 3 {
		b0 := data[i]
		b1 := byte(0)
		b2 := byte(0)
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}
		result = append(result, alphabet[b0>>2])
		result = append(result, alphabet[((b0&0x03)<<4)|(b1>>4)])
		if i+1 < len(data) {
			result = append(result, alphabet[((b1&0x0f)<<2)|(b2>>6)])
		} else {
			result = append(result, alphabet[64])
		}
		if i+2 < len(data) {
			result = append(result, alphabet[b2&0x3f])
		} else {
			result = append(result, alphabet[64])
		}
	}
	return string(result)
}
