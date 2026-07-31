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

// giteeProvider Gitee 适配器（API v5）
type giteeProvider struct {
	cfg    conf.PlatformOAuth
	client *httpClient
}

// NewGiteeProvider 创建 Gitee 适配器
func NewGiteeProvider(cfg conf.PlatformOAuth) GitProvider {
	return &giteeProvider{cfg: cfg, client: newHTTPClient(10 * time.Second)}
}

func (p *giteeProvider) Platform() string { return PlatformGitee }

func (p *giteeProvider) apiBase() string { return strings.TrimRight(p.cfg.BaseURL, "/") + "/api/v5" }

// BuildAuthURL 构造授权跳转URL（scope: user_info projects pull_requests）
func (p *giteeProvider) BuildAuthURL(state string) string {
	return fmt.Sprintf("%s/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&state=%s&scope=user_info+projects+pull_requests",
		p.cfg.BaseURL, p.cfg.ClientID, url.QueryEscape(p.cfg.RedirectURI), state)
}

// ExchangeToken 授权码换 Token
func (p *giteeProvider) ExchangeToken(ctx context.Context, code string) (*TokenPair, error) {
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
func (p *giteeProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	err := p.client.doJSON(ctx, "POST", p.cfg.BaseURL+"/oauth/token", "", postForm(map[string]string{
		"refresh_token": refreshToken,
		"grant_type":    "refresh_token",
	}), &resp)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: resp.AccessToken, RefreshToken: resp.RefreshToken, ExpiresIn: resp.ExpiresIn}, nil
}

func (p *giteeProvider) fetchUser(ctx context.Context, accessToken string) (*PlatformUser, error) {
	var u struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	reqURL := fmt.Sprintf("%s/user?access_token=%s", p.apiBase(), accessToken)
	if err := p.client.doJSON(ctx, "GET", reqURL, "", nil, &u); err != nil {
		return nil, err
	}
	return &PlatformUser{ID: fmt.Sprint(u.ID), Username: u.Login, AvatarURL: u.AvatarURL}, nil
}

// ListRepos 拉取授权用户仓库
func (p *giteeProvider) ListRepos(ctx context.Context, accessToken, keyword string, page, pageSize int) ([]*RemoteRepo, int, error) {
	reqURL := fmt.Sprintf("%s/user/repos?access_token=%s&q=%s&page=%d&per_page=%d&sort=updated",
		p.apiBase(), accessToken, url.QueryEscape(keyword), page, pageSize)
	var items []struct {
		ID            int64  `json:"id"`
		FullName      string `json:"full_name"`
		HumanName     string `json:"human_name"`
		Description   string `json:"description"`
		DefaultBranch string `json:"default_branch"`
		HTMLURL       string `json:"html_url"`
		Private       bool   `json:"private"`
		UpdatedAt     string `json:"updated_at"`
	}
	if err := p.client.doJSON(ctx, "GET", reqURL, "", nil, &items); err != nil {
		return nil, 0, err
	}

	repos := make([]*RemoteRepo, 0, len(items))
	for _, it := range items {
		repos = append(repos, &RemoteRepo{
			PlatformRepoID: it.FullName, // Gitee API 以 owner/repo 作为路径标识
			FullName:       it.FullName,
			Description:    it.Description,
			DefaultBranch:  it.DefaultBranch,
			CloneURL:       it.HTMLURL + ".git",
			WebURL:         it.HTMLURL,
			IsPrivate:      it.Private,
			UpdatedAt:      parseTime(it.UpdatedAt),
		})
	}
	return repos, len(repos), nil
}

// GetMergeRequest 获取 PR 信息（Gitee 称 Pull Request）
func (p *giteeProvider) GetMergeRequest(ctx context.Context, accessToken, platformRepoID, mrID string) (*MergeRequest, error) {
	var pr struct {
		Number   int64  `json:"number"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		State    string `json:"state"` // open / closed / merged
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
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
	}
	reqURL := fmt.Sprintf("%s/repos/%s/pulls/%s?access_token=%s",
		p.apiBase(), platformRepoID, mrID, accessToken)
	if err := p.client.doJSON(ctx, "GET", reqURL, "", nil, &pr); err != nil {
		return nil, err
	}
	return &MergeRequest{
		MRID:             mrID,
		Title:            pr.Title,
		Description:      pr.Body,
		SourceBranch:     pr.Head.Ref,
		TargetBranch:     pr.Base.Ref,
		AuthorName:       pr.User.Name,
		AuthorAvatar:     pr.User.AvatarURL,
		Status:           pr.State,
		LatestCommitHash: pr.Head.SHA,
		WebURL:           pr.HTMLURL,
		CreatedAt:        parseTime(pr.CreatedAt),
		UpdatedAt:        parseTime(pr.UpdatedAt),
	}, nil
}

// ListMRFiles 获取 PR 变更文件
func (p *giteeProvider) ListMRFiles(ctx context.Context, accessToken, platformRepoID, mrID string) ([]*ChangedFile, error) {
	var items []struct {
		FileName  string `json:"filename"`
		Status    string `json:"status"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Patch     string `json:"patch"`
	}
	reqURL := fmt.Sprintf("%s/repos/%s/pulls/%s/files?access_token=%s&per_page=100",
		p.apiBase(), platformRepoID, mrID, accessToken)
	if err := p.client.doJSON(ctx, "GET", reqURL, "", nil, &items); err != nil {
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
			ChangeType: ct,
			Additions:  it.Additions,
			Deletions:  it.Deletions,
		})
	}
	return files, nil
}

// GetFileDiff 获取指定文件 patch
func (p *giteeProvider) GetFileDiff(ctx context.Context, accessToken, platformRepoID, mrID, filePath string) (string, error) {
	var items []struct {
		FileName string `json:"filename"`
		Patch    string `json:"patch"`
	}
	reqURL := fmt.Sprintf("%s/repos/%s/pulls/%s/files?access_token=%s&per_page=100",
		p.apiBase(), platformRepoID, mrID, accessToken)
	if err := p.client.doJSON(ctx, "GET", reqURL, "", nil, &items); err != nil {
		return "", err
	}
	for _, it := range items {
		if it.FileName == filePath {
			return it.Patch, nil
		}
	}
	return "", errcode.ErrGitDiffParseFailed.WithDetail("未找到文件: " + filePath)
}
