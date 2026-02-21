import { Status } from "../models/types";

export const STATUS_LABELS: Record<Status, string> = {
  SUCCEEDED: "Succeeded",
  FAILED: "Failed",
  RUNNING: "Running",
  PENDING: "Pending",
  CANCELLED: "Cancelled",
  TIMEOUT: "Timeout"
};

export function statusTone(status: Status) {
  switch (status) {
    case "SUCCEEDED":
      return "bg-success/10 text-success border-success/30";
    case "FAILED":
      return "bg-danger/10 text-danger border-danger/30";
    case "RUNNING":
      return "bg-info/10 text-info border-info/30";
    case "TIMEOUT":
      return "bg-warning/10 text-warning border-warning/30";
    case "CANCELLED":
      return "bg-ink-100 text-ink-500 border-ink-200";
    default:
      return "bg-ink-100 text-ink-500 border-ink-200";
  }
}
