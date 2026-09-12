package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

var (
	errMissingClientID = errors.New("请先在设置中配置 GitHub OAuth App 的 Client ID")
	errGitHubAuth      = errors.New("GitHub 授权服务返回异常")

	// httpClient 限制单个请求超时，避免网络异常时长时间挂起。
	httpClient = &http.Client{Timeout: 15 * time.Second}
)

const (
	githubDeviceCodeURL  = "https://github.com/login/device/code"
	githubAccessTokenURL = "https://github.com/login/oauth/access_token"
	deviceScope          = "repo"

	// defaultClientID 是内置的共享 OAuth App client_id（设备授权流的公开信息）。
	// 填入后，其他用户零配置即可点击「一键授权」，无需各自创建应用。
	// 用户仍可在设置页填写自己的 client_id 来覆盖此默认值。
	defaultClientID = "Ov23liEWJEx6JCEK1pEX"
)

// effectiveClientID 返回生效的 client_id：优先用户设置，否则回退到内置默认值。
func effectiveClientID(storeID string) string {
	if storeID != "" {
		return storeID
	}
	return defaultClientID
}

// parseFormBody 读取并解析 x-www-form-urlencoded 格式的响应体。
func parseFormBody(resp *http.Response) (url.Values, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return url.ParseQuery(string(data))
}

// atoi 将字符串转为整数，失败返回 0。
func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// deviceCodeResponse 是 GitHub 设备授权流第一步的返回。
type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// deviceStatus 是向前端暴露的授权进度。
type deviceStatus struct {
	Status          string `json:"status"` // pending | success | error
	UserCode        string `json:"userCode,omitempty"`
	VerificationURI string `json:"verificationUri,omitempty"`
	ExpiresIn       int    `json:"expiresIn,omitempty"`
	Message         string `json:"message,omitempty"`
}

// DeviceFlow 管理单次设备授权流程与后台轮询。
type DeviceFlow struct {
	mu     sync.Mutex
	active *deviceStatus
	cancel context.CancelFunc
	store  *Store
}

func NewDeviceFlow(s *Store) *DeviceFlow {
	return &DeviceFlow{store: s}
}

// Start 发起设备授权，返回给用户展示的验证码与地址，并在后台轮询直至拿到 token。
func (d *DeviceFlow) Start() (*deviceStatus, error) {
	clientID := effectiveClientID(d.store.GetClientID())
	if clientID == "" {
		return nil, errMissingClientID
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", deviceScope)
	resp, err := httpClient.PostForm(githubDeviceCodeURL, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errGitHubAuth
	}

	values, err := parseFormBody(resp)
	if err != nil {
		return nil, err
	}
	code := deviceCodeResponse{
		DeviceCode:      values.Get("device_code"),
		UserCode:        values.Get("user_code"),
		VerificationURI: values.Get("verification_uri"),
		ExpiresIn:       atoi(values.Get("expires_in")),
		Interval:        atoi(values.Get("interval")),
	}

	interval := code.Interval
	if interval <= 0 {
		interval = 5
	}

	ctx, cancel := context.WithCancel(context.Background())
	st := &deviceStatus{
		Status:          "pending",
		UserCode:        code.UserCode,
		VerificationURI: code.VerificationURI,
		ExpiresIn:       code.ExpiresIn,
	}

	d.mu.Lock()
	if d.cancel != nil {
		d.cancel()
	}
	d.active = st
	d.cancel = cancel
	d.mu.Unlock()

	go d.poll(ctx, clientID, code.DeviceCode, interval, code.ExpiresIn)
	return st, nil
}

// Status 返回当前授权进度。
func (d *DeviceFlow) Status() *deviceStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.active == nil {
		return &deviceStatus{Status: "pending"}
	}
	cp := *d.active
	return &cp
}

func (d *DeviceFlow) poll(ctx context.Context, clientID, deviceCode string, interval, expiresIn int) {
	// 设备码过期后留 30 秒缓冲，避免网络延迟导致误判。
	deadline := time.Now().Add(time.Duration(expiresIn)*time.Second + 30*time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(deadline)):
			d.setStatus("error", "", "授权超时，请重新发起授权")
			return
		case <-time.After(time.Duration(interval) * time.Second):
		}

		form := url.Values{}
		form.Set("client_id", clientID)
		form.Set("device_code", deviceCode)
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		resp, err := httpClient.PostForm(githubAccessTokenURL, form)
		if err != nil {
			// 网络异常可恢复：继续轮询直到设备码过期，避免一次抖动就中断授权。
			continue
		}
		values, err := parseFormBody(resp)
		resp.Body.Close()
		if err != nil {
			continue
		}

		accessToken := values.Get("access_token")
		if accessToken != "" {
			if err := d.store.SetToken(accessToken); err != nil {
				d.setStatus("error", "", err.Error())
				return
			}
			d.setStatus("success", "", "")
			return
		}

		errName := values.Get("error")
		errDesc := values.Get("error_description")
		switch errName {
		case "authorization_pending":
			// 用户尚未授权，继续等待。
		case "slow_down":
			interval += 5
		case "access_denied":
			d.setStatus("error", "", "授权被拒绝")
			return
		case "expired_token":
			d.setStatus("error", "", "验证码已过期，请重新发起授权")
			return
		default:
			d.setStatus("error", "", errDesc)
			return
		}
	}
}

func (d *DeviceFlow) setStatus(status, message, errMsg string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.active == nil {
		return
	}
	d.active.Status = status
	if errMsg != "" {
		d.active.Message = errMsg
	} else {
		d.active.Message = message
	}
}
