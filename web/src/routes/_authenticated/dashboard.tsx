import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery, useQueries } from "@tanstack/react-query";
import { getTeams } from "@/api/teams";
import { getApps } from "@/api/apps";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Users, FolderKanban, Activity } from "lucide-react";

export const Route = createFileRoute("/_authenticated/dashboard")({
  component: DashboardPage,
});

function DashboardPage() {
  const { data: teams, isLoading: teamsLoading } = useQuery({
    queryKey: ["teams"],
    queryFn: getTeams,
  });

  // Fetch apps for all teams to get counts
  const appsQueries = useQueries({
    queries: (teams || []).map((team) => ({
      queryKey: ["apps", team.slug],
      queryFn: () => getApps(team.slug),
      enabled: !!teams,
    })),
  });

  const isLoading = teamsLoading;
  const appsLoading = appsQueries.some((q) => q.isLoading);

  // Calculate totals
  const totalApps = appsQueries.reduce(
    (sum, q) => sum + (q.data?.length || 0),
    0,
  );
  const totalRuns = appsQueries.reduce(
    (sum, q) =>
      sum + (q.data?.reduce((s, app) => s + (app.run_count || 0), 0) || 0),
    0,
  );

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-muted-foreground">
          Overview of your teams and experiments
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Teams</CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {isLoading ? (
                <Skeleton className="h-8 w-12" />
              ) : (
                (teams?.length ?? 0)
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Apps</CardTitle>
            <FolderKanban className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {appsLoading ? <Skeleton className="h-8 w-12" /> : totalApps}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Runs</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {appsLoading ? <Skeleton className="h-8 w-12" /> : totalRuns}
            </div>
          </CardContent>
        </Card>
      </div>

      <div>
        <h2 className="text-xl font-semibold mb-4">Your Teams</h2>
        {isLoading ? (
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {[1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-32" />
            ))}
          </div>
        ) : teams && teams.length > 0 ? (
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {teams.map((team) => (
              <Link
                key={team.id}
                to="/$teamSlug"
                params={{ teamSlug: team.slug }}
              >
                <Card className="hover:border-primary/50 transition-colors cursor-pointer">
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <CardTitle className="text-lg">{team.name}</CardTitle>
                      <Badge variant="secondary">{team.role}</Badge>
                    </div>
                    {team.description && (
                      <CardDescription className="line-clamp-2">
                        {team.description}
                      </CardDescription>
                    )}
                  </CardHeader>
                </Card>
              </Link>
            ))}
          </div>
        ) : (
          <Card>
            <CardContent className="py-8 text-center text-muted-foreground">
              No teams yet. Create one to get started.
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}
