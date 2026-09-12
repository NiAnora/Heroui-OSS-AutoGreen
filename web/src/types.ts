/** 调度模式：每周随机 N 天 / 每天随机是否提交。 */
export type ScheduleMode = "weekly" | "daily";

/** 全局提交计划配置，所有启用的仓库共用；时分秒为闭区间，在区间内真随机。 */
export interface ScheduleConfig {
  mode: ScheduleMode;
  weekDays: number;
  hourFrom: number;
  hourTo: number;
  minuteFrom: number;
  minuteTo: number;
  secondFrom: number;
  secondTo: number;
}

export interface Repo {
  id: string;
  name: string;
  repoUrl: string;
  branch: string;
  userName: string;
  userEmail: string;
  commitMsg: string;
  enabled: boolean;
  workDir?: string;
  lastCommit?: string | null;
  nextCommit?: string | null;
  commitCount: number;
  lastError?: string;
}

/** 创建 / 更新仓库时提交的可配置字段（不含 id 与运行时状态）。 */
export interface RepoInput {
  name: string;
  repoUrl: string;
  branch: string;
  userName: string;
  userEmail: string;
  commitMsg: string;
  enabled: boolean;
}

export interface CommitRecord {
  repoId: string;
  repo: string;
  hash: string;
  time: string;
  success: boolean;
  message: string;
}

export interface Stats {
  daily: Record<string, number>;
  recent: CommitRecord[];
}

export interface Settings {
  tokenSet: boolean;
  tokenPreview: string;
  clientId: string;
  schedule: ScheduleConfig;
}

export interface DeviceFlow {
  status: "pending" | "success" | "error";
  userCode?: string;
  verificationUri?: string;
  expiresIn?: number;
  message?: string;
}

export interface GithubRepo {
  fullName: string;
  htmlUrl: string;
  cloneUrl: string;
  defaultBranch: string;
  private: boolean;
}
