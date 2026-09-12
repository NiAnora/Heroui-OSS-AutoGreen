import type {
  DeviceFlow,
  GithubRepo,
  Repo,
  RepoInput,
  ScheduleConfig,
  Settings,
  Stats,
} from "./types";

/**
 * 会话失效（401）时的回调。由 App 注册，用于把界面切回登录页，
 * 否则会话过期后页面会一直停留在报错状态。
 */
let unauthorizedHandler: (() => void) | null = null;

export function setUnauthorizedHandler(handler: (() => void) | null) {
  unauthorizedHandler = handler;
}

/** 登录与查询会话这两个接口自身会返回 401，不应触发回退。 */
function isAuthEndpoint(path: string): boolean {
  return path === "/auth/login" || path === "/auth/session";
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, {
    ...init,
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });

  if (res.status === 401 && !isAuthEndpoint(path)) {
    unauthorizedHandler?.();
  }

  let data: unknown;
  try {
    data = await res.json();
  } catch {
    data = null;
  }

  if (!res.ok) {
    const message =
      data && typeof data === "object" && "error" in data
        ? String((data as { error: unknown }).error)
        : `请求失败（HTTP ${res.status}）`;
    throw new Error(message);
  }

  return data as T;
}

export const api = {
  /** 查询当前是否已登录。 */
  session: () => request<{ authenticated: boolean }>("/auth/session"),
  login: (key: string) =>
    request<{ authenticated: boolean }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ key }),
    }),

  health: () => request<{ ok: boolean }>("/health"),

  listRepos: () => request<Repo[]>("/repos"),
  getRepo: (id: string) => request<Repo>(`/repos/${id}`),
  createRepo: (input: RepoInput) =>
    request<Repo>("/repos", { method: "POST", body: JSON.stringify(input) }),
  updateRepo: (id: string, input: RepoInput) =>
    request<Repo>(`/repos/${id}`, { method: "PUT", body: JSON.stringify(input) }),
  deleteRepo: (id: string) =>
    request<{ ok: boolean }>(`/repos/${id}`, { method: "DELETE" }),

  triggerCommit: (id: string) =>
    request<{ status: string }>(`/repos/${id}/commit`, { method: "POST" }),
  pauseRepo: (id: string) => request<Repo>(`/repos/${id}/pause`, { method: "POST" }),
  resumeRepo: (id: string) => request<Repo>(`/repos/${id}/resume`, { method: "POST" }),

  stats: () => request<Stats>("/stats"),

  listGithubRepos: () => request<GithubRepo[]>("/github/repos"),

  getSettings: () => request<Settings>("/settings"),
  updateSettings: (payload: {
    token?: string;
    clientId?: string;
    schedule?: ScheduleConfig;
  }) => request<Settings>("/settings", { method: "PUT", body: JSON.stringify(payload) }),

  startDeviceAuth: () => request<DeviceFlow>("/auth/device", { method: "POST" }),
  deviceAuthStatus: () => request<DeviceFlow>("/auth/device/status"),
};
