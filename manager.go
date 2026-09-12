package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var errMissingToken = errors.New("缺少 token，请先在设置中配置")

// Manager 负责为每个启用的仓库运行独立的调度 goroutine，
// 并支持暂停/恢复、手动触发。
type Manager struct {
	store    *Store
	mu       sync.Mutex
	cancels  map[string]context.CancelFunc
	triggers map[string]chan struct{}
}

func NewManager(s *Store) *Manager {
	return &Manager{
		store:    s,
		cancels:  make(map[string]context.CancelFunc),
		triggers: make(map[string]chan struct{}),
	}
}

// Start 启动所有已启用仓库的调度循环。
func (m *Manager) Start() {
	m.reconcile()
}

// Reload 在仓库配置变更后重启其调度循环（使新频率等配置生效）。
func (m *Manager) Reload(id string) {
	m.mu.Lock()
	if cancel, ok := m.cancels[id]; ok {
		cancel()
		delete(m.cancels, id)
		delete(m.triggers, id)
	}
	m.mu.Unlock()
	// 丢弃旧的计划时刻，让新配置立即重新计算。
	m.store.ClearNext(id)
	m.reconcile()
}

// Trigger 立即触发一次提交（不影响原有调度节奏）；若仓库未在运行（已暂停），则直接执行一次。
func (m *Manager) Trigger(id string) {
	m.mu.Lock()
	ch, ok := m.triggers[id]
	m.mu.Unlock()
	if ok {
		select {
		case ch <- struct{}{}:
		default:
		}
		return
	}
	go m.doCommit(id)
}

// ReloadAll 在全局提交计划变更后重启所有调度循环，使新配置立即生效。
func (m *Manager) ReloadAll() {
	m.mu.Lock()
	for id, cancel := range m.cancels {
		cancel()
		delete(m.cancels, id)
		delete(m.triggers, id)
	}
	m.mu.Unlock()
	m.store.ClearAllNext()
	m.reconcile()
}

// StopAll 停止所有调度循环。
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cancel := range m.cancels {
		cancel()
		delete(m.cancels, id)
		delete(m.triggers, id)
	}
}

// reconcile 根据当前仓库状态，启动缺失的、停用不再需要的调度循环。
func (m *Manager) reconcile() {
	m.mu.Lock()
	defer m.mu.Unlock()

	active := make(map[string]bool)
	for _, r := range m.store.ListRepos() {
		if r.Enabled {
			active[r.ID] = true
			if _, running := m.cancels[r.ID]; !running {
				m.startRepo(r.ID)
			}
		}
	}
	for id, cancel := range m.cancels {
		if !active[id] {
			cancel()
			delete(m.cancels, id)
			delete(m.triggers, id)
		}
	}
}

func (m *Manager) startRepo(id string) {
	ctx, cancel := context.WithCancel(context.Background())
	trigger := make(chan struct{}, 1)
	m.cancels[id] = cancel
	m.triggers[id] = trigger
	go m.runRepo(ctx, id, trigger)
}

func (m *Manager) runRepo(ctx context.Context, id string, trigger <-chan struct{}) {
	for {
		repo := m.store.GetRepo(id)
		if repo == nil {
			return
		}

		var interval time.Duration
		if repo.NextCommit != nil && repo.NextCommit.After(time.Now()) {
			// 沿用已持久化的计划，保证进程重启后调度节奏稳定、不重掷随机数。
			interval = time.Until(*repo.NextCommit)
		} else {
			plan, err := nextCommitPlan(repo, m.store.GetSchedule(), time.Now())
			if err != nil {
				log.Printf("[%s] 计算下次提交时间失败: %v", repo.Name, err)
				interval = time.Hour
			} else {
				m.store.SetPlan(id, plan)
				interval = time.Until(plan.Next)
			}
		}
		if interval <= 0 {
			interval = time.Minute
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-trigger:
			timer.Stop()
		case <-timer.C:
		}

		m.doCommit(id)
	}
}

func (m *Manager) doCommit(id string) {
	repo := m.store.GetRepo(id)
	if repo == nil {
		return
	}

	token := m.store.GetToken()
	if token == "" {
		log.Printf("[%s] 提交失败: 缺少 token", repo.Name)
		m.record(id, repo, "", errMissingToken)
		return
	}

	r, err := openOrClone(repo, token)
	if err != nil {
		log.Printf("[%s] 打开/克隆仓库失败: %v", repo.Name, err)
		m.record(id, repo, "", err)
		return
	}
	if err := syncRepo(r, repo, token); err != nil {
		log.Printf("[%s] 同步失败: %v", repo.Name, err)
		m.record(id, repo, "", err)
		return
	}
	hash, err := createEmptyCommit(r, repo)
	if err != nil {
		log.Printf("[%s] 创建提交失败: %v", repo.Name, err)
		m.record(id, repo, "", err)
		return
	}
	if err := pushRepo(r, repo, token); err != nil {
		log.Printf("[%s] 推送失败: %v", repo.Name, err)
		m.record(id, repo, hash.String(), err)
		return
	}
	log.Printf("[%s] 已推送提交 %s", repo.Name, hash)
	m.record(id, repo, hash.String(), nil)
}

func (m *Manager) record(id string, repo *Repo, hash string, err error) {
	rec := &CommitRecord{
		RepoID:  id,
		Repo:    repo.Name,
		Hash:    hash,
		Time:    time.Now(),
		Success: err == nil,
		Message: repo.CommitMsg,
	}
	if err != nil {
		rec.Message = err.Error()
	}
	m.store.RecordCommit(id, rec)
}
