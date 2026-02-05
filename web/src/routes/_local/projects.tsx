import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/_local/projects")({
  component: () => <Outlet />,
});
