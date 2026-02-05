import { createFileRoute, Outlet } from "@tanstack/react-router";
import { LocalShell } from "@/components/layout/local-shell";

export const Route = createFileRoute("/_local")({
  component: LocalLayout,
});

function LocalLayout() {
  return (
    <LocalShell>
      <Outlet />
    </LocalShell>
  );
}
