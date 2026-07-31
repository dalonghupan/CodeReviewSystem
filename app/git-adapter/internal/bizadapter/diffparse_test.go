package bizadapter

import "testing"

const sampleDiff = `@@ -1,4 +1,5 @@
 package main

+import "fmt"
 func main() {
-	println("hello")
+	fmt.Println("hello world")
 }
@@ -10,2 +11,3 @@ func util() {
 	a := 1
+	b := 2
 	return a
 }`

// TestParseUnifiedDiff 核心解析逻辑（LLD §2 步骤8）
func TestParseUnifiedDiff(t *testing.T) {
	pd, err := ParseUnifiedDiff("main.go", "", sampleDiff)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if pd.FilePath != "main.go" {
		t.Errorf("FilePath = %q", pd.FilePath)
	}
	if len(pd.Hunks) != 2 {
		t.Fatalf("hunk数量 = %d, want 2", len(pd.Hunks))
	}

	// 第一个 hunk：@@ -1,4 +1,5 @@
	h1 := pd.Hunks[0]
	if h1.OldStart != 1 || h1.OldLines != 4 || h1.NewStart != 1 || h1.NewLines != 5 {
		t.Errorf("hunk头解析错误: %+v", h1)
	}

	// 行号映射校验
	var addedLine, deletedLine *DiffLine
	for _, l := range h1.Lines {
		if l.ChangeType == "added" && l.Content == `import "fmt"` {
			addedLine = l
		}
		if l.ChangeType == "deleted" {
			deletedLine = l
		}
	}
	if addedLine == nil || addedLine.NewLineNum != 3 || addedLine.OldLineNum != 0 {
		t.Errorf("新增行行号错误: %+v", addedLine)
	}
	if deletedLine == nil || deletedLine.OldLineNum != 4 || deletedLine.NewLineNum != 0 {
		t.Errorf("删除行行号错误: %+v", deletedLine)
	}

	// 增删统计（hunk1: +import/+fmt.Println = 2；hunk2: +b := 2 = 1；删除 -println = 1）
	if pd.TotalAdditions != 3 {
		t.Errorf("TotalAdditions = %d, want 3", pd.TotalAdditions)
	}
	if pd.TotalDeletions != 1 {
		t.Errorf("TotalDeletions = %d, want 1", pd.TotalDeletions)
	}

	// 第二个 hunk 首行（上下文行）行号
	h2 := pd.Hunks[1]
	if h2.OldStart != 10 || h2.NewStart != 11 {
		t.Errorf("第二个hunk起始行错误: %+v", h2)
	}
	if len(h2.Lines) == 0 || h2.Lines[0].OldLineNum != 10 || h2.Lines[0].NewLineNum != 11 {
		t.Errorf("上下文行行号错误: %+v", h2.Lines[0])
	}
}

func TestParseUnifiedDiffEmpty(t *testing.T) {
	if _, err := ParseUnifiedDiff("a.go", "", ""); err == nil {
		t.Error("空diff应返回错误")
	}
	if _, err := ParseUnifiedDiff("a.go", "", "no hunk header here"); err == nil {
		t.Error("无hunk头的diff应返回错误")
	}
}

func TestIsSensitiveFile(t *testing.T) {
	exts := []string{".pem", ".key", ".env"}
	cases := []struct {
		path string
		want bool
	}{
		{"certs/server.pem", true},
		{"config/.env", true},
		{"keys/private.KEY", true}, // 大小写不敏感
		{"main.go", false},
		{"docs/env-setup.md", false}, // 后缀不匹配
	}
	for _, c := range cases {
		if got := IsSensitiveFile(c.path, exts); got != c.want {
			t.Errorf("IsSensitiveFile(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestMapChangeType(t *testing.T) {
	if MapChangeType("removed") != "deleted" {
		t.Error("removed 应映射为 deleted")
	}
	if MapChangeType("new") != "added" {
		t.Error("new 应映射为 added")
	}
	if MapChangeType("renamed") != "renamed" {
		t.Error("renamed 应保留")
	}
}
