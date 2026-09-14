# AutoGreen&Heroui-OSS

自动保持 GitHub 贡献图常绿(作用:未知????)。Go 单文件后端 + React 管理界面，无需 GitHub Actions，在你自己的服务器上常驻运行。

核心代码来自 [XiaoCaoAskedForHelp/AutoGreen](https://github.com/XiaoCaoAskedForHelp/AutoGreen)，本项目把XiaoCaoAskedForHelp的 GitHub Actions 定时空提交方案改写为独立的 Go 服务，并加入了真随机调度、多仓库管理、Web 界面与登录门禁。

## 特性

- **真随机调度**：基于 `crypto/rand`（CSPRNG），支持「每周随机 N 天」或「每天随机提交/不提交」，时分秒各自在指定区间内随机，杜绝固定节律
- **多仓库管理**：一个进程管理任意数量仓库，各自独立的提交信息、分支与身份
- **Web 管理界面**：前端通过 `//go:embed` 内嵌进二进制，**部署只需一个文件**
- **GitHub 授权**：内置共享 OAuth App 的 Device Flow，零配置一键授权；也可手动填 Token
- **登录门禁**：环境变量配置 48 位密钥，服务端会话 + HttpOnly Cookie
- **无 CGO**：纯 Go 实现 git 操作（`go-git`），交叉编译开箱即用

## 编译

### 前置依赖

| 依赖 | 版本 | 说明 |
| --- | --- | --- |
| Go | 1.26+ | 必须 |
| Node.js | 20+ | 仅构建前端时需要 |

> **重要**：Go 通过 `//go:embed all:web/dist` 内嵌前端产物，而 `web/dist` 属于构建产物、不入版本库。**直接 `go build` 会报 `pattern all:web/dist: no matching files found`，必须先构建前端。**

### 1. 构建前端

```bash
cd web
npm install
npm run build      # 产物输出到 web/dist
cd ..
```

`npm run build` 会先跑 `tsc -b` 做类型检查，再执行 vite 打包，任何类型错误都会导致构建失败。

### 2. 构建后端

```bash
# 当前平台
go build -trimpath -ldflags="-s -w" -o autogreen .

# 交叉编译 Linux arm64（常见服务器架构）
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o autogreen-linux-arm64 .

# 交叉编译 Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o autogreen-linux-amd64 .
```

Windows PowerShell 下设置环境变量：

```powershell
$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="arm64"
go build -trimpath -ldflags="-s -w" -o autogreen-linux-arm64 .
```

参数说明：`CGO_ENABLED=0` 保证纯静态链接（交叉编译必需），`-trimpath` 去掉本机绝对路径，`-ldflags="-s -w"` 去掉符号表与调试信息。

> 内存较小的机器上，链接阶段可能因并行压缩 DWARF 而 OOM。加上 `GOMAXPROCS=1` 可显著降低内存占用。

### 3. 验证产物

产物约 10 MB，可确认架构是否正确：

```bash
file autogreen-linux-arm64     # 应显示 ARM aarch64
uname -m                       # 服务器上执行，arm64 机器输出 aarch64
```

## 部署

### 环境变量

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `AUTOGREEN_SECRET` | **是** | 无 | 后台登录密钥，**必须恰好 48 位**。未设置或长度不符时启动会打警告，且后台无法登录 |
| `AUTOGREEN_LISTEN` | 否 | `:8080` | 监听地址。写 `:8080` 或直接写 `8080` 均可，前者可绑定所有网卡 |
| `AUTOGREEN_TOKEN` | 否 | 无 | 首次初始化时写入的 GitHub Token，仅当 `autogreen.json` 不存在时生效 |
| `GITHUB_TOKEN` | 否 | 无 | 同上，优先级低于 `AUTOGREEN_TOKEN` |

生成 48 位密钥：

```bash
openssl rand -hex 24        # 24 字节 → 48 个十六进制字符
```

若 `AUTOGREEN_TOKEN` 与 `GITHUB_TOKEN` 都未设置，程序还会尝试执行 `gh auth token` 复用本机 gh CLI 凭证；全部失败则 token 为空，需登录后台手动授权。

### 运行

```bash
chmod +x autogreen-linux-arm64

# 建议先切到专门目录，数据文件会生成在当前工作目录
mkdir -p /opt/autogreen && cd /opt/autogreen

AUTOGREEN_SECRET=你的48位密钥 AUTOGREEN_LISTEN=:8080 /path/to/autogreen-linux-arm64
```

启动成功后日志输出：

```
autogreen 后台已启动，访问 http://localhost:8080
```

### 数据文件

程序在**当前工作目录**下读写以下内容：

| 路径 | 说明 |
| --- | --- |
| `autogreen.json` | 全部持久化状态：GitHub Token、OAuth client_id、提交计划、仓库列表、提交历史。文件权限 `0600` |
| `repos/<仓库ID>/` | 各仓库的本地克隆副本，用于产生空提交并推送 |

> `autogreen.json` 内**以明文保存 GitHub Token**。切勿提交到版本库、切勿随二进制一起分发。删除某仓库时，其对应的 `repos/<ID>/` 目录会一并清除。

### systemd 常驻

`/etc/systemd/system/autogreen.service`：

```ini
[Unit]
Description=AutoGreen
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/autogreen
Environment=AUTOGREEN_SECRET=你的48位密钥
Environment=AUTOGREEN_LISTEN=:8080
ExecStart=/opt/autogreen/autogreen-linux-arm64
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now autogreen
sudo systemctl status autogreen
journalctl -u autogreen -f
```

### 反向代理

服务本身**不提供 HTTPS**。若需公网访问，请前置 Nginx / Caddy 配置 TLS，否则登录 Cookie 会以明文传输。例如 Caddy 只需一行：

```
green.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

## 使用

1. 浏览器打开 `http://<服务器地址>:8080`，用 48 位密钥登录
2. 进入**设置**页：
   - **GitHub 授权**：点「一键授权」走 Device Flow，按提示在 GitHub 输入验证码即可；也可展开手动配置直接粘贴 Personal Access Token
   - **提交计划**：配置全局调度规则（见下）
3. 进入**仓库**页，点「添加仓库」从 GitHub 仓库列表中直接勾选，或手动填写仓库地址

### 提交计划

全局生效，所有启用的仓库共用同一套规则。两种模式**二选一**：

| 模式 | 含义 |
| --- | --- |
| 每周随机 | 从周一到周日中随机选出 N 天（N 可配 1–7），这 N 天各提交一次 |
| 每天随机 | 每天独立掷一次硬币，命中才提交（0 不提交 / 1 提交） |

两种模式下都可为**小时、分钟、秒各指定一个闭区间**，程序在区间内用真随机取值组装出提交时刻。例如小时填 `9-22`、分钟填 `0-59`、秒填 `0-59`，则提交永远落在上午 9 点到晚上 10 点之间，不会出现在凌晨。

调度状态会持久化（本周已选中的日期、今日是否已提交），保证**每周恰好 N 次、每天至多一次**。服务重启不会重复提交或丢失当日计划。

修改计划并保存后，所有仓库立即按新规则重新计算下次提交时间。

## 二次开发

### 目录结构

```
AutoGreen/
├── main.go          程序入口：加载配置 → 启动调度 → 启动 HTTP 服务 → 优雅关闭
├── api.go           HTTP 路由注册、各接口处理器、鉴权中间件挂载
├── auth.go          登录门禁：密钥校验、会话管理、Cookie、请求守卫
├── store.go         状态存储：Repo/CommitRecord 结构、JSON 读写、并发保护
├── manager.go       调度管理器：每仓库一个 goroutine，负责定时触发与重算
├── scheduler.go     调度算法：ScheduleConfig 定义、真随机取值、计划生成
├── git.go           基于 go-git 的克隆、拉取、空提交与推送
├── github_api.go    调用 GitHub REST API（拉取用户仓库列表）
├── oauth.go         GitHub Device Flow 授权与后台轮询
├── embed.go         前端静态资源内嵌与 http.FileServer 封装
└── web/             前端工程（Vite + React + TypeScript + HeroUI）
```

### 前端开发

前端是独立的 Vite 工程，开发时享受热更新，无需每次重新编译 Go：

```bash
# 终端 1：启动后端，注意端口要和 vite 代理配置一致
AUTOGREEN_SECRET=... go run .

# 终端 2：启动前端开发服务器
cd web && npm run dev
```

[vite.config.ts](./web/vite.config.ts) 已把 `/api` 代理到 `http://localhost:8080`，因此把后端监听端口设为默认的 8080 即可正常联调。

> **改完前端务必重新 `npm run build`**：只有 `web/dist` 更新后，重新编译的 Go 二进制才会内嵌新界面。`npm run dev` 的改动不会自动进入二进制。

前端代码组织：

| 路径 | 说明 |
| --- | --- |
| [web/src/api.ts](./web/src/api.ts) | 所有后端接口的封装，统一处理错误与 401 跳登录 |
| [web/src/types.ts](./web/src/types.ts) | 与 Go 结构体一一对应的 TypeScript 类型 |
| [web/src/App.tsx](./web/src/App.tsx) | 根组件，负责会话检查与登录态切换 |
| [web/src/pages/](./web/src/pages/) | 页面：Dashboard / Repos / Settings / Login |
| [web/src/components/](./web/src/components/) | 复用组件：侧栏、仓库表单弹窗、确认框、图标 |

### 新增一个 API

后端在 [api.go](./api.go) 的 `NewServer` 中按 Go 1.22+ 的「方法 + 路径」模式注册：

```go
mux.HandleFunc("POST /api/repos/{id}/commit", srv.triggerCommit)
```

路径参数用 `r.PathValue("id")` 读取，响应统一走 `writeJSON` / `httpError`。前端在 [api.ts](./web/src/api.ts) 里加一个方法即可，`request<T>` 已处理 JSON 序列化与错误提取。

### 关于鉴权

[embed.go](./embed.go) 与 [auth.go](./auth.go) 中的 `Guard` 共同构成访问控制：所有 `/api/` 开头的请求默认需要有效会话，静态资源和白名单接口（`/api/auth/login`、`/api/auth/session`、`/api/health`）除外。**新增接口默认就是受保护的**，无需额外配置；确实需要匿名访问时，把路径加进 [auth.go](./auth.go) 的 `publicAPI`。

会话保存在内存的 `map[token]expiry` 中，**进程重启即失效**，用户需要重新登录。

### 关于调度算法

[manager.go](./manager.go) 为每个启用的仓库维护一个 goroutine：先看 `Repo.NextCommit` 是否仍然有效，无效则调用 [scheduler.go](./scheduler.go) 的 `nextCommitPlan` 重新规划，再睡眠到目标时刻执行提交。

这是**单次执行模型**而非固定 ticker——每次提交完成后都会重新规划下一次，因此调度间隔天然随机。修改调度逻辑时注意：计划状态（`PlanWeek` / `PlanDays` / `PlanDate`）是「每天至多一次」的保证，误改可能导致同日重复提交或漏提交。

在设置页保存计划会触发 `Manager.ReloadAll()`，它会取消所有 goroutine、清空全部 `NextCommit` 后重新收敛，使新规则立刻对所有仓库生效。

## 常见问题

**登录提示「密码错误」**
检查 `AUTOGREEN_SECRET` 是否**恰好 48 位**。若长度不符，程序会在启动时打印警告，此时任何密钥都无法通过校验。

**启动报 `listen tcp: address 8080: missing port in address`**
旧版本要求必须带冒号。当前版本已容错，`AUTOGREEN_LISTEN` 填 `8080` 会自动补成 `:8080`。

**编译报 `pattern all:web/dist: no matching files found`**
前端产物缺失，先执行 `cd web && npm install && npm run build`。

**启动后仓库没有被调度**
确认该仓库处于「启用」状态。暂停的仓库不会参与调度。

**提交失败**
在后台仓库详情中查看 `LastError` 字段的报错信息。常见原因：Token 权限不足（需要 `repo` 权限才能操作私有仓库）、仓库地址或分支填写错误、服务器无法访问 GitHub。

## 致谢

- [XiaoCaoAskedForHelp/AutoGreen](https://github.com/XiaoCaoAskedForHelp/AutoGreen) —— 最初的 GitHub Actions 定时空提交方案，本项目的核心思路与调度模型均源于此
- [heroui-oss](https://github.com/heroui-inc/heroui) —— 提供 HeroUI React 组件库（Apache-2.0），本项目的前端界面基于其构建

## License

[MIT](./LICENSE)
