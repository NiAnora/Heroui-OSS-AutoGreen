import { useCallback, useEffect, useState } from "react";
import {
  Button,
  Card,
  Chip,
  Input,
  Label,
  ListBox,
  Select,
  Spinner,
  TextField,
  toast,
} from "@heroui/react";
import { api } from "../api";
import type { DeviceFlow, ScheduleConfig, ScheduleMode } from "../types";
import { CheckCircleIcon, ClockIcon, XCircleIcon } from "../components/icons";

/** 时间范围表单：输入框统一用字符串，保存时再解析校验。 */
interface RangeForm {
  hourFrom: string;
  hourTo: string;
  minuteFrom: string;
  minuteTo: string;
  secondFrom: string;
  secondTo: string;
}

const DEFAULT_RANGE: RangeForm = {
  hourFrom: "0",
  hourTo: "23",
  minuteFrom: "0",
  minuteTo: "59",
  secondFrom: "0",
  secondTo: "59",
};

function toRangeForm(cfg: ScheduleConfig): RangeForm {
  return {
    hourFrom: String(cfg.hourFrom),
    hourTo: String(cfg.hourTo),
    minuteFrom: String(cfg.minuteFrom),
    minuteTo: String(cfg.minuteTo),
    secondFrom: String(cfg.secondFrom),
    secondTo: String(cfg.secondTo),
  };
}

/** 解析范围输入：去空、夹取到 [0,max]，非法值回退到默认上界。 */
function parseRange(from: string, to: string, max: number): [number, number] {
  const pick = (raw: string, fallback: number) => {
    const n = Number.parseInt(raw, 10);
    if (Number.isNaN(n)) return fallback;
    return Math.min(max, Math.max(0, n));
  };
  return [pick(from, 0), pick(to, max)];
}

/** 一行时间范围输入：标签 + 起止两个数字框。 */
function RangeRow({
  label,
  suffix,
  from,
  to,
  max,
  onFrom,
  onTo,
}: {
  label: string;
  suffix: string;
  from: string;
  to: string;
  max: number;
  onFrom: (value: string) => void;
  onTo: (value: string) => void;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <span className="text-sm text-foreground">{label}</span>
      <div className="flex items-center gap-2">
        <TextField className="w-24" type="number" value={from} onChange={onFrom}>
          <Label className="sr-only">{`起始${label}`}</Label>
          <Input aria-label={`起始${label}`} />
        </TextField>
        <span className="text-sm text-muted">–</span>
        <TextField className="w-24" type="number" value={to} onChange={onTo}>
          <Label className="sr-only">{`结束${label}`}</Label>
          <Input aria-label={`结束${label}`} />
        </TextField>
        <span className="text-xs text-muted">
          {suffix}（0-{max}）
        </span>
      </div>
    </div>
  );
}

