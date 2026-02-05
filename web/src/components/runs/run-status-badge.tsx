import { Badge } from "@/components/ui/badge";
import type { RunStatus } from "@/api/types";
import { CheckCircle, XCircle, Loader2, StopCircle, Ban } from "lucide-react";

interface RunStatusBadgeProps {
  status: RunStatus;
}

const statusConfig: Record<
  RunStatus,
  {
    label: string;
    variant:
      | "default"
      | "secondary"
      | "destructive"
      | "outline"
      | "success"
      | "warning";
    icon: React.ComponentType<{ className?: string }>;
  }
> = {
  running: {
    label: "Running",
    variant: "default",
    icon: Loader2,
  },
  completed: {
    label: "Completed",
    variant: "success",
    icon: CheckCircle,
  },
  failed: {
    label: "Failed",
    variant: "destructive",
    icon: XCircle,
  },
  aborted: {
    label: "Aborted",
    variant: "warning",
    icon: StopCircle,
  },
  canceled: {
    label: "Canceled",
    variant: "secondary",
    icon: Ban,
  },
};

export function RunStatusBadge({ status }: RunStatusBadgeProps) {
  const config = statusConfig[status];
  const Icon = config.icon;

  return (
    <Badge variant={config.variant} className="gap-1">
      <Icon
        className={`h-3 w-3 ${status === "running" ? "animate-spin" : ""}`}
      />
      {config.label}
    </Badge>
  );
}
