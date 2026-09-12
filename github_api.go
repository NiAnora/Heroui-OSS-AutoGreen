package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// githubAPIRepo 对应 GitHub API 的响应结构（下划线命名，仅保留需要的字段）。
type githubAPIRepo struct {
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// githubRepo 是返回给前端的结构（驼峰命名，与其余接口保持一致）。
type githubRepo struct {
	FullName      string `json:"fullName"`
	HTMLURL       string `json:"htmlUrl"`
	CloneURL      string `json:"cloneUrl"`
	DefaultBranch string `json:"defaultBranch"`
	Private       bool   `json:"private"`
}

// listGithubRepos 使用当前 token 拉取登录用户名下拥有的仓库列表（分页获取）。
// 仅取本人拥有的仓库（affiliation=owner），不含仅作为协作者参与的仓库。
func listGithubRepos(token string) ([]githubRepo, error) {
	if token == "" {
		return nil, fmt.Errorf("尚未配置 token，请先在设置页完成授权")
	}

	out := []githubRepo{}
	page := 1
	for {
		url := fmt.Sprintf("https://api.github.com/user/repos?per_page=100&page=%d&sort=updated&affiliation=owner", page)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("拉取仓库列表失败（HTTP %d）：请确认 token 有效且有 repo 权限", resp.StatusCode)
		}

		var batch []githubAPIRepo
		if err := json.NewDecoder(resp.Body).Decode(&batch); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, r := range batch {
			out = append(out, githubRepo{
				FullName:      r.FullName,
				HTMLURL:       r.HTMLURL,
				CloneURL:      r.CloneURL,
				DefaultBranch: r.DefaultBranch,
				Private:       r.Private,
			})
		}
		if len(batch) < 100 {
			break
		}
		page++
	}
	return out, nil
}
