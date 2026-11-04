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
    className: "border border-success-border bg-success-soft text-success",
  },
  applying: {
    icon: Loader2,
    label: "applying",
    className: "border border-border bg-secondary text-muted-foreground",
  },
  error: {
    icon: AlertCircle,
    label: "error",
    className:
      "border border-destructive-border bg-destructive-soft text-destructive",
  },
  idle: {
    icon: Circle,
    label: "not applied",
    className: "border border-border bg-secondary text-muted-foreground",
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
