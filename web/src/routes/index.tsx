import { createFileRoute, redirect } from "@tanstack/react-router";
import { useConfigStore } from "@/store/config-store";

export const Route = createFileRoute("/")({
  beforeLoad: () => {
    const { config } = useConfigStore.getState();

    // In local mode, redirect to projects page using window.location
    // since /projects is not a typed route
    if (config?.mode === "local") {
      window.location.href = "/projects";
      return;
    }

    // In hosted mode, redirect to dashboard
    throw redirect({ to: "/dashboard" });
  },
});
