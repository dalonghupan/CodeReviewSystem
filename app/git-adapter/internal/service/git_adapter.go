// Package service git-adapter 业务逻辑层（LLD §3.2）
// 实现 GitAdapterService 全部 RPC：OAuth授权、仓库管理、MR/Diff数据、黑白名单
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "cr-system/api/crsystem/v1"
	"cr-system/app/git-adapter/internal/bizadapter"
	"cr-system/app/git-adapter/internal/data"
	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// 系统内部限流标识（服务自身调用平台接口时的计数维度）
const selfRateLimitKey = "cr-system"

// GitAdapterService 业务服务
type GitAdapterService struct {
	v1.UnimplementedGitAdapterServiceServer

	data      *data.Data
	registry  *bizadapter.Registry
	aesKey    []byte
	rateLimit int
	log       *log.Helper
}

// NewGitAdapterService 构造服务
func NewGitAdapterService(d *data.Data, registry *bizadapter.Registry, aesKey []byte, maxCallsPerHour int, logger log.Logger) *GitAdapterService {
	return &GitAdapterService{
		data:      d,
		registry:  registry,
		aesKey:    aesKey,
		rateLimit: maxCallsPerHour,
		log:       log.NewHelper(logger),
	}
}

// ==================== OAuth 授权管理 ====================

// GetAuthURL 生成授权跳转URL（state 存 Redis，10分钟有效）
func (s *GitAdapterService) GetAuthURL(ctx context.Context, req *v1.GetAuthURLReq) (*v1.GetAuthURLResp, error) {
	platform := platformName(req.Platform)
	provider, err := s.registry.Get(platform)
	if err != nil {
		return nil, errcode.ErrParamInvalid.WithDetail(err.Error())
	}
	if req.TenantId == "" {
		return nil, errcode.ErrParamInvalid.WithDetail("租户ID不能为空")
	}

	state := util.NewUUID()
	if err := s.data.SaveOAuthState(ctx, state, req.TenantId, platform); err != nil {
		return nil, errcode.ErrRedis.WithDetail(err.Error())
	}
	return &v1.GetAuthURLResp{
		AuthUrl: provider.BuildAuthURL(state),
		State:   state,
	}, nil
}

// HandleOAuthCallback OAuth 回调：校验state → 换Token → 加密入库（LLD §3.2-3）
func (s *GitAdapterService) HandleOAuthCallback(ctx context.Context, req *v1.OAuthCallbackReq) (*v1.OAuthCallbackResp, error) {
	platform := platformName(req.Platform)
	provider, err := s.registry.Get(platform)
	if err != nil {
		return nil, errcode.ErrParamInvalid.WithDetail(err.Error())
	}

	// state 一次性消费（防CSRF/重放）
	stateTenant, statePlatform, err := s.data.ConsumeOAuthState(ctx, req.State)
	if err != nil {
		return nil, err
	}
	if statePlatform != platform || (req.TenantId != "" && stateTenant != req.TenantId) {
		return nil, errcode.ErrGitAuthFailed.WithDetail("state与平台/租户不匹配")
	}

	// 换 Token
	token, err := provider.ExchangeToken(ctx, req.Code)
	if err != nil {
		return nil, err
	}

	// AES 加密存储（LLD §9-4：数据库不可解密查看原文）
	encAccess, err := util.AESEncrypt(token.AccessToken, s.aesKey)
	if err != nil {
		return nil, errcode.ErrInternal.WithDetail("Token加密失败")
	}
	encRefresh := ""
	if token.RefreshToken != "" {
		if encRefresh, err = util.AESEncrypt(token.RefreshToken, s.aesKey); err != nil {
			return nil, errcode.ErrInternal.WithDetail("Token加密失败")
		}
	}

	expireAt := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	auth := &data.GitAuth{
		TenantID:         stateTenant,
		Platform:         platform,
		PlatformUserID:   token.PlatformUser.ID,
		PlatformUsername: token.PlatformUser.Username,
		PlatformAvatar:   token.PlatformUser.AvatarURL,
		AccessToken:      encAccess,
		RefreshToken:     encRefresh,
		TokenExpireTime:  expireAt,
	}
	if err := s.data.CreateAuth(ctx, auth); err != nil {
		return nil, err
	}
	s.log.Infow("msg", "Git授权绑定成功", "platform", platform, "user", token.PlatformUser.Username, "tenant_id", stateTenant)

	return &v1.OAuthCallbackResp{
		AuthId:           auth.AuthID,
		PlatformUsername: token.PlatformUser.Username,
		ExpireAt:         timestamppb.New(expireAt),
	}, nil
}

