// Package util 全局通用工具函数
// 对应 LLD §9-4：用户密码、Token全部AES加密存储，数据库不可解密查看原文
package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// ErrInvalidKey 密钥长度非法（必须为16/24/32字节，对应AES-128/192/256）
var ErrInvalidKey = errors.New("AES密钥长度必须为16/24/32字节")

// ErrCiphertext 密文非法或已被篡改
var ErrCiphertext = errors.New("密文非法或已篡改")

// AESEncrypt AES-GCM 加密，输出 base64 编码字符串（nonce前置拼接）
// key 必须从 K8s Secret 注入，禁止硬编码（LLD §9-1）
func AESEncrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrInvalidKey
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// AESDecrypt AES-GCM 解密，输入 base64 编码字符串
func AESDecrypt(encoded string, key []byte) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrCiphertext
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrInvalidKey
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", ErrCiphertext
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrCiphertext
	}
	return string(plaintext), nil
}
