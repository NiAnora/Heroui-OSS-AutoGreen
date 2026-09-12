package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// secretEnv 是存放登录密钥的环境变量名。
	secretEnv = "AUTOGREEN_SECRET"
	// secretLen 是密钥期望的长度（位）。
	secretLen = 48
	// sessionTTL 是登录会话的有效期。
	sessionTTL = 7 * 24 * time.Hour
	// sessionCookie 是承载会话 token 的 Cookie 名。
	sessionCookie = "autogreen_session"
)

// Authenticator 负责校验登录密钥并维护内存中的登录会话。
// 会话仅存在于进程内存，进程重启后全部失效，需重新登录。
type Authenticator struct {
	secret string
	ttl    time.Duration

	mu       sync.RWMutex
	sessions map[string]time.Time
}

// NewAuthenticator 从环境变量读取密钥并初始化。
// 未配置时不会阻止启动，但任何人都无法登录。
func NewAuthenticator() *Authenticator {
	secret := os.Getenv(secretEnv)
	switch {
	case secret == "":
		log.Printf("警告: 未设置环境变量 %s，后台将无法登录", secretEnv)
	case len(secret) != secretLen:
		log.Printf("警告: %s 长度为 %d，期望 %d 位", secretEnv, len(secret), secretLen)
	}
	return &Authenticator{
		secret:   secret,
		ttl:      sessionTTL,
		sessions: make(map[string]time.Time),
	}
}

// Login 校验密钥，通过则创建会话并返回 token。
func (a *Authenticator) Login(key string) (string, bool) {
	if !a.match(key) {
		return "", false
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Printf("生成会话 token 失败: %v", err)
		return "", false
	}
	token := hex.EncodeToString(b)

	a.mu.Lock()
	defer a.mu.Unlock()
	a.purgeLocked()
	a.sessions[token] = time.Now().Add(a.ttl)
	return token, true
}

// Validate 判断会话 token 是否存在且未过期。
func (a *Authenticator) Validate(token string) bool {
	if token == "" {
		return false
	}

	a.mu.RLock()
	expiry, ok := a.sessions[token]
	a.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		a.mu.Lock()
		delete(a.sessions, token)
		a.mu.Unlock()
		return false
	}
	return true
}

// match 以常量时间比较密钥，避免通过响应耗时逐位猜测。
func (a *Authenticator) match(key string) bool {
	// 未配置密钥时任何人都不能登录。注意 ConstantTimeCompare 对两个空串
	// 会返回 1，因此必须先显式排除空配置。
	if a.secret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(key), []byte(a.secret)) == 1
}

// purgeLocked 清理已过期会话，调用方需持有写锁。
func (a *Authenticator) purgeLocked() {
	now := time.Now()
	for token, expiry := range a.sessions {
		if now.After(expiry) {
			delete(a.sessions, token)
		}
	}
}

// sessionToken 从请求 Cookie 中取出会话 token。
func sessionToken(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// publicAPI 列出无需登录即可访问的接口。
// login 与 session 必须公开，否则前端无法完成登录；health 用于探活。
func publicAPI(path string) bool {
	switch path {
	case "/api/auth/login", "/api/auth/session", "/api/health":
		return true
	}
	return false
}

// Guard 拦截未登录的 API 请求。
// 非 /api/ 的静态资源一律放行，否则前端连登录页都加载不出来。
func (a *Authenticator) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || publicAPI(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if !a.Validate(sessionToken(r)) {
			httpError(w, http.StatusUnauthorized, "未登录")
			return
		}
		next.ServeHTTP(w, r)
	})
}
