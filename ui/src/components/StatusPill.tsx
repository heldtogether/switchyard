import { Status } from "../models/types";
import { STATUS_LABELS, statusTone } from "../utils/status";
import clsx from "clsx";

interface StatusPillProps {
  status: Status;
}

export function StatusPill({ status }: StatusPillProps) {
  return (
    <span
      className={clsx(
        "inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-semibold",
        statusTone(status)
      )}
    >
      {STATUS_LABELS[status]}
    </span>
  );
}
