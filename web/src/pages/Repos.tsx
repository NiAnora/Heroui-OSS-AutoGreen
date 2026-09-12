import { useCallback, useEffect, useState } from "react";
import { Button, Card, Chip, Spinner, toast } from "@heroui/react";
import { api } from "../api";
import type { Repo } from "../types";
import { formatTime } from "../utils";
import ConfirmModal from "../components/ConfirmModal";
import RepoFormModal from "../components/RepoFormModal";
import {
  PauseIcon,
  PencilIcon,
  PlayIcon,
  PlusIcon,
  SendIcon,
  TrashIcon,
} from "../components/icons";

type PendingAction = { id: string; type: "toggle" | "commit" } | null;

export default function Repos() {
  const [repos, setRepos] = useState<Repo[]>([]);
  const [loading, setLoading] = useState(true);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Repo | null>(null);
  const [deleting, setDeleting] = useState<Repo | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [pending, setPending] = useState<PendingAction>(null);

  const load = useCallback(async () => {
    try {
      const repoList = await api.listRepos();
      setRepos(repoList);
    } catch (error) {
      toast.danger((error as Error).message);
    }
  }, []);

  useEffect(() => {
    (async () => {
      await load();
      setLoading(false);
    })();
  }, [load]);

  const handleToggle = async (repo: Repo) => {
    setPending({ id: repo.id, type: "toggle" });
    try {
      if (repo.enabled) {
        await api.pauseRepo(repo.id);
        toast.success(`已暂停「${repo.name}」`);
      } else {
        await api.resumeRepo(repo.id);
        toast.success(`已恢复「${repo.name}」`);
      }
      await load();
    } catch (error) {
      toast.danger((error as Error).message);
    } finally {
      setPending(null);
    }
  };

  const handleCommit = async (repo: Repo) => {
    setPending({ id: repo.id, type: "commit" });
    try {
      await api.triggerCommit(repo.id);
      toast.success(`已触发「${repo.name}」提交`);
      await load();
      window.setTimeout(() => load(), 1500);
    } catch (error) {
      toast.danger((error as Error).message);
    } finally {
      setPending(null);
    }
  };

  const handleDelete = async () => {
    if (!deleting) return;
    setDeleteLoading(true);
    try {
      await api.deleteRepo(deleting.id);
      toast.success(`已删除「${deleting.name}」`);
      setDeleting(null);
      await load();
    } catch (error) {
      toast.danger((error as Error).message);
    } finally {
      setDeleteLoading(false);
    }
  };

  const openAdd = () => {
    setEditing(null);
    setFormOpen(true);
  };

  const openEdit = (repo: Repo) => {
    setEditing(repo);
    setFormOpen(true);
  };

  return (
    <div className="mx-auto w-full max-w-6xl px-6 pb-12 pt-8">
      <header className="mb-8 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">仓库管理</h1>
          <p className="mt-1 text-sm text-muted">管理自动提交的 GitHub 仓库</p>
        </div>
        <Button variant="primary" onPress={openAdd}>
          <PlusIcon className="size-4" />
          添加仓库
        </Button>
      </header>

      {loading ? (
        <div className="flex h-[50vh] items-center justify-center">
          <Spinner size="lg" />
        </div>
      ) : repos.length === 0 ? (
        <Card className="flex flex-col items-center justify-center gap-3 py-20">
          <div className="flex size-12 items-center justify-center rounded-2xl bg-surface-secondary text-muted">
            <PlusIcon className="size-6" />
          </div>
          <p className="text-sm font-medium text-foreground">还没有仓库</p>
          <p className="text-sm text-muted">添加一个 GitHub 仓库开始自动提交</p>
          <Button className="mt-2" variant="secondary" onPress={openAdd}>
            <PlusIcon className="size-4" />
            添加仓库
          </Button>
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {repos.map((repo) => (
            <Card key={repo.id} className="flex flex-col">
              <Card.Header className="flex items-start justify-between gap-3">
                <div className="flex min-w-0 flex-col gap-1">
                  <Card.Title className="truncate">{repo.name}</Card.Title>
                  <a
                    href={repo.repoUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="truncate text-xs text-muted hover:text-foreground"
                  >
                    {repo.repoUrl}
                  </a>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-1.5">
                  {repo.enabled ? (
                    <Chip color="success" variant="soft" size="sm">
                      已启用
                    </Chip>
                  ) : (
                    <Chip color="warning" variant="soft" size="sm">
                      已暂停
                    </Chip>
                  )}
                </div>
              </Card.Header>

              <Card.Content className="flex flex-1 flex-col gap-4">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <p className="text-xs text-muted">下次提交</p>
                    <p className="mt-0.5 text-sm font-medium text-foreground">
                      {formatTime(repo.nextCommit)}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs text-muted">累计提交</p>
                    <p className="mt-0.5 text-sm font-medium tabular-nums text-foreground">
                      {repo.commitCount}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs text-muted">分支</p>
                    <p className="mt-0.5 text-sm font-medium text-foreground">
                      {repo.branch || "main"}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs text-muted">最近提交</p>
                    <p className="mt-0.5 text-sm font-medium text-foreground">
                      {formatTime(repo.lastCommit)}
                    </p>
                  </div>
                </div>

                {repo.lastError ? (
                  <div className="rounded-lg bg-danger/10 px-3 py-2 text-xs leading-relaxed text-danger">
                    {repo.lastError}
                  </div>
                ) : null}
              </Card.Content>

              <Card.Footer className="flex flex-wrap items-center gap-2 border-t border-separator">
                <Button
                  size="sm"
                  variant="secondary"
                  isPending={pending?.id === repo.id && pending?.type === "toggle"}
                  onPress={() => handleToggle(repo)}
                >
                  {repo.enabled ? (
                    <PauseIcon className="size-4" />
                  ) : (
                    <PlayIcon className="size-4" />
                  )}
                  {repo.enabled ? "暂停" : "恢复"}
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  isPending={pending?.id === repo.id && pending?.type === "commit"}
                  onPress={() => handleCommit(repo)}
                >
                  <SendIcon className="size-4" />
                  立即提交
                </Button>
                <div className="ml-auto flex items-center gap-1">
                  <Button
                    size="sm"
                    variant="ghost"
                    isIconOnly
                    aria-label="编辑"
                    onPress={() => openEdit(repo)}
                  >
                    <PencilIcon className="size-4" />
                  </Button>
                  <Button
                    size="sm"
                    variant="danger-soft"
                    isIconOnly
                    aria-label="删除"
                    onPress={() => setDeleting(repo)}
                  >
                    <TrashIcon className="size-4" />
                  </Button>
                </div>
              </Card.Footer>
            </Card>
          ))}
        </div>
      )}

      <RepoFormModal
        open={formOpen}
        repo={editing}
        onClose={() => setFormOpen(false)}
        onSaved={load}
      />

      <ConfirmModal
        open={deleting !== null}
        title="删除仓库"
        description={`确定要删除「${deleting?.name}」吗？该操作会同时移除本地仓库目录，且不可恢复。`}
        confirmText="删除"
        loading={deleteLoading}
        onConfirm={handleDelete}
        onClose={() => setDeleting(null)}
      />
    </div>
  );
}
