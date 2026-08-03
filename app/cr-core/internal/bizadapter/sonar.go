// Package bizadapter cr-core 第三方服务适配层
// SonarQube HTTP API 客户端封装（LLD §4.3-1）
package bizadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"cr-system/pkg/errcode"
)

// SonarClient SonarQube API 客户端
type SonarClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// SonarIssue SonarQube 问题条目
type SonarIssue struct {
	Key       string `json:"key"`
	Severity  string `json:"severity"`  // BLOCKER / CRITICAL / MAJOR / MINOR / INFO
	Type      string `json:"type"`      // BUG / VULNERABILITY / CODE_SMELL
	Component string `json:"component"` // 含文件路径
	Line      int    `json:"line"`
	Message   string `json:"message"`
	Rule      string `json:"rule"`
	Status    string `json:"status"` // OPEN / CONFIRMED / RESOLVED / CLOSED
}

// SonarMeasures SonarQube 指标数据
type SonarMeasures struct {
	Coverage       float64 `json:"coverage"`
	Duplications   float64 `json:"duplications"`
	Bugs           int64   `json:"bugs"`
	Vulnerabilities int64  `json:"vulnerabilities"`
	CodeSmells     int64   `json:"code_smells"`
	TechDebt       string  `json:"tech_debt"`
}

// SonarQualityGate SonarQube 质量门状态
type SonarQualityGate struct {
	Status string `json:"status"` // OK / ERROR
}

// NewSonarClient 创建 SonarQube 客户端
func NewSonarClient(baseURL, token string, timeout time.Duration) *SonarClient {
	return &SonarClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetIssues 获取项目 Sonar 问题列表
func (c *SonarClient) GetIssues(ctx context.Context, projectKey string) ([]SonarIssue, error) {
	u := fmt.Sprintf("%s/api/issues/search?componentKeys=%s&ps=500", c.baseURL, url.QueryEscape(projectKey))
	body, err := c.doGet(ctx, u)
	if err != nil {
		return nil, err
	}

	var result struct {
		Issues []SonarIssue `json:"issues"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errcode.ErrSonarConnectFailed.WithDetail("解析Sonar问题列表失败: " + err.Error())
	}
	return result.Issues, nil
}

// GetMeasures 获取项目 Sonar 指标
func (c *SonarClient) GetMeasures(ctx context.Context, projectKey string) (*SonarMeasures, error) {
	metricKeys := "coverage,duplicated_lines_density,bugs,vulnerabilities,code_smells,sqale_index"
	u := fmt.Sprintf("%s/api/measures/component?component=%s&metricKeys=%s",
		c.baseURL, url.QueryEscape(projectKey), metricKeys)
	body, err := c.doGet(ctx, u)
	if err != nil {
		return nil, err
	}

	var result struct {
		Component struct {
			Measures []struct {
				Metric string  `json:"metric"`
				Value  float64 `json:"value"`
			} `json:"measures"`
		} `json:"component"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errcode.ErrSonarConnectFailed.WithDetail("解析Sonar指标失败: " + err.Error())
	}

	measures := &SonarMeasures{}
	for _, m := range result.Component.Measures {
		switch m.Metric {
		case "coverage":
			measures.Coverage = m.Value
		case "duplicated_lines_density":
			measures.Duplications = m.Value
		case "bugs":
			measures.Bugs = int64(m.Value)
		case "vulnerabilities":
			measures.Vulnerabilities = int64(m.Value)
		case "code_smells":
			measures.CodeSmells = int64(m.Value)
		case "sqale_index":
			measures.TechDebt = fmt.Sprintf("%.0fmin", m.Value)
		}
	}
	return measures, nil
}

// GetQualityGateStatus 获取质量门状态
func (c *SonarClient) GetQualityGateStatus(ctx context.Context, projectKey string) (string, error) {
	u := fmt.Sprintf("%s/api/qualitygates/project_status?projectKey=%s",
		c.baseURL, url.QueryEscape(projectKey))
	body, err := c.doGet(ctx, u)
	if err != nil {
		return "", err
	}

	var result struct {
		ProjectStatus struct {
			Status string `json:"status"`
		} `json:"projectStatus"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", errcode.ErrSonarConnectFailed.WithDetail("解析Sonar质量门状态失败: " + err.Error())
	}
	return result.ProjectStatus.Status, nil
}

// doGet 执行 HTTP GET 请求
func (c *SonarClient) doGet(ctx context.Context, urlStr string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, errcode.ErrSonarConnectFailed.WithDetail("创建Sonar请求失败: " + err.Error())
	}
	req.SetBasicAuth(c.token, "")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errcode.ErrSonarConnectFailed.WithDetail("Sonar请求失败: " + err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errcode.ErrSonarConnectFailed.WithDetail("读取Sonar响应失败: " + err.Error())
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errcode.ErrSonarConnectFailed.WithDetail(
			fmt.Sprintf("Sonar返回状态码 %d: %s", resp.StatusCode, string(body)))
	}
	return body, nil
}
