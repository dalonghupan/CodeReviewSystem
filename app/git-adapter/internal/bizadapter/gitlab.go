package bizadapter

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"cr-system/app/git-adapter/internal/conf"
	"cr-system/pkg/errcode"
)

// gitlabProvider GitLab 适配器（GitLab REST API v4，兼容私有化部署）
type gitlabProvider struct {
	cfg    conf.PlatformOAuth
	client *httpClient
}

// NewGitLabProvider 创建 GitLab 适配器
func NewGitLabProvider(cfg conf.PlatformOAuth) GitProvider {
	return &gitlabProvider{cfg: cfg, client: newHTTPClient(10 * time.Second)}
}

func (p *gitlabProvider) Platform() string { return PlatformGitLab }

func (p *gitlabProvider) apiBase() string { return strings.TrimRight(p.cfg.BaseURL, "/") + "/api/v4" }

// BuildAuthURL 构造授权跳转URL（scope: api read_user）
func (p *gitlabProvider) BuildAuthURL(state string) string {
	return fmt.Sprintf("%s/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&state=%s&scope=api+read_user",
		p.cfg.BaseURL, p.cfg.ClientID, url.QueryEscape(p.cfg.RedirectURI), state)
}

// ExchangeToken 授权码换 Token
func (p *gitlabProvider) ExchangeToken(ctx context.Context, code string) (*TokenPair, error) {
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	err := p.client.doJSON(ctx, "POST", p.cfg.BaseURL+"/oauth/token", "", postForm(map[string]string{
		"client_id":     p.cfg.ClientID,
		"client_secret": p.cfg.ClientSecret,
		"code":          code,
		"grant_type":    "authorization_code",
		"redirect_uri":  p.cfg.RedirectURI,
	}), &resp)
	if err != nil {
		return nil, err
	}

	user, err := p.fetchUser(ctx, resp.AccessToken)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpiresIn,
		PlatformUser: *user,
	}, nil
}

// RefreshToken 刷新 Token
func (p *gitlabProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	err := p.client.doJSON(ctx, "POST", p.cfg.BaseURL+"/oauth/token", "", postForm(map[string]string{
		"client_id":     p.cfg.ClientID,
		"client_secret": p.cfg.ClientSecret,
		"refresh_token": refreshToken,
		"grant_type":    "refresh_token",
	}), &resp)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpiresIn,
	}, nil
}

func (p *gitlabProvider) fetchUser(ctx context.Context, accessToken string) (*PlatformUser, error) {
	var u struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := p.client.doJSON(ctx, "GET", p.apiBase()+"/user", "Bearer "+accessToken, nil, &u); err != nil {
		return nil, err
	}
	return &PlatformUser{ID: fmt.Sprint(u.ID), Username: u.Username, AvatarURL: u.AvatarURL}, nil
}

// ListRepos 拉取成员仓库（membership=true）
func (p *gitlabProvider) ListRepos(ctx context.Context, accessToken, keyword string, page, pageSize int) ([]*RemoteRepo, int, error) {
	reqURL := fmt.Sprintf("%s/projects?membership=true&search=%s&page=%d&per_page=%d&order_by=updated_at",
		p.apiBase(), url.QueryEscape(keyword), page, pageSize)
	var items []struct {
		ID            int64  `json:"id"`
		PathWithNS    string `json:"path_with_namespace"`
		Description   string `json:"description"`
		DefaultBranch string `json:"default_branch"`
		HTTPURL       string `json:"http_url_to_repo"`
		WebURL        string `json:"web_url"`
		Visibility    string `json:"visibility"`
		UpdatedAt     string `json:"last_activity_at"`
	}
	if err := p.client.doJSON(ctx, "GET", reqURL, "Bearer "+accessToken, nil, &items); err != nil {
		return nil, 0, err
	}

	repos := make([]*RemoteRepo, 0, len(items))
	for _, it := range items {
		repos = append(repos, &RemoteRepo{
			PlatformRepoID: fmt.Sprint(it.ID),
			FullName:       it.PathWithNS,
			Description:    it.Description,
			DefaultBranch:  it.DefaultBranch,
			CloneURL:       it.HTTPURL,
			WebURL:         it.WebURL,
			IsPrivate:      it.Visibility == "private",
			UpdatedAt:      parseTime(it.UpdatedAt),
		})
	}
	// GitLab 分页总数在 X-Total 头，基座未透传，列表场景以前端翻页为准
	return repos, len(repos), nil
}