// ListAuthorizations 授权列表
func (s *GitAdapterService) ListAuthorizations(ctx context.Context, req *v1.ListAuthReq) (*v1.ListAuthResp, error) {
	items, err := s.data.ListAuths(ctx, req.TenantId, platformName(req.Platform))
	if err != nil {
		return nil, err
	}
	resp := &v1.ListAuthResp{}
	for _, a := range items {
		info := &v1.AuthInfo{
			AuthId:           a.AuthID,
			TenantId:         a.TenantID,
			Platform:         platformEnum(a.Platform),
			PlatformUsername: a.PlatformUsername,
			AvatarUrl:        a.PlatformAvatar,
			TokenExpireAt:    timestamppb.New(a.TokenExpireTime),
			SyncStatus:       a.SyncStatus,
			CreatedAt:        timestamppb.New(a.CreatedAt),
		}
		resp.Items = append(resp.Items, info)
	}
	return resp, nil
}

// RevokeAuthorization 撤销授权（联动解绑关联仓库）
func (s *GitAdapterService) RevokeAuthorization(ctx context.Context, req *v1.RevokeAuthReq) (*v1.OperateResult, error) {
	repos, err := s.data.ListReposByAuth(ctx, req.AuthId)
	if err != nil {
		return nil, err
	}
	for _, r := range repos {
		if err := s.data.UnbindRepository(ctx, r.RepoID); err != nil {
			s.log.Warnw("msg", "联动解绑仓库失败", "repo_id", r.RepoID, "error", err.Error())
		}
	}
	if err := s.data.DeleteAuth(ctx, req.AuthId); err != nil {
		return nil, err
	}
	return &v1.OperateResult{Success: true, Message: "授权已撤销"}, nil
}

// ==================== 仓库管理 ====================

// ListRemoteRepos 拉取平台侧仓库（限流校验后透传）
func (s *GitAdapterService) ListRemoteRepos(ctx context.Context, req *v1.ListRemoteReposReq) (*v1.ListRemoteReposResp, error) {
	auth, token, err := s.resolveToken(ctx, req.AuthId)
	if err != nil {
		return nil, err
	}
	provider, err := s.registry.Get(auth.Platform)
	if err != nil {
		return nil, errcode.ErrParamInvalid.WithDetail(err.Error())
	}
	if err := s.checkRateLimit(ctx, auth.Platform); err != nil {
		return nil, err
	}

	page, pageSize := int(req.Page), int(req.PageSize)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	repos, total, err := provider.ListRepos(ctx, token, req.Keyword, page, pageSize)
	if err != nil {
		return nil, err
	}

	resp := &v1.ListRemoteReposResp{Total: uint32(total)}
	for _, r := range repos {
		resp.Items = append(resp.Items, &v1.RemoteRepoInfo{
			PlatformRepoId: r.PlatformRepoID,
			FullName:       r.FullName,
			Description:    r.Description,
			DefaultBranch:  r.DefaultBranch,
			CloneUrl:       r.CloneURL,
			WebUrl:         r.WebURL,
			IsPrivate:      r.IsPrivate,
			UpdatedAt:      timestamppb.New(r.UpdatedAt),
		})
	}
	return resp, nil
}

