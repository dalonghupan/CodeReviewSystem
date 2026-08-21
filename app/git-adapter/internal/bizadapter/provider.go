// Package bizadapter Git 三方平台适配层（HLD §8-1）
// 统一封装 GitLab/Gitee/GitHub 接口，新增平台仅扩展适配器，业务核心零侵入
package bizadapter

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Platform 平台标识常量（与 DDL git_auth.platform 一致）
const (
	PlatformGitLab = "gitlab"
	PlatformGitee  = "gitee"
	PlatformGitHub = "github"
)

// TokenPair OAuth 令牌对
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // 秒
	PlatformUser PlatformUser
}

// PlatformUser 平台侧用户信息
type PlatformUser struct {
	ID        string
	Username  string
	AvatarURL string
}

// RemoteRepo 平台侧仓库信息
type RemoteRepo struct {
	PlatformRepoID string
	FullName       string // org/repo
	Description    string
	DefaultBranch  string
	CloneURL       string
	WebURL         string
	IsPrivate      bool
	UpdatedAt      time.Time
}

// MergeRequest MR 基础信息
type MergeRequest struct {
	MRID             string // 平台侧 MR 编号（IID）
	Title            string
	Description      string
	SourceBranch     string
	TargetBranch     string
	AuthorName       string
	AuthorAvatar     string
	Status           string // opened / merged / closed
	CommitCount      int
	ChangedFileCount int
	LatestCommitHash string
	WebURL           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ChangedFile MR 变更文件
type ChangedFile struct {
	FilePath   string
	OldPath    string // 重命名前路径
	ChangeType string // added / deleted / modified / renamed
	Additions  int
	Deletions  int
	FileSize   int64
}

// GitProvider 平台适配器统一接口
// 三个平台（GitLab/Gitee/GitHub）各自实现，业务层面向接口编程
type GitProvider interface {
	// Platform 返回平台标识
	Platform() string

	// BuildAuthURL 构造 OAuth 授权跳转 URL（state 由调用方生成并存 Redis）
	BuildAuthURL(state string) string

	// ExchangeToken 用授权码换取 Token（OAuth 回调）
	ExchangeToken(ctx context.Context, code string) (*TokenPair, error)

	// RefreshToken 刷新 Token（定时任务）
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)

	// ListRepos 拉取授权用户可见仓库列表
	ListRepos(ctx context.Context, accessToken, keyword string, page, pageSize int) ([]*RemoteRepo, int, error)

	// GetMergeRequest 获取 MR 基础信息
	GetMergeRequest(ctx context.Context, accessToken, platformRepoID, mrID string) (*MergeRequest, error)

	// ListMRFiles 获取 MR 变更文件列表
	ListMRFiles(ctx context.Context, accessToken, platformRepoID, mrID string) ([]*ChangedFile, error)

	// GetFileDiff 获取指定文件的原始 unified diff 文本
	GetFileDiff(ctx context.Context, accessToken, platformRepoID, mrID, filePath string) (string, error)
}

// Registry 平台适配器注册表
type Registry struct {
	providers map[string]GitProvider
}

// NewRegistry 创建注册表
func NewRegistry(providers ...GitProvider) *Registry {
	r := &Registry{providers: make(map[string]GitProvider, len(providers))}
	for _, p := range providers {
		r.providers[p.Platform()] = p
	}
	return r
}

// Get 按平台标识获取适配器
func (r *Registry) Get(platform string) (GitProvider, error) {
	p, ok := r.providers[strings.ToLower(platform)]
	if !ok {
		return nil, fmt.Errorf("不支持的Git平台: %s", platform)
	}
	return p, nil
}

// MapChangeType 平台变更类型 → 系统统一类型（与 proto DiffChangeType 对应）
func MapChangeType(platformType string) string {
	switch strings.ToLower(platformType) {
	case "added", "new":
		return "added"
	case "deleted", "removed":
		return "deleted"
	case "renamed":
		return "renamed"
	default:
		return "modified"
	}
}
