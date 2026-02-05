import { createFileRoute, Outlet, redirect } from "@tanstack/react-router";
import { useAuthStore } from "@/store/auth-store";
import { useConfigStore } from "@/store/config-store";
import { AppShell } from "@/components/layout/app-shell";
import { LocalShell } from "@/components/layout/local-shell";

export const Route = createFileRoute("/_authenticated")({
  beforeLoad: () => {
    const { config } = useConfigStore.getState();

    // In local mode, skip auth check entirely
    if (config?.mode === "local") {
      return;
    }

    // In hosted mode, require authentication
    const { isAuthenticated } = useAuthStore.getState();
    if (!isAuthenticated) {
      throw redirect({ to: "/login" });
    }
  },
  component: AuthenticatedLayout,
});

function AuthenticatedLayout() {
  const { config } = useConfigStore();

  // Use local shell in local mode
  if (config?.mode === "local") {
    return (
      <LocalShell>
        <Outlet />
      </LocalShell>
    );
  }

  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}
