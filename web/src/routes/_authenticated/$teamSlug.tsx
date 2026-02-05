import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/_authenticated/$teamSlug")({
  component: TeamLayout,
});

function TeamLayout() {
  return <Outlet />;
}
