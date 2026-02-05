import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { getRuns } from "@/api/runs";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatRelativeTime, formatDuration } from "@/lib/utils";
import { RunStatusBadge } from "@/components/runs/run-status-badge";
import { GitBranch, Clock, ArrowRight } from "lucide-react";

export const Route = createFileRoute("/_authenticated/$teamSlug/$appSlug/")({
  component: AppRunsPage,
});

function AppRunsPage() {
  const { teamSlug, appSlug } = Route.useParams();

  const { data: runs, isLoading } = useQuery({
    queryKey: ["runs", teamSlug, appSlug],
    queryFn: () =>
      getRuns(teamSlug, appSlug, {
        limit: 50,
        order_by: "started_at",
        order_dir: "desc",
      }),
  });

  if (isLoading) {
    return <Skeleton className="h-64 w-full" />;
  }

  if (!runs || runs.length === 0) {
    return (
      <Card>
        <CardContent className="py-12 text-center">
          <p className="text-muted-foreground mb-2">No runs yet</p>
          <p className="text-sm text-muted-foreground">
            Start tracking with the SDK using project:{" "}
            <code className="bg-muted px-1 rounded">
              {teamSlug}/{appSlug}
            </code>
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Branch</TableHead>
            <TableHead>Duration</TableHead>
            <TableHead>Started</TableHead>
            <TableHead></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {runs.map((run) => (
            <TableRow key={run.id}>
              <TableCell className="font-medium">
                <Link
                  to="/runs/$runId"
                  params={{ runId: run.id }}
                  className="hover:underline"
                >
                  {run.name}
                </Link>
                {run.tags && run.tags.length > 0 && (
                  <div className="flex gap-1 mt-1">
                    {run.tags.slice(0, 3).map((tag) => (
                      <Badge key={tag} variant="outline" className="text-xs">
                        {tag}
                      </Badge>
                    ))}
                  </div>
                )}
              </TableCell>
              <TableCell>
                <RunStatusBadge status={run.status} />
              </TableCell>
              <TableCell>
                {run.git_info?.branch && (
                  <div className="flex items-center gap-1 text-sm text-muted-foreground">
                    <GitBranch className="h-3 w-3" />
                    {run.git_info.branch}
                  </div>
                )}
              </TableCell>
              <TableCell>
                {run.duration_seconds !== undefined ? (
                  <div className="flex items-center gap-1 text-sm text-muted-foreground">
                    <Clock className="h-3 w-3" />
                    {formatDuration(run.duration_seconds)}
                  </div>
                ) : run.status === "running" ? (
                  <span className="text-sm text-muted-foreground">
                    Running...
                  </span>
                ) : (
                  "-"
                )}
              </TableCell>
              <TableCell className="text-muted-foreground text-sm">
                {formatRelativeTime(run.started_at)}
              </TableCell>
              <TableCell>
                <Link to="/runs/$runId" params={{ runId: run.id }}>
                  <ArrowRight className="h-4 w-4 text-muted-foreground hover:text-foreground" />
                </Link>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Card>
  );
}
