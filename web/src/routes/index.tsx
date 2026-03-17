import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/")({
  beforeLoad: () => {
    // Always redirect to projects page in local mode
    throw redirect({ to: "/projects" });
  },
});
