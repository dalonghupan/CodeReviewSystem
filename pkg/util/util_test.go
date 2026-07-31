package util

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAESRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef") // 32字节 AES-256
	plain := "ghp_test_token_敏感数据"

	encrypted, err := AESEncrypt(plain, key)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if encrypted == plain {
		t.Fatal("密文与明文相同")
	}
	decrypted, err := AESDecrypt(encrypted, key)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if decrypted != plain {
		t.Fatalf("解密结果不一致: got %q want %q", decrypted, plain)
	}
}

func TestAESWrongKey(t *testing.T) {
	key1 := []byte("0123456789abcdef0123456789abcdef")
	key2 := []byte("fedcba9876543210fedcba9876543210")

	encrypted, _ := AESEncrypt("secret", key1)
	if _, err := AESDecrypt(encrypted, key2); !errors.Is(err, ErrCiphertext) {
		t.Fatalf("错误密钥应返回 ErrCiphertext, got %v", err)
	}
}

func TestAESInvalidKey(t *testing.T) {
	if _, err := AESEncrypt("x", []byte("short")); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("短密钥应返回 ErrInvalidKey, got %v", err)
	}
}

func TestNormalizePage(t *testing.T) {
	cases := []struct{ in1, in2, wantP, wantS uint32 }{
		{0, 0, 1, DefaultPageSize},
		{2, 50, 2, 50},
		{1, 200, 1, MaxPageSize},
	}
	for _, c := range cases {
		p, s := NormalizePage(c.in1, c.in2)
		if p != c.wantP || s != c.wantS {
			t.Errorf("NormalizePage(%d,%d) = (%d,%d), want (%d,%d)", c.in1, c.in2, p, s, c.wantP, c.wantS)
		}
	}
}

func TestTotalPages(t *testing.T) {
	if got := TotalPages(0, 20); got != 0 {
		t.Errorf("TotalPages(0,20) = %d, want 0", got)
	}
	if got := TotalPages(21, 20); got != 2 {
		t.Errorf("TotalPages(21,20) = %d, want 2", got)
	}
}

func TestRetryWithBackoff(t *testing.T) {
	attempts := 0
	failErr := errors.New("fail")
	err := RetryWithBackoff(context.Background(), []time.Duration{time.Millisecond, time.Millisecond},
		func(ctx context.Context) error {
			attempts++
			return failErr
		})
	if !errors.Is(err, failErr) {
		t.Fatalf("应返回最后一次错误, got %v", err)
	}
	if attempts != 3 { // 首次 + 2次重试
		t.Fatalf("attempts = %d, want 3", attempts)
	}

	// 第二次成功
	attempts = 0
	err = RetryWithBackoff(context.Background(), []time.Duration{time.Millisecond, time.Millisecond},
		func(ctx context.Context) error {
			attempts++
			if attempts < 2 {
				return failErr
			}
			return nil
		})
	if err != nil || attempts != 2 {
		t.Fatalf("重试成功场景错误: err=%v attempts=%d", err, attempts)
	}
}

func TestParseDurationOr(t *testing.T) {
	if got := ParseDurationOr("5s", time.Second); got != 5*time.Second {
		t.Errorf("got %v", got)
	}
	if got := ParseDurationOr("bad", 3*time.Second); got != 3*time.Second {
		t.Errorf("非法值应返回默认值, got %v", got)
	}
	if got := ParseDurationOr("", 3*time.Second); got != 3*time.Second {
		t.Errorf("空串应返回默认值, got %v", got)
	}
}