// BindRepository 绑定仓库（黑名单拦截，LLD §3.2-4）
func (s *GitAdapterService) BindRepository(ctx context.Context, req *v1.BindRepoReq) (*v1.RepoInfo, error) {
	// 黑名单校验
	if err := s.checkBlacklist(ctx, req.TenantId, req.FullName); err != nil {
		return nil, err
	}
	if _, err := s.data.GetAuth(ctx, req.AuthId); err != nil {
		return nil, err
	}

	repo := &data.GitRepository{
		TenantID:       req.TenantId,
		AuthID:         req.AuthId,
		Platform:       platformName(req.Platform),
		PlatformRepoID: req.PlatformRepoId,
		FullName:       req.FullName,
	}
	if err := s.data.BindRepository(ctx, repo); err != nil {
		return nil, err
	}
	return &v1.RepoInfo{
		RepoId:    repo.RepoID,
		TenantId:  repo.TenantID,
		Platform:  req.Platform,
		FullName:  repo.FullName,
		CreatedAt: timestamppb.New(time.Now()),
	}, nil
}

// ListRepositories 绑定仓库列表
func (s *GitAdapterService) ListRepositories(ctx context.Context, req *v1.ListReposReq) (*v1.ListReposResp, error) {
	page, pageSize := paginationOf(req.Pagination)
	items, total, err := s.data.ListRepositories(ctx, req.TenantId, platformName(req.Platform), req.Keyword, page, pageSize)
	if err != nil {
		return nil, err
	}
	resp := &v1.ListReposResp{Pagination: paginationResp(page, pageSize, total)}
	for _, r := range items {
		resp.Items = append(resp.Items, repoToProto(r))
	}
	return resp, nil
}

// UnbindRepository 解绑仓库
func (s *GitAdapterService) UnbindRepository(ctx context.Context, req *v1.UnbindRepoReq) (*v1.OperateResult, error) {
	if err := s.data.UnbindRepository(ctx, req.RepoId); err != nil {
		return nil, err
	}
	return &v1.OperateResult{Success: true, Message: "仓库已解绑"}, nil
}

// ==================== MR 与 Diff 数据 ====================

// GetMRData 获取 MR 基础信息（cr-core 创建评审单时调用）
func (s *GitAdapterService) GetMRData(ctx context.Context, req *v1.GetMRDataReq) (*v1.MRData, error) {
	repo, provider, token, err := s.resolveRepoProvider(ctx, req.RepoId)
	if err != nil {
		return nil, err
	}
	mr, err := provider.GetMergeRequest(ctx, token, repo.PlatformRepoID, req.MrId)
	if err != nil {
		return nil, err
	}
	return mrToProto(repo.RepoID, mr), nil
}

// GetMRFiles 获取 MR 变更文件列表（敏感文件标记）
func (s *GitAdapterService) GetMRFiles(ctx context.Context, req *v1.GetMRFilesReq) (*v1.GetMRFilesResp, error) {
	repo, provider, token, err := s.resolveRepoProvider(ctx, req.RepoId)
	if err != nil {
		return nil, err
	}
	files, err := provider.ListMRFiles(ctx, token, repo.PlatformRepoID, req.MrId)
	if err != nil {
		return nil, err
	}

	sensitiveExts, err := s.data.GetSensitiveExtensions(ctx, repo.TenantID)
	if err != nil {
		return nil, err
	}

	resp := &v1.GetMRFilesResp{}
	for _, f := range files {
		resp.Files = append(resp.Files, &v1.ChangedFile{
			FilePath:    f.FilePath,
			ChangeType:  mapProtoChangeType(f.ChangeType),
			Additions:   uint32(f.Additions),
			Deletions:   uint32(f.Deletions),
			IsSensitive: bizadapter.IsSensitiveFile(f.FilePath, sensitiveExts),
			FileSize:    uint64(f.FileSize),
		})
	}
	return resp, nil
}

