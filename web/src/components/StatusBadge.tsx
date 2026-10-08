import {
  CheckCircle2,
  Loader2,
  AlertCircle,
  Circle,
  type LucideIcon,
} from "lucide-react";

export type PolicyStatus = "idle" | "applying" | "applied" | "error";

const CONFIG: Record<
  PolicyStatus,
  { icon: LucideIcon; label: string; className: string }
> = {
  applied: {
    icon: CheckCircle2,
    label: "applied",
    className:
      "bg-green-50 text-green-700 dark:bg-green-950 dark:text-green-400",
  },
  applying: {
    icon: Loader2,
    label: "applying",
    className:
      "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400",
  },
  error: {
    icon: AlertCircle,
    label: "error",
    className: "bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-300",
  },
  idle: {
    icon: Circle,
    label: "not applied",
    className:
      "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400",
  },
};

export function StatusBadge({
  status,
  label,
}: {
  status: PolicyStatus;
  label?: string;
}) {
  const { icon: Icon, label: defaultLabel, className } = CONFIG[status];
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium ${className}`}
    >
      <Icon
        size={12}
        className={status === "applying" ? "animate-spin" : undefined}
      />
      {label ?? defaultLabel}
    </span>
  );
}
