package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Repo 表示一个受管理的仓库。提交计划（模式与时间范围）为全局配置，不在此处。
type Repo struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	RepoURL     string     `json:"repoUrl"`
	Branch      string     `json:"branch"`
	UserName    string     `json:"userName"`
	UserEmail   string     `json:"userEmail"`
	CommitMsg   string     `json:"commitMsg"`
	Enabled     bool       `json:"enabled"`
	WorkDir     string     `json:"workDir"`
	LastCommit  *time.Time `json:"lastCommit,omitempty"`
	NextCommit  *time.Time `json:"nextCommit,omitempty"`
	CommitCount int        `json:"commitCount"`
	LastError   string     `json:"lastError,omitempty"`

	// 以下为调度运行时状态，用于保证「每天最多提交一次」。
	PlanWeek string `json:"planWeek,omitempty"` // weekly：已抽取计划所属周的周一日期
	PlanDays []int  `json:"planDays,omitempty"` // weekly：本周选中的天偏移（0=周一）
	PlanDate string `json:"planDate,omitempty"` // 已规划提交的日期（YYYY-MM-DD）
}

// CommitRecord 记录一次提交结果，用于历史与统计。
type CommitRecord struct {
	RepoID  string    `json:"repoId"`
	Repo    string    `json:"repo"`
	Hash    string    `json:"hash"`
	Time    time.Time `json:"time"`
	Success bool      `json:"success"`
	Message string    `json:"message"`
}

type storeData struct {
	Token    string          `json:"token"`
	ClientID string          `json:"clientId"`
	Schedule *ScheduleConfig `json:"schedule,omitempty"`
	Repos    []*Repo         `json:"repos"`
	Log      []*CommitRecord `json:"log"`
}

// Store 负责配置与状态的线程安全读写及 JSON 持久化。
type Store struct {
	mu   sync.RWMutex
	path string
	data storeData
}

// LoadStore 从磁盘加载持久化数据；文件不存在时初始化并回退到环境变量/gh CLI 的 token。
func LoadStore(path string) (*Store, error) {
	s := &Store{path: path, data: storeData{Repos: []*Repo{}, Log: []*CommitRecord{}}}

	b, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		s.data.Token = getToken()
		cfg := defaultScheduleConfig()
		s.data.Schedule = &cfg
		if err := s.save(); err != nil {
			return nil, err
		}
		return s, nil
	}

	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, err
	}
	if s.data.Repos == nil {
		s.data.Repos = []*Repo{}
	}
	if s.data.Log == nil {
		s.data.Log = []*CommitRecord{}
	}
	// 兼容早期数据：缺少全局提交计划时补默认值。
	if s.data.Schedule == nil {
		cfg := defaultScheduleConfig()
		s.data.Schedule = &cfg
	}
	normalizeScheduleConfig(s.data.Schedule)
	return s, nil
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}

// GetToken 返回当前 token。
func (s *Store) GetToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Token
}

// SetToken 更新 token 并持久化。
func (s *Store) SetToken(t string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Token = t
	return s.save()
}

// GetClientID 返回 GitHub OAuth App 的 client_id。
func (s *Store) GetClientID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.ClientID
}

// GetSchedule 返回全局提交计划配置副本。
func (s *Store) GetSchedule() ScheduleConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.Schedule == nil {
		return defaultScheduleConfig()
	}
	return *s.data.Schedule
}

// SetSchedule 更新全局提交计划配置并持久化。
func (s *Store) SetSchedule(cfg ScheduleConfig) (ScheduleConfig, error) {
	normalizeScheduleConfig(&cfg)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Schedule = &cfg
	return cfg, s.save()
}

// SetClientID 更新 client_id 并持久化。
func (s *Store) SetClientID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.ClientID = id
	return s.save()
}

// ListRepos 返回所有仓库的副本。
func (s *Store) ListRepos() []*Repo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Repo, 0, len(s.data.Repos))
	for _, r := range s.data.Repos {
		out = append(out, cloneRepo(r))
	}
	return out
}

// GetRepo 返回指定仓库副本；不存在返回 nil。
func (s *Store) GetRepo(id string) *Repo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.data.Repos {
		if r.ID == id {
			return cloneRepo(r)
		}
	}
	return nil
}