// GetDiffData 获取结构化 Diff（缓存优先 diff:{mrId}:{commitHash} TTL=7天，LLD §2 步骤9）
func (s *GitAdapterService) GetDiffData(ctx context.Context, req *v1.GetDiffDataReq) (*v1.DiffData, error) {
	repo, provider, token, err := s.resolveRepoProvider(ctx, req.RepoId)
	if err != nil {
		return nil, err
	}

	// 敏感文件：只返回标记，不返回内容（SRS F01-04）
	sensitiveExts, err := s.data.GetSensitiveExtensions(ctx, repo.TenantID)
	if err != nil {
		return nil, err
	}
	if bizadapter.IsSensitiveFile(req.FilePath, sensitiveExts) {
		return &v1.DiffData{FilePath: req.FilePath, CommitHash: req.CommitHash, IsReady: true}, nil
	}

	// 缓存命中直接返回
	if cached, err := s.data.GetCachedDiff(ctx, req.MrId, req.CommitHash, req.FilePath); err == nil && cached != nil {
		var dd v1.DiffData
		if json.Unmarshal(cached, &dd) == nil {
			return &dd, nil
		}
	}

	// 拉取原始 diff 并结构化解析
	rawDiff, err := provider.GetFileDiff(ctx, token, repo.PlatformRepoID, req.MrId, req.FilePath)
	if err != nil {
		return nil, err
	}
	parsed, err := bizadapter.ParseUnifiedDiff(req.FilePath, "", rawDiff)
	if err != nil {
		return nil, errcode.ErrGitDiffParseFailed.WithDetail(err.Error())
	}

	dd := parsedToProto(parsed)
	dd.CommitHash = req.CommitHash
	// 回写缓存（7天TTL）
	if data, err := json.Marshal(dd); err == nil {
		_ = s.data.CacheDiff(ctx, req.MrId, req.CommitHash, req.FilePath, data)
	}
	return dd, nil
}

// TriggerSync 手动触发仓库同步（标记 syncing，实际由 job-scheduler 调度增量同步）
func (s *GitAdapterService) TriggerSync(ctx context.Context, req *v1.TriggerSyncReq) (*v1.OperateResult, error) {
	if err := s.data.UpdateRepoSyncStatus(ctx, req.RepoId, "syncing"); err != nil {
		return nil, err
	}
	return &v1.OperateResult{Success: true, Message: "同步任务已触发"}, nil
}

// ==================== 黑白名单与敏感文件 ====================

// SetRepoBlacklist 配置黑名单
func (s *GitAdapterService) SetRepoBlacklist(ctx context.Context, req *v1.SetBlacklistReq) (*v1.OperateResult, error) {
	if err := s.data.SetBlacklist(ctx, req.TenantId, req.RepoPatterns, ""); err != nil {
		return nil, err
	}
	return &v1.OperateResult{Success: true, Message: "黑名单已更新"}, nil
}

