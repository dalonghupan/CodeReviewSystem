package bizadapter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ============================================================
// Unified Diff 结构化解析器（LLD §2 步骤8）
// 将原始 diff 文本解析为「文件路径-行号-变更类型」映射结构，
// 供 Monaco 编辑器分栏渲染与行级评论三元组绑定
// ============================================================

// ParsedDiff 单文件结构化 Diff
type ParsedDiff struct {
	FilePath       string
	OldPath        string
	Hunks          []*DiffHunk
	TotalAdditions int
	TotalDeletions int
}

// DiffHunk Diff 块（@@ -old_start,old_lines +new_start,new_lines @@）
type DiffHunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []*DiffLine
}

// DiffLine 单行变更
type DiffLine struct {
	OldLineNum int    // 旧文件行号（新增行为0）
	NewLineNum int    // 新文件行号（删除行为0）
	ChangeType string // context / added / deleted
	Content    string
}

// hunk 头正则：@@ -1,3 +1,4 @@ 或 @@ -1 +1 @@
var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// ParseUnifiedDiff 解析单文件 unified diff 文本
// filePath 为新文件路径，oldPath 为重命名前路径（可空）
func ParseUnifiedDiff(filePath, oldPath, diffText string) (*ParsedDiff, error) {
	if strings.TrimSpace(diffText) == "" {
		return nil, fmt.Errorf("diff文本为空: %s", filePath)
	}

	pd := &ParsedDiff{FilePath: filePath, OldPath: oldPath}
	var curHunk *DiffHunk
	var oldLine, newLine int

	for _, raw := range strings.Split(diffText, "\n") {
		// hunk 头
		if m := hunkHeaderRe.FindStringSubmatch(raw); m != nil {
			curHunk = &DiffHunk{
				OldStart: atoiDefault(m[1], 0),
				OldLines: atoiDefault(m[2], 1), // 省略时默认为1
				NewStart: atoiDefault(m[3], 0),
				NewLines: atoiDefault(m[4], 1),
			}
			pd.Hunks = append(pd.Hunks, curHunk)
			oldLine = curHunk.OldStart
			newLine = curHunk.NewStart
			continue
		}

		if curHunk == nil {
			continue // 跳过文件头（--- +++ 等）
		}

		switch {
		case strings.HasPrefix(raw, "+"):
			curHunk.Lines = append(curHunk.Lines, &DiffLine{
				OldLineNum: 0, NewLineNum: newLine,
				ChangeType: "added", Content: raw[1:],
			})
			newLine++
			pd.TotalAdditions++
		case strings.HasPrefix(raw, "-"):
			curHunk.Lines = append(curHunk.Lines, &DiffLine{
				OldLineNum: oldLine, NewLineNum: 0,
				ChangeType: "deleted", Content: raw[1:],
			})
			oldLine++
			pd.TotalDeletions++
		case strings.HasPrefix(raw, `\`): // "\ No newline at end of file"
			continue
		default: // 上下文行（可能以空格开头或空行）
			content := raw
			if strings.HasPrefix(raw, " ") {
				content = raw[1:]
			}
			curHunk.Lines = append(curHunk.Lines, &DiffLine{
				OldLineNum: oldLine, NewLineNum: newLine,
				ChangeType: "context", Content: content,
			})
			oldLine++
			newLine++
		}
	}

	if len(pd.Hunks) == 0 {
		return nil, fmt.Errorf("未解析到有效diff块: %s", filePath)
	}
	return pd, nil
}

// IsSensitiveFile 判断文件是否命中敏感后缀（内容屏蔽，SRS F01-04）
func IsSensitiveFile(filePath string, sensitiveExts []string) bool {
	lower := strings.ToLower(filePath)
	for _, ext := range sensitiveExts {
		if strings.HasSuffix(lower, strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
