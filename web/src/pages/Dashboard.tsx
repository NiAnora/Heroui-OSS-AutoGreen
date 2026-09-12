import { useEffect, useState } from "react";
import type { ReactNode } from "react";
import { Card, Chip, Spinner, toast } from "@heroui/react";
import { api } from "../api";
import type { Repo, Stats } from "../types";
import {
  lastNDays,
  monthDay,
  relativeTime,
  shortHash,
  todayKey,
} from "../utils";
import {
  CheckCircleIcon,
  ClockIcon,
  DashboardIcon,
  RepoIcon,
  SendIcon,
  XCircleIcon,
} from "../components/icons";

function StatCard({
  label,
  value,
  icon,
  accent,
}: {
  label: string;
  value: string | number;
  icon: ReactNode;
  accent: string;
}) {
  return (
    <Card className="p-5">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted">{label}</p>
        <div className={`flex size-9 items-center justify-center rounded-lg ${accent}`}>{icon}</div>
      </div>
      <p className="mt-4 text-3xl font-semibold leading-none tabular-nums text-foreground">
        {value}
      </p>
    </Card>
  );
}

export default function Dashboard() {
  const [repos, setRepos] = useState<Repo[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [repoList, statData] = await Promise.all([api.listRepos(), api.stats()]);
        if (cancelled) return;
        setRepos(repoList);
        setStats(statData);
      } catch (error) {
        toast.danger((error as Error).message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  if (loading) {
    return (
      <div className="flex h-[60vh] items-center justify-center">
        <Spinner size="lg" />
      </div>
    );
  }

  const enabledCount = repos.filter((repo) => repo.enabled).length;
  const totalCommits = repos.reduce((sum, repo) => sum + repo.commitCount, 0);
  const todayCommits = stats?.daily[todayKey()] ?? 0;

  const days = lastNDays(30);
  const values = days.map((day) => stats?.daily[day] ?? 0);
  const maxValue = Math.max(1, ...values);

  return (
    <div className="mx-auto w-full max-w-6xl px-6 pb-12 pt-8">
      <header className="mb-8">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">仪表盘</h1>
        <p className="mt-1 text-sm text-muted">贡献图与提交状态概览</p>
      </header>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="仓库总数"
          value={repos.length}
          icon={<RepoIcon className="size-5" />}
          accent="bg-accent/10 text-accent"
        />
        <StatCard
          label="启用中仓库数"
          value={enabledCount}
          icon={<DashboardIcon className="size-5" />}
          accent="bg-success/10 text-success"
        />
        <StatCard
          label="今日提交次数"
          value={todayCommits}
          icon={<SendIcon className="size-5" />}
          accent="bg-warning/10 text-warning"
        />
        <StatCard
          label="累计提交次数"
          value={totalCommits}
          icon={<ClockIcon className="size-5" />}
          accent="bg-accent/10 text-accent"
        />
      </div>

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <Card.Header>
            <Card.Title>每日提交趋势</Card.Title>
            <Card.Description>最近 30 天成功提交次数</Card.Description>
          </Card.Header>
          <Card.Content>
            <div className="flex h-44 items-end gap-1">
              {days.map((day, index) => (
                <div
                  key={day}
                  className="flex h-full flex-1 flex-col justify-end"
                  title={`${day} · ${values[index]} 次提交`}
                >
                  <div
                    className={`w-full rounded-t ${
                      values[index] === 0 ? "bg-surface-secondary" : "bg-accent"
                    }`}
                    style={{
                      height: `${
                        values[index] === 0 ? 2 : Math.max(6, (values[index] / maxValue) * 100)
                      }%`,
                    }}
                  />
                </div>
              ))}
            </div>
            <div className="mt-2 flex justify-between text-[11px] text-muted">
              <span>{monthDay(days[0])}</span>
              <span>{monthDay(days[Math.floor(days.length / 2)])}</span>
              <span>{monthDay(days[days.length - 1])}</span>
            </div>
          </Card.Content>
        </Card>

        <Card>
          <Card.Header>
            <Card.Title>最近提交</Card.Title>
            <Card.Description>最近 {stats?.recent.length ?? 0} 条记录</Card.Description>
          </Card.Header>
          <Card.Content className="max-h-[340px] overflow-y-auto">
            {stats && stats.recent.length > 0 ? (
              <ul className="flex flex-col divide-y divide-separator">
                {stats.recent.map((record, index) => (
                  <li key={`${record.time}-${index}`} className="flex items-start gap-3 py-3">
                    {record.success ? (
                      <CheckCircleIcon className="mt-0.5 size-4 shrink-0 text-success" />
                    ) : (
                      <XCircleIcon className="mt-0.5 size-4 shrink-0 text-danger" />
                    )}
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between gap-2">
                        <span className="truncate text-sm font-medium text-foreground">
                          {record.repo}
                        </span>
                        <span className="shrink-0 font-mono text-xs text-muted">
                          {record.hash ? shortHash(record.hash) : "—"}
                        </span>
                      </div>
                      <p className="truncate text-xs text-muted">{record.message}</p>
                      <p className="mt-0.5 text-xs text-muted">{relativeTime(record.time)}</p>
                    </div>
                  </li>
                ))}
              </ul>
            ) : (
              <div className="flex flex-col items-center justify-center gap-2 py-16 text-center">
                <p className="text-sm text-muted">暂无提交记录</p>
                <Chip variant="soft" size="sm">
                  添加仓库后自动提交
                </Chip>
              </div>
            )}
          </Card.Content>
        </Card>
      </div>
    </div>
  );
}
