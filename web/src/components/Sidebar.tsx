import type { Page } from "../App";
import { DashboardIcon, LeafIcon, RepoIcon, SettingsIcon } from "./icons";

interface SidebarProps {
  active: Page;
  onChange: (page: Page) => void;
}

const NAV_GROUPS: {
  label: string;
  items: { key: Page; label: string; icon: typeof DashboardIcon }[];
}[] = [
  {
    label: "概览",
    items: [
      { key: "dashboard", label: "仪表盘", icon: DashboardIcon },
      { key: "repos", label: "仓库管理", icon: RepoIcon },
    ],
  },
  {
    label: "配置",
    items: [{ key: "settings", label: "设置", icon: SettingsIcon }],
  },
];

export default function Sidebar({ active, onChange }: SidebarProps) {
  return (
    <aside className="sticky top-0 flex h-screen w-64 shrink-0 flex-col border-r border-border bg-background">
      <div className="flex items-center gap-2.5 px-5 pb-6 pt-7">
        <div className="flex size-9 items-center justify-center rounded-xl bg-accent text-accent-foreground">
          <LeafIcon className="size-5" />
        </div>
        <div className="flex flex-col">
          <span className="text-base font-semibold leading-tight text-foreground">AutoGreen</span>
          <span className="text-xs text-muted">贡献图常绿助手</span>
        </div>
      </div>

      <nav className="flex flex-1 flex-col gap-6 px-3">
        {NAV_GROUPS.map((group) => (
          <div key={group.label} className="flex flex-col gap-1">
            <p className="px-3 pb-1 text-xs font-medium text-muted">{group.label}</p>
            {group.items.map((item) => {
              const isActive = active === item.key;
              const Icon = item.icon;
              return (
                <button
                  key={item.key}
                  type="button"
                  onClick={() => onChange(item.key)}
                  aria-current={isActive ? "page" : undefined}
                  className={`flex items-center gap-2.5 rounded-xl px-3 py-2 text-sm font-medium transition-none ${
                    isActive
                      ? "bg-surface text-foreground shadow-sm"
                      : "text-muted hover:bg-surface-secondary hover:text-foreground"
                  }`}
                >
                  <Icon className="size-4" />
                  {item.label}
                </button>
              );
            })}
          </div>
        ))}
      </nav>

      <div className="border-t border-border px-5 py-4">
        <p className="text-xs text-muted">后端服务 · localhost:8080</p>
      </div>
    </aside>
  );
}
