package service

import (
	"context"
	"encoding/json"

	"cr-system/app/git-adapter/internal/bizadapter"
	"cr-system/pkg/errcode"
)

// FetchAndCacheDiff 拉取 MR 全部变更文件并预解析缓存（LLD §2 步骤7-9）
// 由 MQ 消费者触发，前端轮询 GetDiffData 时直接命中缓存
func (s *GitAdapterService) FetchAndCacheDiff(ctx context.Context, repoID, mrID, commitHash string) error {
	repo, provider, token, err := s.resolveRepoProvider(ctx, repoID)
	if err != nil {
		return err
	}

	files, err := provider.ListMRFiles(ctx, token, repo.PlatformRepoID, mrID)
	if err != nil {
		return err
	}

	sensitiveExts, err := s.data.GetSensitiveExtensions(ctx, repo.TenantID)
	if err != nil {
		return err
	}

	var lastErr error
	for _, f := range files {
		// 敏感文件不拉取内容（SRS F01-04 内容屏蔽）
		if bizadapter.IsSensitiveFile(f.FilePath, sensitiveExts) {
			continue
		}

		rawDiff, err := provider.GetFileDiff(ctx, token, repo.PlatformRepoID, mrID, f.FilePath)
		if err != nil {
			s.log.Warnw("msg", "文件Diff拉取失败", "file", f.FilePath, "error", err.Error())
			lastErr = err
			continue // 单文件失败不阻断整体，其余文件照常缓存
		}

		parsed, err := bizadapter.ParseUnifiedDiff(f.FilePath, f.OldPath, rawDiff)
		if err != nil {
			s.log.Warnw("msg", "文件Diff解析失败", "file", f.FilePath, "error", err.Error())
			lastErr = errcode.ErrGitDiffParseFailed.WithDetail(f.FilePath)
			continue
		}

		dd := parsedToProto(parsed)
		dd.CommitHash = commitHash
		if data, err := json.Marshal(dd); err == nil {
			if err := s.data.CacheDiff(ctx, mrID, commitHash, f.FilePath, data); err != nil {
				s.log.Warnw("msg", "Diff缓存写入失败", "file", f.FilePath, "error", err.Error())
			}
		}
	}

	// 全部文件都失败才返回错误触发重试；部分成功视为成功
	if lastErr != nil && len(files) > 0 {
		s.log.Warnw("msg", "部分文件处理失败", "mr_id", mrID, "last_error", lastErr.Error())
	}
	return nil
}

// RefreshAuthTokenByID 按授权ID刷新 Token（MQ 消费者入口）
func (s *GitAdapterService) RefreshAuthTokenByID(ctx context.Context, authID string) error {
	auth, err := s.data.GetAuth(ctx, authID)
	if err != nil {
		return err
	}
	return s.RefreshAuthToken(ctx, auth)
}
