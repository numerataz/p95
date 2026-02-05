import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { getTeam } from "@/api/teams";
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
import { CreateAppDialog } from "@/components/apps/create-app-dialog";
import { FolderKanban, ArrowRight } from "lucide-react";

export const Route = createFileRoute("/_authenticated/$teamSlug/")({
  component: TeamIndexPage,
});

function TeamIndexPage() {
  const params = Route.useParams();
  const teamSlug = params.teamSlug;

  const { data: team, isLoading: teamLoading } = useQuery({
    queryKey: ["team", teamSlug],
    queryFn: () => getTeam(teamSlug),
  });

  const { data: apps, isLoading: appsLoading } = useQuery({
    queryKey: ["apps", teamSlug],
    queryFn: () => getApps(teamSlug),
  });

  const isLoading = teamLoading || appsLoading;

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-32" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">{team?.name}</h1>
        {team?.description && (
          <p className="text-muted-foreground">{team.description}</p>
        )}
      </div>

      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold">Apps</h2>
          <CreateAppDialog teamSlug={teamSlug} />
        </div>
        {apps && apps.length > 0 ? (
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {apps.map((app) => (
              <Link
                key={app.id}
                to="/$teamSlug/$appSlug"
                params={{ teamSlug, appSlug: app.slug }}
              >
                <Card className="hover:border-primary/50 transition-colors cursor-pointer">
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <FolderKanban className="h-5 w-5 text-muted-foreground" />
                        <CardTitle className="text-lg">{app.name}</CardTitle>
                      </div>
                      <ArrowRight className="h-4 w-4 text-muted-foreground" />
                    </div>
                    {app.description && (
                      <CardDescription className="line-clamp-2">
                        {app.description}
                      </CardDescription>
                    )}
                  </CardHeader>
                  <CardContent className="pt-0">
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      <Badge variant="outline">{app.visibility}</Badge>
                      {app.run_count !== undefined && (
                        <span>{app.run_count} runs</span>
                      )}
                    </div>
                  </CardContent>
                </Card>
              </Link>
            ))}
          </div>
        ) : (
          <Card>
            <CardContent className="py-8 text-center text-muted-foreground">
              No apps yet. Create one to start tracking experiments.
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}
