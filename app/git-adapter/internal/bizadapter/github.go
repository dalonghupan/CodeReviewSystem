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

// githubProvider GitHub 适配器（REST API v3）
type githubProvider struct {
	cfg    conf.PlatformOAuth
	client *httpClient
}

// NewGitHubProvider 创建 GitHub 适配器
func NewGitHubProvider(cfg conf.PlatformOAuth) GitProvider {
	return &githubProvider{cfg: cfg, client: newHTTPClient(10 * time.Second)}
}

func (p *githubProvider) Platform() string { return PlatformGitHub }

// apiBase API 域名（github.com → api.github.com；企业版 GHE 为 {host}/api/v3）
func (p *githubProvider) apiBase() string {
	base := strings.TrimRight(p.cfg.BaseURL, "/")
	if strings.Contains(base, "github.com") {
		return "https://api.github.com"
	}
	return base + "/api/v3"
}

// BuildAuthURL 构造授权跳转URL（scope: repo read:user）
func (p *githubProvider) BuildAuthURL(state string) string {
	return fmt.Sprintf("%s/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=repo+read:user",
		p.cfg.BaseURL, p.cfg.ClientID, url.QueryEscape(p.cfg.RedirectURI), state)
}

// ExchangeToken 授权码换 Token（GitHub 标准 OAuth 无 refresh_token，Expiring Token 模式才有）
func (p *githubProvider) ExchangeToken(ctx context.Context, code string) (*TokenPair, error) {
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	err := p.client.doJSON(ctx, "POST", p.cfg.BaseURL+"/login/oauth/access_token", "", postForm(map[string]string{
		"client_id":     p.cfg.ClientID,
		"client_secret": p.cfg.ClientSecret,
		"code":          code,
		"redirect_uri":  p.cfg.RedirectURI,
	}), &resp)
	if err != nil {
		return nil, err
	}
	if resp.ExpiresIn == 0 {
		resp.ExpiresIn = 10 * 365 * 24 * 3600 // 非过期型Token按10年处理
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

// RefreshToken 刷新 Token（仅 Expiring User Token 模式支持）
func (p *githubProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	if refreshToken == "" {
		return nil, errcode.ErrGitTokenExpired.WithDetail("GitHub非过期型Token无需刷新")
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	err := p.client.doJSON(ctx, "POST", p.cfg.BaseURL+"/login/oauth/access_token", "", postForm(map[string]string{
		"client_id":     p.cfg.ClientID,
		"client_secret": p.cfg.ClientSecret,
		"refresh_token": refreshToken,
		"grant_type":    "refresh_token",
	}), &resp)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: resp.AccessToken, RefreshToken: resp.RefreshToken, ExpiresIn: resp.ExpiresIn}, nil
}

func (p *githubProvider) fetchUser(ctx context.Context, accessToken string) (*PlatformUser, error) {
	var u struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := p.client.doJSON(ctx, "GET", p.apiBase()+"/user", "Bearer "+accessToken, nil, &u); err != nil {
		return nil, err
	}
	return &PlatformUser{ID: fmt.Sprint(u.ID), Username: u.Login, AvatarURL: u.AvatarURL}, nil
}

// ListRepos 拉取授权用户仓库
func (p *githubProvider) ListRepos(ctx context.Context, accessToken, keyword string, page, pageSize int) ([]*RemoteRepo, int, error) {
	reqURL := fmt.Sprintf("%s/user/repos?page=%d&per_page=%d&sort=updated&affiliation=owner,collaborator",
		p.apiBase(), page, pageSize)
	var items []struct {
		ID            int64  `json:"id"`
		FullName      string `json:"full_name"`
		Description   string `json:"description"`
		DefaultBranch string `json:"default_branch"`
		CloneURL      string `json:"clone_url"`
		HTMLURL       string `json:"html_url"`
		Private       bool   `json:"private"`
		UpdatedAt     string `json:"updated_at"`
	}
	if err := p.client.doJSON(ctx, "GET", reqURL, "Bearer "+accessToken, nil, &items); err != nil {
		return nil, 0, err
	}

	repos := make([]*RemoteRepo, 0, len(items))
	for _, it := range items {
		if keyword != "" && !strings.Contains(strings.ToLower(it.FullName), strings.ToLower(keyword)) {
			continue
		}
		repos = append(repos, &RemoteRepo{
			PlatformRepoID: it.FullName, // GitHub API 以 owner/repo 作为路径标识
			FullName:       it.FullName,
			Description:    it.Description,
			DefaultBranch:  it.DefaultBranch,
			CloneURL:       it.CloneURL,
			WebURL:         it.HTMLURL,
			IsPrivate:      it.Private,
			UpdatedAt:      parseTime(it.UpdatedAt),
		})
	}
	return repos, len(repos), nil
}

// GetMergeRequest 获取 PR 信息
func (p *githubProvider) GetMergeRequest(ctx context.Context, accessToken, platformRepoID, mrID string) (*MergeRequest, error) {
	var pr struct {
		Number   int64  `json:"number"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		State    string `json:"state"` // open / closed
		Merged   bool   `json:"merged"`
		HTMLURL  string `json:"html_url"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		Head     struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		User struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
		Commits      int `json:"commits"`
		ChangedFiles int `json:"changed_files"`
	}
	reqURL := fmt.Sprintf("%s/repos/%s/pulls/%s", p.apiBase(), platformRepoID, mrID)
	if err := p.client.doJSON(ctx, "GET", reqURL, "Bearer "+accessToken, nil, &pr); err != nil {
		return nil, err
	}

	status := pr.State
	if pr.Merged {
		status = "merged"
	}
	return &MergeRequest{
		MRID:             mrID,
		Title:            pr.Title,
		Description:      pr.Body,
		SourceBranch:     pr.Head.Ref,
		TargetBranch:     pr.Base.Ref,
		AuthorName:       pr.User.Login,
		AuthorAvatar:     pr.User.AvatarURL,
		Status:           status,
		CommitCount:      pr.Commits,
		ChangedFileCount: pr.ChangedFiles,
		LatestCommitHash: pr.Head.SHA,
		WebURL:           pr.HTMLURL,
		CreatedAt:        parseTime(pr.CreatedAt),
		UpdatedAt:        parseTime(pr.UpdatedAt),
	}, nil
}

// ListMRFiles 获取 PR 变更文件
func (p *githubProvider) ListMRFiles(ctx context.Context, accessToken, platformRepoID, mrID string) ([]*ChangedFile, error) {
	var items []struct {
		FileName  string `json:"filename"`
		PrevName  string `json:"previous_filename"`
		Status    string `json:"status"` // added / removed / modified / renamed
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Patch     string `json:"patch"`
	}
	reqURL := fmt.Sprintf("%s/repos/%s/pulls/%s/files?per_page=100", p.apiBase(), platformRepoID, mrID)
	if err := p.client.doJSON(ctx, "GET", reqURL, "Bearer "+accessToken, nil, &items); err != nil {
		return nil, err
	}

	files := make([]*ChangedFile, 0, len(items))
	for _, it := range items {
		ct := it.Status
		if ct == "removed" {
			ct = "deleted"
		}
		files = append(files, &ChangedFile{
			FilePath:   it.FileName,
			OldPath:    it.PrevName,
			ChangeType: ct,
			Additions:  it.Additions,
			Deletions:  it.Deletions,
		})
	}
	return files, nil
}

// GetFileDiff 获取指定文件 patch
func (p *githubProvider) GetFileDiff(ctx context.Context, accessToken, platformRepoID, mrID, filePath string) (string, error) {
	var items []struct {
		FileName string `json:"filename"`
		Patch    string `json:"patch"`
	}
	reqURL := fmt.Sprintf("%s/repos/%s/pulls/%s/files?per_page=100", p.apiBase(), platformRepoID, mrID)
	if err := p.client.doJSON(ctx, "GET", reqURL, "Bearer "+accessToken, nil, &items); err != nil {
		return "", err
	}
	for _, it := range items {
		if it.FileName == filePath {
			return it.Patch, nil
		}
	}
	return "", errcode.ErrGitDiffParseFailed.WithDetail("未找到文件: " + filePath)
}
