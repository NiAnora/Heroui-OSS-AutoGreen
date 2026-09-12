import { useEffect, useState } from "react";
import {
  Button,
  Input,
  Label,
  ListBox,
  Modal,
  Switch,
  TextField,
  toast,
} from "@heroui/react";
import type { GithubRepo, Repo, RepoInput } from "../types";
import { api } from "../api";

interface RepoFormModalProps {
  open: boolean;
  repo: Repo | null;
  onClose: () => void;
  onSaved: () => void;
}

interface FormState {
  name: string;
  repoUrl: string;
  branch: string;
  userName: string;
  userEmail: string;
  commitMsg: string;
  enabled: boolean;
}

const EMPTY_FORM: FormState = {
  name: "",
  repoUrl: "",
  branch: "main",
  userName: "AutoGreen",
  userEmail: "autogreen@users.noreply.github.com",
  commitMsg: "chore: auto commit",
  enabled: true,
};

export default function RepoFormModal({ open, repo, onClose, onSaved }: RepoFormModalProps) {
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [ghRepos, setGhRepos] = useState<GithubRepo[]>([]);
  const [loadingRepos, setLoadingRepos] = useState(false);

  useEffect(() => {
    if (!open) return;
    setForm(
      repo
        ? {
            name: repo.name,
            repoUrl: repo.repoUrl,
            branch: repo.branch || "main",
            userName: repo.userName,
            userEmail: repo.userEmail,
            commitMsg: repo.commitMsg,
            enabled: repo.enabled,
          }
        : EMPTY_FORM,
    );
  }, [open, repo]);

  const setField = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }));

  const openPicker = async () => {
    setPickerOpen(true);
    setLoadingRepos(true);
    setGhRepos([]);
    try {
      const repos = await api.listGithubRepos();
      setGhRepos(repos);
      if (repos.length === 0) {
        toast.info("未找到可访问的仓库");
      }
    } catch (error) {
      toast.danger((error as Error).message);
    } finally {
      setLoadingRepos(false);
    }
  };

  const pickRepo = (r: GithubRepo) => {
    setField("name", r.fullName);
    setField("repoUrl", r.cloneUrl);
    setField("branch", r.defaultBranch || "main");
    setPickerOpen(false);
  };

  const handleSave = async () => {
    if (!form.name.trim() || !form.repoUrl.trim()) {
      toast.warning("请填写仓库名称与仓库地址");
      return;
    }
    setSaving(true);
    try {
      const input: RepoInput = { ...form };
      if (repo) {
        await api.updateRepo(repo.id, input);
        toast.success("仓库已更新");
      } else {
        await api.createRepo(input);
        toast.success("仓库已添加");
      }
      onClose();
      onSaved();
    } catch (error) {
      toast.danger((error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal.Backdrop isOpen={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <Modal.Container>
        <Modal.Dialog className="sm:max-w-lg">
          <Modal.CloseTrigger />
          <Modal.Header>
            <Modal.Heading>{repo ? "编辑仓库" : "添加仓库"}</Modal.Heading>
          </Modal.Header>
          <Modal.Body className="flex flex-col gap-5">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <TextField
                className="w-full"
                value={form.name}
                onChange={(value) => setField("name", value)}
                isRequired
              >
                <Label>名称</Label>
                <Input placeholder="my-repo" />
              </TextField>
              <TextField
                className="w-full"
                value={form.repoUrl}
                onChange={(value) => setField("repoUrl", value)}
                isRequired
              >
                <Label>仓库地址</Label>
                <Input placeholder="https://github.com/user/repo.git" />
              </TextField>
              <div className="flex items-end">
                <Button
                  variant="secondary"
                  className="w-full"
                  isPending={loadingRepos}
                  onPress={openPicker}
                >
                  从 GitHub 选择
                </Button>
              </div>
              <TextField
                className="w-full"
                value={form.branch}
                onChange={(value) => setField("branch", value)}
              >
                <Label>分支</Label>
                <Input placeholder="main" />
              </TextField>
              <TextField
                className="w-full"
                value={form.userName}
                onChange={(value) => setField("userName", value)}
              >
                <Label>用户名</Label>
                <Input placeholder="AutoGreen" />
              </TextField>
              <TextField
                className="w-full"
                type="email"
                value={form.userEmail}
                onChange={(value) => setField("userEmail", value)}
              >
                <Label>邮箱</Label>
                <Input placeholder="autogreen@users.noreply.github.com" />
              </TextField>
              <TextField
                className="w-full sm:col-span-2"
                value={form.commitMsg}
                onChange={(value) => setField("commitMsg", value)}
              >
                <Label>提交信息</Label>
                <Input placeholder="chore: auto commit" />
              </TextField>
            </div>

            <Switch isSelected={form.enabled} onChange={(value) => setField("enabled", value)}>
              <Switch.Content>
                <Switch.Control>
                  <Switch.Thumb />
                </Switch.Control>
                启用自动提交
              </Switch.Content>
            </Switch>
          </Modal.Body>
          <Modal.Footer>
            <Button variant="secondary" onPress={onClose}>
              取消
            </Button>
            <Button variant="primary" isPending={saving} onPress={handleSave}>
              {repo ? "保存" : "添加"}
            </Button>
          </Modal.Footer>
        </Modal.Dialog>
      </Modal.Container>

      <Modal.Backdrop isOpen={pickerOpen} onOpenChange={(isOpen) => !isOpen && setPickerOpen(false)}>
        <Modal.Container>
          <Modal.Dialog className="sm:max-w-lg">
            <Modal.CloseTrigger />
            <Modal.Header>
              <Modal.Heading>选择 GitHub 仓库</Modal.Heading>
            </Modal.Header>
            <Modal.Body className="max-h-[60vh] overflow-y-auto">
              {loadingRepos ? (
                <div className="py-8 text-center text-sm text-default-500">加载中…</div>
              ) : ghRepos.length === 0 ? (
                <div className="py-8 text-center text-sm text-default-500">暂无仓库</div>
              ) : (
                <ListBox aria-label="GitHub 仓库列表">
                  {ghRepos.map((r) => (
                    <ListBox.Item
                      key={r.fullName}
                      id={r.fullName}
                      textValue={r.fullName}
                      onAction={() => pickRepo(r)}
                    >
                      <span className="flex w-full items-center justify-between gap-3">
                        <span className="truncate font-medium">{r.fullName}</span>
                        <span className="shrink-0 text-xs text-default-400">
                          {r.private ? "私有" : "公开"}
                        </span>
                      </span>
                    </ListBox.Item>
                  ))}
                </ListBox>
              )}
            </Modal.Body>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal.Backdrop>
  );
}
