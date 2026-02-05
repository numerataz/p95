import { formatRelativeTime } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { Continuation } from "@/api/types";
import { RotateCcw, GitBranch, Plus, Minus, Pencil } from "lucide-react";

interface ContinuationsPanelProps {
  continuations: Continuation[];
}

interface ConfigDiff {
  key: string;
  type: "added" | "removed" | "modified";
  before?: unknown;
  after?: unknown;
}

function computeConfigDiff(
  before: Record<string, unknown> | undefined,
  after: Record<string, unknown> | undefined,
): ConfigDiff[] {
  const diffs: ConfigDiff[] = [];
  const beforeKeys = new Set(Object.keys(before || {}));
  const afterKeys = new Set(Object.keys(after || {}));

  // Find modified and removed keys
  for (const key of beforeKeys) {
    if (afterKeys.has(key)) {
      const beforeVal = before?.[key];
      const afterVal = after?.[key];
      if (JSON.stringify(beforeVal) !== JSON.stringify(afterVal)) {
        diffs.push({
          key,
          type: "modified",
          before: beforeVal,
          after: afterVal,
        });
      }
    } else {
      diffs.push({ key, type: "removed", before: before?.[key] });
    }
  }

  // Find added keys
  for (const key of afterKeys) {
    if (!beforeKeys.has(key)) {
      diffs.push({ key, type: "added", after: after?.[key] });
    }
  }

  return diffs.sort((a, b) => a.key.localeCompare(b.key));
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return "null";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function ConfigDiffBadge({ diff }: { diff: ConfigDiff }) {
  switch (diff.type) {
    case "added":
      return (
        <div className="flex items-center gap-2 text-sm">
          <Badge
            variant="outline"
            className="bg-green-500/10 text-green-600 border-green-500/30"
          >
            <Plus className="h-3 w-3 mr-1" />
            {diff.key}
          </Badge>
          <span className="text-muted-foreground">=</span>
          <code className="text-green-600 bg-green-500/10 px-1 rounded">
            {formatValue(diff.after)}
          </code>
        </div>
      );
    case "removed":
      return (
        <div className="flex items-center gap-2 text-sm">
          <Badge
            variant="outline"
            className="bg-red-500/10 text-red-600 border-red-500/30"
          >
            <Minus className="h-3 w-3 mr-1" />
            {diff.key}
          </Badge>
          <span className="text-muted-foreground line-through">
            {formatValue(diff.before)}
          </span>
        </div>
      );
    case "modified":
      return (
        <div className="flex items-center gap-2 text-sm flex-wrap">
          <Badge
            variant="outline"
            className="bg-yellow-500/10 text-yellow-600 border-yellow-500/30"
          >
            <Pencil className="h-3 w-3 mr-1" />
            {diff.key}
          </Badge>
          <code className="text-red-600 bg-red-500/10 px-1 rounded line-through">
            {formatValue(diff.before)}
          </code>
          <span className="text-muted-foreground">→</span>
          <code className="text-green-600 bg-green-500/10 px-1 rounded">
            {formatValue(diff.after)}
          </code>
        </div>
      );
  }
}

export function ContinuationsPanel({ continuations }: ContinuationsPanelProps) {
  if (!continuations || continuations.length === 0) {
    return (
      <Card>
        <CardContent className="py-8 text-center text-muted-foreground">
          No continuations - this run has not been resumed
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      {continuations.map((cont, index) => {
        const diffs = computeConfigDiff(cont.config_before, cont.config_after);

        return (
          <Card key={cont.id}>
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <CardTitle className="text-base flex items-center gap-2">
                  <RotateCcw className="h-4 w-4" />
                  Continuation {index + 1}
                </CardTitle>
                <div className="flex items-center gap-3 text-sm text-muted-foreground">
                  <span>Step {cont.step}</span>
                  <span>•</span>
                  <span>{formatRelativeTime(cont.timestamp)}</span>
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              {/* Note */}
              {cont.note && (
                <div className="bg-muted/50 rounded-md p-3 text-sm">
                  <span className="font-medium">Note:</span> {cont.note}
                </div>
              )}

              {/* Config Changes */}
              {diffs.length > 0 ? (
                <div className="space-y-2">
                  <h4 className="text-sm font-medium">Config Changes</h4>
                  <div className="space-y-1.5">
                    {diffs.map((diff) => (
                      <ConfigDiffBadge key={diff.key} diff={diff} />
                    ))}
                  </div>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">
                  No config changes
                </p>
              )}

              {/* Git Info */}
              {cont.git_info && cont.git_info.commit && (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <GitBranch className="h-3 w-3" />
                  <span>{cont.git_info.branch}</span>
                  <span className="font-mono">
                    {cont.git_info.commit?.slice(0, 7)}
                  </span>
                </div>
              )}
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}
