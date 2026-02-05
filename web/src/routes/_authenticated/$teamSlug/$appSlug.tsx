import {
  createFileRoute,
  Link,
  Outlet,
  useParams,
  useLocation,
} from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { getApp } from "@/api/apps";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { FolderKanban, Play, Settings } from "lucide-react";

export const Route = createFileRoute("/_authenticated/$teamSlug/$appSlug")({
  component: AppLayout,
});

function AppLayout() {
  const { teamSlug, appSlug } = useParams({
    from: "/_authenticated/$teamSlug/$appSlug",
  });
  const location = useLocation();

  const { data: app, isLoading } = useQuery({
    queryKey: ["app", teamSlug, appSlug],
    queryFn: () => getApp(teamSlug, appSlug),
  });

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  const basePath = `/${teamSlug}/${appSlug}`;
  const isRunsActive =
    location.pathname === basePath || location.pathname === `${basePath}/`;
  const isSettingsActive = location.pathname === `${basePath}/settings`;

  const tabs = [
    { href: basePath, label: "Runs", icon: Play, active: isRunsActive },
    {
      href: `${basePath}/settings`,
      label: "Settings",
      icon: Settings,
      active: isSettingsActive,
    },
  ];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="flex items-center gap-3">
          <FolderKanban className="h-8 w-8 text-muted-foreground" />
          <div>
            <h1 className="text-3xl font-bold tracking-tight">{app?.name}</h1>
            {app?.description && (
              <p className="text-muted-foreground">{app.description}</p>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 mt-2">
          <Badge variant="outline">{app?.visibility}</Badge>
          {app?.run_count !== undefined && (
            <span className="text-sm text-muted-foreground">
              {app.run_count} runs
            </span>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b">
        <nav className="flex gap-4" aria-label="Tabs">
          {tabs.map((tab) => (
            <Link
              key={tab.href}
              to={tab.href}
              className={cn(
                "flex items-center gap-2 px-1 py-3 text-sm font-medium border-b-2 -mb-px transition-colors",
                tab.active
                  ? "border-primary text-primary"
                  : "border-transparent text-muted-foreground hover:text-foreground hover:border-muted-foreground",
              )}
            >
              <tab.icon className="h-4 w-4" />
              {tab.label}
            </Link>
          ))}
        </nav>
      </div>

      {/* Tab content */}
      <Outlet />
    </div>
  );
}
