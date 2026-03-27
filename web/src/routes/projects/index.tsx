import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { getProjects } from "@/api/projects";
import type { Project } from "@/api/projects";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { FolderOpen, Activity } from "lucide-react";
import { formatRelativeTime } from "@/lib/utils";

export const Route = createFileRoute("/projects/")({
  component: ProjectsPage,
});

function projectHref(project: Project): string {
  if (project.source === "remote" && project.team_slug) {
    return `/projects/${project.slug}?source=remote&team=${encodeURIComponent(project.team_slug)}`;
  }
  return `/projects/${project.slug}`;
}

function ProjectsPage() {
  const { data: projects, isLoading } = useQuery({
    queryKey: ["projects"],
    queryFn: getProjects,
    refetchInterval: 5000, // Poll for new projects
  });

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          <Skeleton className="h-32" />
          <Skeleton className="h-32" />
          <Skeleton className="h-32" />
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Projects</h1>
        <p className="text-muted-foreground">
          View and monitor your ML experiments
        </p>
      </div>

      {!projects || projects.length === 0 ? (
        <Card>
          <CardContent className="py-12">
            <div className="text-center">
              <FolderOpen className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
              <h3 className="text-lg font-medium mb-2">No projects yet</h3>
              <p className="text-muted-foreground max-w-sm mx-auto">
                Start logging metrics from your Python code to see them here.
              </p>
              <pre className="mt-4 bg-muted p-4 rounded-md text-left max-w-md mx-auto text-sm overflow-x-auto">
                {`from p95 import Run

with Run(project="my-project") as run:
    run.log_metrics({"loss": 0.5}, step=0)`}
              </pre>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {projects.map((project) => (
            <a
              key={`${project.source}-${project.team_slug}-${project.slug}`}
              href={projectHref(project)}
              className="block"
            >
              <Card className="hover:bg-muted/50 transition-colors cursor-pointer">
                <CardHeader className="pb-2">
                  <CardTitle className="flex items-center gap-2">
                    <Activity className="h-5 w-5" />
                    {project.name}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center justify-between text-sm">
                    <div className="flex items-center gap-2">
                      <Badge variant="secondary">
                        {project.run_count} run
                        {project.run_count !== 1 ? "s" : ""}
                      </Badge>
                      {project.source === "remote" && (
                        <Badge variant="outline" className="text-xs">
                          Cloud
                        </Badge>
                      )}
                    </div>
                    {project.last_updated && (
                      <span className="text-muted-foreground">
                        {formatRelativeTime(project.last_updated)}
                      </span>
                    )}
                  </div>
                </CardContent>
              </Card>
            </a>
          ))}
        </div>
      )}
    </div>
  );
}
