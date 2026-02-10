import { useLocation } from "@tanstack/react-router";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Activity, FolderOpen, ChevronLeft } from "lucide-react";
import { useState, useEffect, useRef } from "react";
import { useConfigStore } from "@/store/config-store";
import { apiClient } from "@/api/client";

interface LocalShellProps {
  children: React.ReactNode;
}

export function LocalShell({ children }: LocalShellProps) {
  const location = useLocation();
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const { config } = useConfigStore();
  const lastActiveRunRef = useRef<string | null>(null);

  // Poll for active run changes and auto-navigate
  useEffect(() => {
    const checkActiveRun = async () => {
      try {
        const response = await apiClient.get<{ run_id: string }>("/active-run");
        const activeRunId = response.data.run_id;

        if (activeRunId && activeRunId !== lastActiveRunRef.current) {
          lastActiveRunRef.current = activeRunId;

          // Only navigate if we're not already on this run's page
          const currentRunId = location.pathname.match(/\/runs\/([^/]+)/)?.[1];
          if (currentRunId !== activeRunId) {
            window.location.href = `/runs/${activeRunId}`;
          }
        }
      } catch {
        // Endpoint might not exist yet, ignore
      }
    };

    // Check immediately on mount
    checkActiveRun();

    // Poll every 2 seconds
    const interval = setInterval(checkActiveRun, 2000);
    return () => clearInterval(interval);
  }, [location.pathname]);

  const goToProjects = () => {
    // Navigate to projects page - using window.location for simplicity
    // since /projects is a local-mode only route
    window.location.href = "/projects";
  };

  return (
    <div className="min-h-screen flex">
      {/* Sidebar */}
      <aside
        className={cn(
          "bg-muted/40 border-r flex flex-col transition-all duration-300",
          sidebarOpen ? "w-64" : "w-16",
        )}
      >
        {/* Logo */}
        <div className="h-14 flex items-center px-4 border-b">
          <button onClick={goToProjects} className="flex items-center gap-2">
            <Activity className="h-6 w-6" />
            {sidebarOpen && <span className="font-semibold">p95</span>}
          </button>
        </div>

        {/* Navigation */}
        <ScrollArea className="flex-1 py-4">
          <nav className="space-y-1 px-2">
            <Button
              variant={
                location.pathname.startsWith("/projects")
                  ? "secondary"
                  : "ghost"
              }
              className={cn(
                "w-full justify-start gap-2",
                !sidebarOpen && "justify-center px-2",
              )}
              onClick={goToProjects}
            >
              <FolderOpen className="h-4 w-4" />
              {sidebarOpen && "Projects"}
            </Button>
          </nav>
        </ScrollArea>

        {/* Logdir info */}
        {sidebarOpen && config?.logdir && (
          <div className="px-4 py-3 border-t text-xs text-muted-foreground">
            <div className="font-medium mb-1">Log Directory</div>
            <div className="truncate" title={config.logdir}>
              {config.logdir}
            </div>
          </div>
        )}

        {/* Collapse button */}
        <div className="p-2 border-t">
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-center"
            onClick={() => setSidebarOpen(!sidebarOpen)}
          >
            <ChevronLeft
              className={cn(
                "h-4 w-4 transition-transform",
                !sidebarOpen && "rotate-180",
              )}
            />
          </Button>
        </div>
      </aside>

      {/* Main content */}
      <div className="flex-1 flex flex-col">
        {/* Header */}
        <header className="h-14 border-b flex items-center justify-between px-6">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full bg-primary/10 text-primary text-xs font-medium">
              Local Mode
            </span>
          </div>
          <div className="text-xs text-muted-foreground">
            v{config?.version || "0.1.0"}
          </div>
        </header>

        {/* Page content */}
        <main className="flex-1 p-6 overflow-auto">{children}</main>
      </div>
    </div>
  );
}