// AddRepo 新增仓库，补齐默认值并持久化。
func (s *Store) AddRepo(r *Repo) (*Repo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.ID == "" {
		r.ID = genID()
	}
	if r.WorkDir == "" {
		r.WorkDir = filepath.Join("repos", r.ID)
	}
	if r.Branch == "" {
		r.Branch = "main"
	}
	if r.UserName == "" {
		r.UserName = "AutoGreen"
	}
	if r.UserEmail == "" {
		r.UserEmail = "autogreen@users.noreply.github.com"
	}
	if r.CommitMsg == "" {
		r.CommitMsg = "chore: auto commit"
	}

	s.data.Repos = append(s.data.Repos, r)
	if err := s.save(); err != nil {
		return nil, err
	}
	return cloneRepo(r), nil
}

// UpdateRepo 更新仓库的可配置字段，保留运行时状态，并持久化。
func (s *Store) UpdateRepo(id string, u *Repo) (*Repo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range s.data.Repos {
		if r.ID != id {
			continue
		}
		r.Name = u.Name
		r.RepoURL = u.RepoURL
		r.Branch = u.Branch
		r.UserName = u.UserName
		r.UserEmail = u.UserEmail
		r.CommitMsg = u.CommitMsg
		r.Enabled = u.Enabled
		if u.WorkDir != "" {
			r.WorkDir = u.WorkDir
		}
		if err := s.save(); err != nil {
			return nil, err
		}
		return cloneRepo(r), nil
	}
	return nil, os.ErrNotExist
}

// DeleteRepo 删除仓库及其本地目录。
func (s *Store) DeleteRepo(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.data.Repos {
		if r.ID != id {
			continue
		}
		workDir := r.WorkDir
		s.data.Repos = append(s.data.Repos[:i], s.data.Repos[i+1:]...)
		if err := s.save(); err != nil {
			return err
		}
		if workDir != "" {
			_ = os.RemoveAll(workDir)
		}
		return nil
	}
	return os.ErrNotExist
}

// SetPlan 记录下一次提交计划（时刻与去重状态）并持久化。
func (s *Store) SetPlan(id string, p schedulePlan) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.data.Repos {
		if r.ID != id {
			continue
		}
		next := p.Next
		r.NextCommit = &next
		r.PlanDate = p.PlanDate
		r.PlanWeek = p.PlanWeek
		r.PlanDays = append([]int(nil), p.PlanDays...)
		_ = s.save()
		return
	}
}

// ClearNext 清除下一次提交时刻，使配置变更后立即重新计算计划。
func (s *Store) ClearNext(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.data.Repos {
		if r.ID == id {
			r.NextCommit = nil
			return
		}
	}
}

// ClearAllNext 清除所有仓库的下次提交时刻，用于全局计划变更后统一重算。
func (s *Store) ClearAllNext() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.data.Repos {
		r.NextCommit = nil
	}
}

// RecordCommit 追加一条提交记录并更新仓库计数；日志保留最近 1000 条。
func (s *Store) RecordCommit(id string, rec *CommitRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range s.data.Repos {
		if r.ID == id {
			if rec.Success {
				r.CommitCount++
				r.LastError = ""
				now := time.Now()
				r.LastCommit = &now
			} else {
				r.LastError = rec.Message
			}
			break
		}
	}
	s.data.Log = append(s.data.Log, rec)
	if len(s.data.Log) > 1000 {
		s.data.Log = s.data.Log[len(s.data.Log)-1000:]
	}
	_ = s.save()
}

// CommitLog 返回最近 limit 条提交记录（倒序）。
func (s *Store) CommitLog(limit int) []*CommitRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(s.data.Log)
	if limit <= 0 || limit > n {
		limit = n
	}
	out := make([]*CommitRecord, 0, limit)
	for i := n - 1; i >= n-limit; i-- {
		out = append(out, s.data.Log[i])
	}
	return out
}

// DailyStats 返回最近 days 天每日成功提交次数（按本地日期聚合）。
func (s *Store) DailyStats(days int) map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]int)
	cutoff := time.Now().AddDate(0, 0, -days+1)
	for _, rec := range s.data.Log {
		if !rec.Success || rec.Time.Before(cutoff) {
			continue
		}
		key := rec.Time.Format("2006-01-02")
		out[key]++
	}
	return out
}

func cloneRepo(r *Repo) *Repo {
	c := *r
	if r.LastCommit != nil {
		t := *r.LastCommit
		c.LastCommit = &t
	}
	if r.NextCommit != nil {
		t := *r.NextCommit
		c.NextCommit = &t
	}
	if r.PlanDays != nil {
		c.PlanDays = append([]int(nil), r.PlanDays...)
	}
	return &c
}

// genID 使用真随机源生成 16 位十六进制 ID。
func genID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// getToken 依次从环境变量与 gh CLI 读取 token，作为首次初始化的回退。
func getToken() string {
	if v := os.Getenv("AUTOGREEN_TOKEN"); v != "" {
		return v
	}
	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		return v
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
