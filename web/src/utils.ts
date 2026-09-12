export function shortHash(hash: string): string {
  return hash.slice(0, 7);
}

/** 完整本地化时间（YYYY/MM/DD HH:mm）。 */
export function formatTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

/** 相对时间（刚刚 / x 分钟前 / x 小时前 / x 天前）。 */
export function relativeTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";

  const diff = Date.now() - date.getTime();
  const sec = Math.floor(diff / 1000);
  if (sec < 0) return "刚刚";
  if (sec < 60) return "刚刚";

  const min = Math.floor(sec / 60);
  if (min < 60) return `${min} 分钟前`;

  const hour = Math.floor(min / 60);
  if (hour < 24) return `${hour} 小时前`;

  const day = Math.floor(hour / 24);
  if (day < 30) return `${day} 天前`;

  return formatTime(iso);
}

/** 返回 YYYY-MM-DD 格式的本地日期 key。 */
export function dateKey(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

export function todayKey(): string {
  return dateKey(new Date());
}

/** 返回最近 n 天的日期 key（升序，最旧在前）。 */
export function lastNDays(n: number): string[] {
  const out: string[] = [];
  const now = new Date();
  for (let i = n - 1; i >= 0; i--) {
    const d = new Date(now);
    d.setDate(now.getDate() - i);
    out.push(dateKey(d));
  }
  return out;
}

/** 将 YYYY-MM-DD 转换为 MM-DD 短标签。 */
export function monthDay(key: string): string {
  return key.slice(5);
}
