package data

import "testing"

// TestCheckPermission RBAC 权限层级匹配（admin > write > read）
func TestCheckPermission(t *testing.T) {
	perms := permSet{
		"repo:read",
		"review:write",
		"tenant:admin",
	}

	cases := []struct {
		name     string
		resource string
		action   string
		want     bool
	}{
		{"精确匹配read", "repo", "read", true},
		{"read不能覆盖write", "repo", "write", false},
		{"write隐含read", "review", "read", true},
		{"write精确匹配", "review", "write", true},
		{"admin隐含全部", "tenant", "delete", true},
		{"admin隐含read", "tenant", "read", true},
		{"资源不存在", "report", "read", false},
		{"跨资源不匹配", "repo", "admin", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CheckPermission(perms, c.resource, c.action); got != c.want {
				t.Errorf("CheckPermission(%s,%s) = %v, want %v", c.resource, c.action, got, c.want)
			}
		})
	}
}

func TestCheckPermissionEmpty(t *testing.T) {
	if CheckPermission(nil, "repo", "read") {
		t.Error("空权限集不应放行任何操作")
	}
	// 畸形条目应被忽略
	if CheckPermission(permSet{"invalid", ":read", "repo:"}, "repo", "read") {
		t.Error("畸形权限条目不应放行")
	}
}