// GetMergeRequest 获取 MR 信息（mrID 为项目内 IID）
func (p *gitlabProvider) GetMergeRequest(ctx context.Context, accessToken, platformRepoID, mrID string) (*MergeRequest, error) {
	var mr struct {
		IID          int64  `json:"iid"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		State        string `json:"state"` // opened / merged / closed
		SHA          string `json:"sha"`
		WebURL       string `json:"web_url"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
		Author       struct {
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"author"`
	}
	reqURL := fmt.Sprintf("%s/projects/%s/merge_requests/%s",
		p.apiBase(), url.PathEscape(platformRepoID), mrID)
	if err := p.client.doJSON(ctx, "GET", reqURL, "Bearer "+accessToken, nil, &mr); err != nil {
		return nil, err
	}
	return &MergeRequest{
		MRID:             mrID,
		Title:            mr.Title,
		Description:      mr.Description,
		SourceBranch:     mr.SourceBranch,
		TargetBranch:     mr.TargetBranch,
		AuthorName:       mr.Author.Name,
		AuthorAvatar:     mr.Author.AvatarURL,
		Status:           mr.State,
		LatestCommitHash: mr.SHA,
		WebURL:           mr.WebURL,
		CreatedAt:        parseTime(mr.CreatedAt),
		UpdatedAt:        parseTime(mr.UpdatedAt),
	}, nil
}

// ListMRFiles 获取 MR 变更文件（/diffs 接口）
func (p *gitlabProvider) ListMRFiles(ctx context.Context, accessToken, platformRepoID, mrID string) ([]*ChangedFile, error) {
	var diffs []struct {
		OldPath   string `json:"old_path"`
		NewPath   string `json:"new_path"`
		NewFile   bool   `json:"new_file"`
		Renamed   bool   `json:"renamed_file"`
		Deleted   bool   `json:"deleted_file"`
		Diff      string `json:"diff"`
	}
	reqURL := fmt.Sprintf("%s/projects/%s/merge_requests/%s/diffs?per_page=100",
		p.apiBase(), url.PathEscape(platformRepoID), mrID)
	if err := p.client.doJSON(ctx, "GET", reqURL, "Bearer "+accessToken, nil, &diffs); err != nil {
		return nil, err
	}

	files := make([]*ChangedFile, 0, len(diffs))
	for _, d := range diffs {
		changeType := "modified"
		switch {
		case d.NewFile:
			changeType = "added"
		case d.Deleted:
			changeType = "deleted"
		case d.Renamed:
			changeType = "renamed"
		}
		add, del := countDiffLines(d.Diff)
		files = append(files, &ChangedFile{
			FilePath:   d.NewPath,
			OldPath:    d.OldPath,
			ChangeType: changeType,
			Additions:  add,
			Deletions:  del,
		})
	}
	return files, nil
}

// GetFileDiff 获取指定文件原始 diff 文本（复用 diffs 接口按路径过滤）
func (p *gitlabProvider) GetFileDiff(ctx context.Context, accessToken, platformRepoID, mrID, filePath string) (string, error) {
	var diffs []struct {
		NewPath string `json:"new_path"`
		Diff    string `json:"diff"`
	}
	reqURL := fmt.Sprintf("%s/projects/%s/merge_requests/%s/diffs?per_page=100",
		p.apiBase(), url.PathEscape(platformRepoID), mrID)
	if err := p.client.doJSON(ctx, "GET", reqURL, "Bearer "+accessToken, nil, &diffs); err != nil {
		return "", err
	}
	for _, d := range diffs {
		if d.NewPath == filePath {
			return d.Diff, nil
		}
	}
	return "", errcode.ErrGitDiffParseFailed.WithDetail("未找到文件: " + filePath)
}

// countDiffLines 统计 unified diff 增删行数
func countDiffLines(diff string) (added, deleted int) {
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deleted++
		}
	}
	return
}

// parseTime 平台时间串解析（ISO8601），失败返回零值
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
