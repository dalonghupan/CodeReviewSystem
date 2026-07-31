package errcode

import "testing"

func TestErrorFormat(t *testing.T) {
	e := ErrReviewNotFound.WithDetail("id=abc")
	want := "[20101] 评审单不存在: id=abc"
	if e.Error() != want {
		t.Errorf("got %q, want %q", e.Error(), want)
	}
}

func TestWithDetailImmutability(t *testing.T) {
	base := ErrReviewNotFound
	derived := base.WithDetail("x")
	if base.Detail != "" {
		t.Error("WithDetail 不应修改原错误实例")
	}
	if derived.Detail != "x" {
		t.Error("WithDetail 应设置 Detail")
	}
	if derived.Code != base.Code {
		t.Error("WithDetail 应保留错误码")
	}
}

func TestToHTTPStatus(t *testing.T) {
	cases := []struct {
		code int
		want int
	}{
		{10009, 401},  // 未授权
		{10010, 403},  // 权限不足
		{20403, 403},  // 跨租户
		{10008, 400},  // 参数错误
		{10012, 409},  // 重复请求
		{20103, 409},  // 评审单已存在
		{10013, 429},  // 限流
		{20101, 404},  // 评审单不存在
		{20502, 404},  // 仓库不存在
		{30101, 502},  // 第三方错误
		{10001, 500},  // 系统内部
	}
	for _, c := range cases {
		if got := ToHTTPStatus(c.code); got != c.want {
			t.Errorf("ToHTTPStatus(%d) = %d, want %d", c.code, got, c.want)
		}
	}
}

func TestCodeRanges(t *testing.T) {
	// 错误码分层约束（LLD §7.1）：系统级10xxx / 业务级20xxx / 第三方30xxx
	sysCodes := []int{ErrInternal.Code, ErrDatabase.Code, ErrParamInvalid.Code}
	for _, c := range sysCodes {
		if c < 10000 || c >= 20000 {
			t.Errorf("系统级错误码 %d 不在 10xxx 段", c)
		}
	}
	bizCodes := []int{ErrReviewNotFound.Code, ErrSonarGateBlocked.Code, ErrTenantNotFound.Code}
	for _, c := range bizCodes {
		if c < 20000 || c >= 30000 {
			t.Errorf("业务级错误码 %d 不在 20xxx 段", c)
		}
	}
	thirdCodes := []int{ErrGitAuthFailed.Code, ErrSonarConnectFailed.Code, ErrMinIOUploadFailed.Code}
	for _, c := range thirdCodes {
		if c < 30000 || c >= 40000 {
			t.Errorf("第三方错误码 %d 不在 30xxx 段", c)
		}
	}
}
