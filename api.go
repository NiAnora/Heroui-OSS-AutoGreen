package main

import (
	"encoding/json"
	"net/http"
	"time"
)

// Server 聚合 Store 与 Manager，对外暴露 REST API。
type Server struct {
	store   *Store
	manager *Manager
	device  *DeviceFlow
	auth    *Authenticator
}

// NewServer 构建路由并返回 http.Handler。
func NewServer(s *Store, m *Manager) http.Handler {
	auth := NewAuthenticator()
	srv := &Server{store: s, manager: m, device: NewDeviceFlow(s), auth: auth}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/auth/login", srv.login)
	mux.HandleFunc("GET /api/auth/session", srv.session)
	mux.HandleFunc("GET /api/health", srv.health)
	mux.HandleFunc("POST /api/auth/device", srv.startDeviceAuth)
	mux.HandleFunc("GET /api/auth/device/status", srv.deviceAuthStatus)
	mux.HandleFunc("GET /api/repos", srv.listRepos)
	mux.HandleFunc("POST /api/repos", srv.addRepo)
	mux.HandleFunc("GET /api/repos/{id}", srv.getRepo)
	mux.HandleFunc("PUT /api/repos/{id}", srv.updateRepo)
	mux.HandleFunc("DELETE /api/repos/{id}", srv.deleteRepo)
	mux.HandleFunc("POST /api/repos/{id}/commit", srv.triggerCommit)
	mux.HandleFunc("POST /api/repos/{id}/pause", srv.pauseRepo)
	mux.HandleFunc("POST /api/repos/{id}/resume", srv.resumeRepo)
	mux.HandleFunc("GET /api/stats", srv.stats)
	mux.HandleFunc("GET /api/github/repos", srv.githubRepos)
	mux.HandleFunc("GET /api/settings", srv.getSettings)
	mux.HandleFunc("PUT /api/settings", srv.setSettings)
	mux.Handle("/", webHandler())

	return auth.Guard(mux)
}

// login 校验访问密钥，成功后下发 HttpOnly 会话 Cookie。
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if err := readJSON(r, &body); err != nil {
		httpError(w, http.StatusBadRequest, "请求体无效: "+err.Error())
		return
	}

	token, ok := s.auth.Login(body.Key)
	if !ok {
		httpError(w, http.StatusUnauthorized, "密码错误")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL / time.Second),
	})
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

// session 返回当前请求是否已登录，供前端决定渲染登录页还是主界面。
func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"authenticated": s.auth.Validate(sessionToken(r)),
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listRepos(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListRepos())
}

func (s *Server) getRepo(w http.ResponseWriter, r *http.Request) {
	repo := s.store.GetRepo(r.PathValue("id"))
	if repo == nil {
		httpError(w, http.StatusNotFound, "仓库不存在")
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (s *Server) addRepo(w http.ResponseWriter, r *http.Request) {
	var repo Repo
	if err := readJSON(r, &repo); err != nil {
		httpError(w, http.StatusBadRequest, "请求体无效: "+err.Error())
		return
	}
	if repo.RepoURL == "" {
		httpError(w, http.StatusBadRequest, "repoUrl 不能为空")
		return
	}
	created, err := s.store.AddRepo(&repo)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.manager.Reload(created.ID)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) updateRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store.GetRepo(id) == nil {
		httpError(w, http.StatusNotFound, "仓库不存在")
		return
	}
	var repo Repo
	if err := readJSON(r, &repo); err != nil {
		httpError(w, http.StatusBadRequest, "请求体无效: "+err.Error())
		return
	}
	updated, err := s.store.UpdateRepo(id, &repo)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.manager.Reload(id)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) deleteRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store.GetRepo(id) == nil {
		httpError(w, http.StatusNotFound, "仓库不存在")
		return
	}
	if err := s.store.DeleteRepo(id); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.manager.Reload(id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) triggerCommit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store.GetRepo(id) == nil {
		httpError(w, http.StatusNotFound, "仓库不存在")
		return
	}
	s.manager.Trigger(id)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "triggered"})
}

func (s *Server) pauseRepo(w http.ResponseWriter, r *http.Request) {
	s.setEnabled(w, r, false)
}

func (s *Server) resumeRepo(w http.ResponseWriter, r *http.Request) {
	s.setEnabled(w, r, true)
}

func (s *Server) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	id := r.PathValue("id")
	repo := s.store.GetRepo(id)
	if repo == nil {
		httpError(w, http.StatusNotFound, "仓库不存在")
		return
	}
	repo.Enabled = enabled
	updated, err := s.store.UpdateRepo(id, repo)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.manager.Reload(id)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) stats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"daily":  s.store.DailyStats(30),
		"recent": s.store.CommitLog(50),
	})
}

func (s *Server) githubRepos(w http.ResponseWriter, _ *http.Request) {
	repos, err := listGithubRepos(s.store.GetToken())
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, repos)
}

func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"tokenSet":     s.store.GetToken() != "",
		"tokenPreview": maskToken(s.store.GetToken()),
		"clientId":     effectiveClientID(s.store.GetClientID()),
		"schedule":     s.store.GetSchedule(),
	})
}

func (s *Server) setSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string          `json:"token"`
		ClientID string          `json:"clientId"`
		Schedule *ScheduleConfig `json:"schedule"`
	}
	if err := readJSON(r, &body); err != nil {
		httpError(w, http.StatusBadRequest, "请求体无效: "+err.Error())
		return
	}
	if body.ClientID != "" {
		if err := s.store.SetClientID(body.ClientID); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if body.Token != "" {
		if err := s.store.SetToken(body.Token); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if body.Schedule != nil {
		if _, err := s.store.SetSchedule(*body.Schedule); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// 计划变更后所有仓库立即按新配置重算下次提交时间。
		s.manager.ReloadAll()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tokenSet":     s.store.GetToken() != "",
		"tokenPreview": maskToken(s.store.GetToken()),
		"clientId":     effectiveClientID(s.store.GetClientID()),
		"schedule":     s.store.GetSchedule(),
	})
}

// maskToken 仅保留 token 末 4 位用于展示，避免泄露完整凭据。
func maskToken(t string) string {
	if t == "" {
		return ""
	}
	if len(t) <= 4 {
		return "••••"
	}
	return "••••" + t[len(t)-4:]
}

func (s *Server) startDeviceAuth(w http.ResponseWriter, _ *http.Request) {
	st, err := s.device.Start()
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) deviceAuthStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.device.Status())
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
