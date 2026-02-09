import { createRootRoute, Outlet, Link } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/router-devtools";
import { Activity } from "lucide-react";

export const Route = createRootRoute({
  component: () => (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container flex h-14 items-center px-6">
          <Link to="/projects" className="flex items-center gap-2 font-semibold">
            <Activity className="h-5 w-5" />
            <span>Sixtyseven</span>
          </Link>
        </div>
      </header>

      {/* Main content */}
      <main className="container px-6 py-6">
        <Outlet />
      </main>

      {import.meta.env.DEV && <TanStackRouterDevtools />}
    </div>
  ),
});
