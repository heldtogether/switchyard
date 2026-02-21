import React from "react";
import { exactTime, relativeTime } from "../utils/format";

export function RelativeTime({ value }: { value?: string | null }) {
  if (!value) return <span>—</span>;
  return <span title={exactTime(value)}>{relativeTime(value)}</span>;
}