export default function Settings() {
  const [tokenSet, setTokenSet] = useState(false);
  const [tokenPreview, setTokenPreview] = useState("");
  const [clientId, setClientId] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [token, setToken] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  const [deviceFlow, setDeviceFlow] = useState<DeviceFlow | null>(null);
  const [polling, setPolling] = useState(false);
  const [starting, setStarting] = useState(false);

  const [mode, setMode] = useState<ScheduleMode>("weekly");
  const [weekDays, setWeekDays] = useState(3);
  const [range, setRange] = useState<RangeForm>(DEFAULT_RANGE);
  const [savingSchedule, setSavingSchedule] = useState(false);
  const [scheduleSaved, setScheduleSaved] = useState(false);

  const load = useCallback(async () => {
    try {
      const settings = await api.getSettings();
      setTokenSet(settings.tokenSet);
      setTokenPreview(settings.tokenPreview);
      setClientId(settings.clientId);
      setMode(settings.schedule.mode);
      setWeekDays(settings.schedule.weekDays);
      setRange(toRangeForm(settings.schedule));
    } catch (error) {
      toast.danger((error as Error).message);
    } finally {
      setLoaded(true);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const setRangeField = (key: keyof RangeForm, value: string) => {
    setRange((prev) => ({ ...prev, [key]: value }));
    setScheduleSaved(false);
  };

  const handleSaveSchedule = async () => {
    setSavingSchedule(true);
    setScheduleSaved(false);
    try {
      const [hourFrom, hourTo] = parseRange(range.hourFrom, range.hourTo, 23);
      const [minuteFrom, minuteTo] = parseRange(range.minuteFrom, range.minuteTo, 59);
      const [secondFrom, secondTo] = parseRange(range.secondFrom, range.secondTo, 59);
      const payload: ScheduleConfig = {
        mode,
        weekDays,
        hourFrom,
        hourTo,
        minuteFrom,
        minuteTo,
        secondFrom,
        secondTo,
      };
      const settings = await api.updateSettings({ schedule: payload });
      setMode(settings.schedule.mode);
      setWeekDays(settings.schedule.weekDays);
      setRange(toRangeForm(settings.schedule));
      setScheduleSaved(true);
      toast.success("提交计划已保存，所有仓库立即生效");
    } catch (error) {
      toast.danger((error as Error).message);
    } finally {
      setSavingSchedule(false);
    }
  };

  const handleSave = async () => {
    if (!clientId.trim() && !token.trim()) {
      toast.warning("请填写 Client ID 或 Token");
      return;
    }
    setSaving(true);
    setSaved(false);
    try {
      const settings = await api.updateSettings({
        clientId: clientId.trim() || undefined,
        token: token.trim() || undefined,
      });
      setTokenSet(settings.tokenSet);
      setTokenPreview(settings.tokenPreview);
      setClientId(settings.clientId);
      setToken("");
      setSaved(true);
      toast.success("设置已保存");
    } catch (error) {
      toast.danger((error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const handleStartAuth = async () => {
    setStarting(true);
    try {
      const st = await api.startDeviceAuth();
      setDeviceFlow(st);
      if (st.verificationUri) {
        window.open(st.verificationUri, "_blank");
      }
      setPolling(true);
    } catch (error) {
      toast.danger((error as Error).message);
    } finally {
      setStarting(false);
    }
  };

  useEffect(() => {
    if (!polling) return;
    const timer = setInterval(async () => {
      try {
        const st = await api.deviceAuthStatus();
        setDeviceFlow(st);
        if (st.status === "success") {
          setPolling(false);
          toast.success("授权成功，Token 已自动保存");
          await load();
        } else if (st.status === "error") {
          setPolling(false);
          toast.danger(st.message || "授权失败");
        }
      } catch (error) {
        setPolling(false);
        toast.danger((error as Error).message);
      }
    }, 3000);
    return () => clearInterval(timer);
  }, [polling, load]);

  return (
    <div className="mx-auto w-full max-w-3xl px-6 pb-12 pt-8">
      <header className="mb-8">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">设置</h1>
        <p className="mt-1 text-sm text-muted">配置 GitHub 凭据与全局提交计划</p>
      </header>

      <div className="flex flex-col gap-6">
        <Card>
          <Card.Header>
            <Card.Title>GitHub 授权</Card.Title>
            <Card.Description>授权后可读取你的仓库列表并推送提交</Card.Description>
          </Card.Header>
          <Card.Content className="flex flex-col gap-5">
            <div className="flex items-center gap-3 rounded-xl border border-default-200 bg-surface-secondary/40 p-4">
              <div
                className={`flex size-9 shrink-0 items-center justify-center rounded-lg ${
                  tokenSet ? "bg-success/10 text-success" : "bg-warning/10 text-warning"
                }`}
              >
                {tokenSet ? (
                  <CheckCircleIcon className="size-5" />
                ) : (
                  <XCircleIcon className="size-5" />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span className="text-sm font-medium text-foreground">
                    {tokenSet ? "已授权" : "未授权"}
                  </span>
                  {tokenSet && tokenPreview ? (
                    <span className="font-mono text-xs text-muted">{tokenPreview}</span>
                  ) : null}
                </div>
                <p className="mt-0.5 text-xs leading-relaxed text-muted">
                  {tokenSet
                    ? "凭据已保存到本地，自动提交可正常运行"
                    : "尚未配置凭据，点击下方按钮完成授权"}
                </p>
              </div>
            </div>

            <div>
              <Button
                variant="primary"
                isPending={starting}
                isDisabled={polling}
                onPress={handleStartAuth}
              >
                使用 GitHub 一键授权
              </Button>
            </div>

            {deviceFlow && deviceFlow.status === "pending" ? (
              <div className="flex flex-col gap-3 rounded-xl border border-default-200 bg-surface-secondary/40 p-4">
                <p className="text-sm text-foreground">
                  已在浏览器打开 GitHub 授权页，输入以下验证码并确认授权：
                </p>
                <p className="font-mono text-2xl font-bold tracking-[0.3em] text-accent">
                  {deviceFlow.userCode}
                </p>
                <div className="flex items-center gap-2.5">
                  <Spinner size="sm" />
                  <span className="text-sm text-muted">等待授权中，请勿关闭此页面…</span>
                </div>
                {deviceFlow.verificationUri ? (
                  <div>
                    <Button
                      variant="outline"
                      size="sm"
                      onPress={() => window.open(deviceFlow.verificationUri, "_blank")}
                    >
                      重新打开授权页面
                    </Button>
                  </div>
                ) : null}
              </div>
            ) : null}

            {deviceFlow && deviceFlow.status === "error" ? (
              <div className="rounded-xl bg-danger/10 p-4 text-sm leading-relaxed text-danger">
                {deviceFlow.message || "授权失败，请重试"}
              </div>
            ) : null}
          </Card.Content>
        </Card>

        <Card>
          <Card.Header>
            <Card.Title>提交计划</Card.Title>
            <Card.Description>全局生效，所有已启用仓库共用同一套调度规则</Card.Description>
          </Card.Header>
          <Card.Content className="flex flex-col gap-5">
            <Select
              className="w-full"
              value={mode}
              onChange={(value) => {
                if (typeof value === "string") {
                  setMode(value as ScheduleMode);
                  setScheduleSaved(false);
                }
              }}
            >
              <Label>调度方式</Label>
              <Select.Trigger>
                <Select.Value />
                <Select.Indicator />
              </Select.Trigger>
              <Select.Popover>
                <ListBox>
                  <ListBox.Item id="weekly" textValue="每周随机天数">
                    每周随机天数
                    <ListBox.ItemIndicator />
                  </ListBox.Item>
                  <ListBox.Item id="daily" textValue="每天随机提交">
                    每天随机提交
                    <ListBox.ItemIndicator />
                  </ListBox.Item>
                </ListBox>
              </Select.Popover>
            </Select>

            {mode === "weekly" ? (
              <Select
                className="w-full"
                value={String(weekDays)}
                onChange={(value) => {
                  if (typeof value === "string") {
                    setWeekDays(Number(value));
                    setScheduleSaved(false);
                  }
                }}
              >
                <Label>每周提交天数</Label>
                <Select.Trigger>
                  <Select.Value />
                  <Select.Indicator />
                </Select.Trigger>
                <Select.Popover>
                  <ListBox>
                    {[1, 2, 3, 4, 5, 6, 7].map((n) => (
                      <ListBox.Item key={n} id={String(n)} textValue={`${n} 天`}>
                        {n} 天
                        <ListBox.ItemIndicator />
                      </ListBox.Item>
                    ))}
                  </ListBox>
                </Select.Popover>
              </Select>
            ) : (
              <div className="rounded-xl bg-surface-secondary/60 px-3 py-2.5 text-xs leading-relaxed text-muted">
                每天以 1/2 的真随机概率决定该天是否提交（0 不提交 / 1 提交）。
              </div>
            )}

            <div className="h-px w-full bg-separator" />

            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <ClockIcon className="size-4 text-accent" />
                <span className="text-sm font-medium text-foreground">提交时间范围</span>
              </div>
              <p className="text-xs leading-relaxed text-muted">
                提交时刻的小时/分钟/秒分别在下列区间内取真随机值，区间内任意一秒概率均等。
              </p>
            </div>

            <div className="flex flex-col gap-4">
              <RangeRow
                label="小时"
                suffix="时"
                max={23}
                from={range.hourFrom}
                to={range.hourTo}
                onFrom={(v) => setRangeField("hourFrom", v)}
                onTo={(v) => setRangeField("hourTo", v)}
              />
              <RangeRow
                label="分钟"
                suffix="分"
                max={59}
                from={range.minuteFrom}
                to={range.minuteTo}
                onFrom={(v) => setRangeField("minuteFrom", v)}
                onTo={(v) => setRangeField("minuteTo", v)}
              />
              <RangeRow
                label="秒"
                suffix="秒"
                max={59}
                from={range.secondFrom}
                to={range.secondTo}
                onFrom={(v) => setRangeField("secondFrom", v)}
                onTo={(v) => setRangeField("secondTo", v)}
              />
            </div>

            <p className="text-xs leading-relaxed text-muted">
              例：小时设为 9 – 22、分钟与秒设为 0 – 59，则提交只会落在每天 09:00:00 至
              22:59:59 之间，不会出现凌晨记录。同一自然日至多提交一次。
            </p>
          </Card.Content>
          <Card.Footer className="flex items-center gap-3 border-t border-separator">
            <Button variant="primary" isPending={savingSchedule} onPress={handleSaveSchedule}>
              保存计划
            </Button>
            {scheduleSaved ? (
              <Chip color="success" variant="soft" size="sm">
                已保存并生效
              </Chip>
            ) : null}
          </Card.Footer>
        </Card>

        <Card>
          <Card.Header>
            <Card.Title>手动配置</Card.Title>
            <Card.Description>也可手动粘贴凭据，或覆盖默认的 Client ID</Card.Description>
          </Card.Header>
          <Card.Content className="flex flex-col gap-5">
            <div>
              <TextField
                className="w-full"
                value={clientId}
                onChange={(value) => {
                  setClientId(value);
                  setSaved(false);
                }}
              >
                <Label>Client ID</Label>
                <Input placeholder="Iv1.xxxxxxxxxxxxxxxx" />
              </TextField>
              <p className="mt-1.5 text-xs leading-relaxed text-muted">
                已内置默认 Client ID，通常无需修改。如需自定义，可在{" "}
                <a
                  className="text-accent underline"
                  href="https://github.com/settings/developers"
                  target="_blank"
                  rel="noreferrer"
                >
                  GitHub OAuth Apps
                </a>{" "}
                注册应用后填入。
              </p>
            </div>

            <div>
              <TextField
                className="w-full"
                type="password"
                value={token}
                onChange={(value) => {
                  setToken(value);
                  setSaved(false);
                }}
              >
                <Label>Personal Access Token</Label>
                <Input placeholder="ghp_xxxxxxxxxxxxxxxx" />
              </TextField>
              <p className="mt-1.5 text-xs leading-relaxed text-muted">
                需具备 <code className="rounded bg-surface-secondary px-1 py-0.5">repo</code>{" "}
                权限。留空则保持现有凭据不变。
              </p>
            </div>
          </Card.Content>
          <Card.Footer className="flex items-center gap-3 border-t border-separator">
            <Button variant="primary" isPending={saving} onPress={handleSave}>
              保存
            </Button>
            {saved ? (
              <Chip color="success" variant="soft" size="sm">
                已保存
              </Chip>
            ) : null}
            {!loaded ? <Spinner size="sm" /> : null}
          </Card.Footer>
        </Card>
      </div>
    </div>
  );
}
