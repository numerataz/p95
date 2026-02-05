import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { getApp, updateApp, deleteApp } from "@/api/apps";
import { getErrorMessage } from "@/api/client";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Loader2, Trash2 } from "lucide-react";

export const Route = createFileRoute(
  "/_authenticated/$teamSlug/$appSlug/settings",
)({
  component: AppSettingsPage,
});

function AppSettingsPage() {
  const { teamSlug, appSlug } = Route.useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const { data: app, isLoading } = useQuery({
    queryKey: ["app", teamSlug, appSlug],
    queryFn: () => getApp(teamSlug, appSlug),
  });

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  // Initialize form when app data loads
  useState(() => {
    if (app) {
      setName(app.name);
      setDescription(app.description || "");
    }
  });

  const updateMutation = useMutation({
    mutationFn: () =>
      updateApp(teamSlug, appSlug, {
        name,
        description: description || undefined,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["app", teamSlug, appSlug] });
      queryClient.invalidateQueries({ queryKey: ["apps", teamSlug] });
      setError("");
    },
    onError: (err) => {
      setError(getErrorMessage(err));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteApp(teamSlug, appSlug),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["apps", teamSlug] });
      navigate({ to: "/$teamSlug", params: { teamSlug } });
    },
    onError: (err) => {
      setError(getErrorMessage(err));
    },
  });

  if (isLoading) {
    return <Skeleton className="h-64 w-full" />;
  }

  // Set initial values once loaded
  if (app && !name) {
    setName(app.name);
    setDescription(app.description || "");
  }

  return (
    <div className="space-y-6 max-w-2xl">
      {/* General Settings */}
      <Card>
        <CardHeader>
          <CardTitle>General</CardTitle>
          <CardDescription>Update your app's basic information</CardDescription>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              updateMutation.mutate();
            }}
            className="space-y-4"
          >
            {error && (
              <div className="bg-destructive/10 text-destructive text-sm p-3 rounded-md">
                {error}
              </div>
            )}
            <div className="space-y-2">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="description">Description</Label>
              <Input
                id="description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Optional description"
              />
            </div>
            <div className="space-y-2">
              <Label>Slug</Label>
              <Input value={app?.slug || ""} disabled />
              <p className="text-xs text-muted-foreground">
                Slug cannot be changed. SDK project: {teamSlug}/{app?.slug}
              </p>
            </div>
            <Button type="submit" disabled={updateMutation.isPending}>
              {updateMutation.isPending && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              Save Changes
            </Button>
          </form>
        </CardContent>
      </Card>

      {/* Danger Zone */}
      <Card className="border-destructive/50">
        <CardHeader>
          <CardTitle className="text-destructive">Danger Zone</CardTitle>
          <CardDescription>Irreversible actions</CardDescription>
        </CardHeader>
        <CardContent>
          <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
            <DialogTrigger asChild>
              <Button variant="destructive">
                <Trash2 className="mr-2 h-4 w-4" />
                Delete App
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Delete App</DialogTitle>
                <DialogDescription>
                  Are you sure you want to delete "{app?.name}"? This will
                  permanently delete all runs and metrics associated with this
                  app. This action cannot be undone.
                </DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <Button
                  variant="outline"
                  onClick={() => setDeleteDialogOpen(false)}
                >
                  Cancel
                </Button>
                <Button
                  variant="destructive"
                  onClick={() => deleteMutation.mutate()}
                  disabled={deleteMutation.isPending}
                >
                  {deleteMutation.isPending && (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  )}
                  Delete
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </CardContent>
      </Card>
    </div>
  );
}