// GetRepoBlacklist 查询黑名单
func (s *GitAdapterService) GetRepoBlacklist(ctx context.Context, req *v1.GetBlacklistReq) (*v1.GetBlacklistResp, error) {
	patterns, err := s.data.GetBlacklist(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	return &v1.GetBlacklistResp{RepoPatterns: patterns, UpdatedAt: timestamppb.New(time.Now())}, nil
}

// SetSensitiveExtensions 配置敏感文件后缀
func (s *GitAdapterService) SetSensitiveExtensions(ctx context.Context, req *v1.SetSensitiveExtReq) (*v1.OperateResult, error) {
	if err := s.data.SetSensitiveExtensions(ctx, req.TenantId, req.Extensions, ""); err != nil {
		return nil, err
	}
	return &v1.OperateResult{Success: true, Message: "敏感文件配置已更新"}, nil
}

// GetSensitiveExtensions 查询敏感文件后缀
func (s *GitAdapterService) GetSensitiveExtensions(ctx context.Context, req *v1.GetSensitiveExtReq) (*v1.GetSensitiveExtResp, error) {
	exts, err := s.data.GetSensitiveExtensions(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	return &v1.GetSensitiveExtResp{Extensions: exts, UpdatedAt: timestamppb.New(time.Now())}, nil
}

// ==================== 内部辅助 ====================

// resolveToken 取授权记录并解密 Token；临期（<5分钟）自动刷新（LLD §3.2-3）
func (s *GitAdapterService) resolveToken(ctx context.Context, authID string) (*data.GitAuth, string, error) {
	auth, err := s.data.GetAuth(ctx, authID)
	if err != nil {
		return nil, "", err
	}

	// 临期自动刷新
	if time.Until(auth.TokenExpireTime) < 5*time.Minute && auth.RefreshToken != "" {
		if err := s.RefreshAuthToken(ctx, auth); err != nil {
			s.log.Warnw("msg", "Token临期刷新失败，尝试使用现有Token", "auth_id", authID, "error", err.Error())
		} else {
			// 刷新成功重新读取
			if refreshed, err := s.data.GetAuth(ctx, authID); err == nil {
				auth = refreshed
			}
		}
	}

	token, err := util.AESDecrypt(auth.AccessToken, s.aesKey)
	if err != nil {
		return nil, "", errcode.ErrInternal.WithDetail("Token解密失败")
	}
	return auth, token, nil
}

// RefreshAuthToken 刷新指定授权的 Token（消费 token_refresh_topic 也走此方法）
func (s *GitAdapterService) RefreshAuthToken(ctx context.Context, auth *data.GitAuth) error {
	provider, err := s.registry.Get(auth.Platform)
	if err != nil {
		return err
	}
	refreshToken, err := util.AESDecrypt(auth.RefreshToken, s.aesKey)
	if err != nil {
		return errcode.ErrInternal.WithDetail("RefreshToken解密失败")
	}

	token, err := provider.RefreshToken(ctx, refreshToken)
	if err != nil {
		return err
	}

	encAccess, err := util.AESEncrypt(token.AccessToken, s.aesKey)
	if err != nil {
		return errcode.ErrInternal.WithDetail("Token加密失败")
	}
	encRefresh := auth.RefreshToken // 平台未返回新refresh时保留旧的
	if token.RefreshToken != "" {
		if encRefresh, err = util.AESEncrypt(token.RefreshToken, s.aesKey); err != nil {
			return errcode.ErrInternal.WithDetail("Token加密失败")
		}
	}

	expireAt := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	if err := s.data.UpdateAuthToken(ctx, auth.AuthID, encAccess, encRefresh, expireAt); err != nil {
		return err
	}
	s.log.Infow("msg", "Token刷新成功", "auth_id", auth.AuthID, "platform", auth.Platform)
	return nil
}

// resolveRepoProvider 仓库 → 平台适配器 + Token 一站式解析（含限流）
func (s *GitAdapterService) resolveRepoProvider(ctx context.Context, repoID string) (*data.GitRepository, bizadapter.GitProvider, string, error) {
	repo, err := s.data.GetRepository(ctx, repoID)
	if err != nil {
		return nil, nil, "", err
	}
	auth, token, err := s.resolveToken(ctx, repo.AuthID)
	if err != nil {
		return nil, nil, "", err
	}
	provider, err := s.registry.Get(auth.Platform)
	if err != nil {
		return nil, nil, "", errcode.ErrParamInvalid.WithDetail(err.Error())
	}
	if err := s.checkRateLimit(ctx, auth.Platform); err != nil {
		return nil, nil, "", err
	}
	return repo, provider, token, nil
}

// checkRateLimit 限流校验（LLD §3.2-2：触发阈值拒绝调用，防平台封禁）
func (s *GitAdapterService) checkRateLimit(ctx context.Context, platform string) error {
	count, err := s.data.IncrGitCall(ctx, platform, selfRateLimitKey)
	if err != nil {
		s.log.Warnw("msg", "限流计数异常，放行本次调用", "error", err.Error())
		return nil // 限流组件故障不阻塞业务（降级策略）
	}
	if s.rateLimit > 0 && count > int64(s.rateLimit) {
		return errcode.ErrGitRateLimited.WithDetail(
			fmt.Sprintf("平台 %s 小时调用量已达上限 %d", platform, s.rateLimit))
	}
	return nil
}

// checkBlacklist 黑名单拦截（支持 * 通配符）
func (s *GitAdapterService) checkBlacklist(ctx context.Context, tenantID, repoFullName string) error {
	patterns, err := s.data.GetBlacklist(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, p := range patterns {
		if ok, _ := path.Match(p, repoFullName); ok || p == repoFullName {
			return errcode.ErrRepoBlacklisted.WithDetail("命中规则: " + p)
		}
	}
	return nil
}

// ==================== 转换辅助 ====================

func platformName(e v1.GitPlatform) string {
	switch e {
	case v1.GitPlatform_GIT_PLATFORM_GITLAB:
		return bizadapter.PlatformGitLab
	case v1.GitPlatform_GIT_PLATFORM_GITEE:
		return bizadapter.PlatformGitee
	case v1.GitPlatform_GIT_PLATFORM_GITHUB:
		return bizadapter.PlatformGitHub
	default:
		return ""
	}
}

func platformEnum(name string) v1.GitPlatform {
	switch strings.ToLower(name) {
	case bizadapter.PlatformGitLab:
		return v1.GitPlatform_GIT_PLATFORM_GITLAB
	case bizadapter.PlatformGitee:
		return v1.GitPlatform_GIT_PLATFORM_GITEE
	case bizadapter.PlatformGitHub:
		return v1.GitPlatform_GIT_PLATFORM_GITHUB
	default:
		return v1.GitPlatform_GIT_PLATFORM_UNSPECIFIED
	}
}

func mapProtoChangeType(ct string) v1.DiffChangeType {
	switch bizadapter.MapChangeType(ct) {
	case "added":
		return v1.DiffChangeType_DIFF_CHANGE_TYPE_ADD
	case "deleted":
		return v1.DiffChangeType_DIFF_CHANGE_TYPE_DELETE
	default:
		return v1.DiffChangeType_DIFF_CHANGE_TYPE_MODIFY
	}
}

func repoToProto(r *data.GitRepository) *v1.RepoInfo {
	info := &v1.RepoInfo{
		RepoId:        r.RepoID,
		TenantId:      r.TenantID,
		Platform:      platformEnum(r.Platform),
		FullName:      r.FullName,
		DefaultBranch: r.DefaultBranch,
		WebUrl:        r.WebURL,
		SyncStatus:    r.SyncStatus,
		CreatedAt:     timestamppb.New(r.CreatedAt),
	}
	if r.LastSyncedAt != nil {
		info.LastSyncedAt = timestamppb.New(*r.LastSyncedAt)
	}
	return info
}

func mrToProto(repoID string, mr *bizadapter.MergeRequest) *v1.MRData {
	return &v1.MRData{
		MrId:             mr.MRID,
		RepoId:           repoID,
		Title:            mr.Title,
		Description:      mr.Description,
		SourceBranch:     mr.SourceBranch,
		TargetBranch:     mr.TargetBranch,
		AuthorName:       mr.AuthorName,
		AuthorAvatar:     mr.AuthorAvatar,
		Status:           mr.Status,
		CommitCount:      uint32(mr.CommitCount),
		ChangedFileCount: uint32(mr.ChangedFileCount),
		WebUrl:           mr.WebURL,
		CreatedAt:        timestamppb.New(mr.CreatedAt),
		UpdatedAt:        timestamppb.New(mr.UpdatedAt),
	}
}

func parsedToProto(pd *bizadapter.ParsedDiff) *v1.DiffData {
	dd := &v1.DiffData{
		FilePath:       pd.FilePath,
		IsReady:        true,
		TotalAdditions: uint32(pd.TotalAdditions),
		TotalDeletions: uint32(pd.TotalDeletions),
	}
	for _, h := range pd.Hunks {
		hunk := &v1.DiffHunk{
			OldStart: uint32(h.OldStart),
			OldLines: uint32(h.OldLines),
			NewStart: uint32(h.NewStart),
			NewLines: uint32(h.NewLines),
		}
		for _, l := range h.Lines {
			hunk.Lines = append(hunk.Lines, &v1.DiffLine{
				OldLineNum: uint32(l.OldLineNum),
				NewLineNum: uint32(l.NewLineNum),
				ChangeType: mapProtoChangeType(l.ChangeType),
				Content:    l.Content,
			})
		}
		dd.Hunks = append(dd.Hunks, hunk)
	}
	return dd
}

func paginationOf(p *v1.Pagination) (uint32, uint32) {
	if p == nil {
		return util.NormalizePage(0, 0)
	}
	return util.NormalizePage(p.Page, p.PageSize)
}

func paginationResp(page, pageSize, total uint32) *v1.PaginationResp {
	return &v1.PaginationResp{
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: util.TotalPages(total, pageSize),
	}
}

// 确保实现接口（编译期检查）
var _ v1.GitAdapterServiceServer = (*GitAdapterService)(nil)
