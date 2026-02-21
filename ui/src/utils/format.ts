import { formatDistanceToNow, format } from "date-fns";

export function relativeTime(value?: string | null) {
  if (!value) return "—";
  const date = new Date(value);
  return formatDistanceToNow(date, { addSuffix: true });
}

export function exactTime(value?: string | null) {
  if (!value) return "";
  return format(new Date(value), "yyyy-MM-dd HH:mm:ss");
}

export function formatBytes(bytes: number) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

export function formatDurationMs(ms?: number | null) {
  if (!ms) return "—";
  const sec = Math.floor(ms / 1000);
  const mins = Math.floor(sec / 60);
  const hrs = Math.floor(mins / 60);
  if (hrs > 0) return `${hrs}h ${mins % 60}m`;
  if (mins > 0) return `${mins}m ${sec % 60}s`;
  return `${sec}s`;
}
