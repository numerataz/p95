import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createApp } from "@/api/apps";
import { getErrorMessage } from "@/api/client";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Plus, Loader2 } from "lucide-react";

interface CreateAppDialogProps {
  teamSlug: string;
}

export function CreateAppDialog({ teamSlug }: CreateAppDialogProps) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");
  const queryClient = useQueryClient();

  const createMutation = useMutation({
    mutationFn: () =>
      createApp(teamSlug, {
        name,
        slug: slug || generateSlug(name),
        description: description || undefined,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["apps", teamSlug] });
      setOpen(false);
      resetForm();
    },
    onError: (err) => {
      setError(getErrorMessage(err));
    },
  });

  const resetForm = () => {
    setName("");
    setSlug("");
    setDescription("");
    setError("");
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (!name.trim()) {
      setError("Name is required");
      return;
    }
    createMutation.mutate();
  };

  const handleNameChange = (value: string) => {
    setName(value);
    // Auto-generate slug from name if slug hasn't been manually edited
    if (!slug || slug === generateSlug(name)) {
      setSlug(generateSlug(value));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(isOpen) => {
        setOpen(isOpen);
        if (!isOpen) resetForm();
      }}
    >
      <DialogTrigger asChild>
        <Button>
          <Plus className="mr-2 h-4 w-4" />
          Create App
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create App</DialogTitle>
          <DialogDescription>
            Create a new app to track your ML experiments.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit}>
          <div className="space-y-4 py-4">
            {error && (
              <div className="bg-destructive/10 text-destructive text-sm p-3 rounded-md">
                {error}
              </div>
            )}
            <div className="space-y-2">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                placeholder="My ML Model"
                value={name}
                onChange={(e) => handleNameChange(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="slug">Slug</Label>
              <Input
                id="slug"
                placeholder="my-ml-model"
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                URL-friendly identifier. Used in SDK: {teamSlug}/
                {slug || "my-app"}
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="description">Description (optional)</Label>
              <Input
                id="description"
                placeholder="Training runs for my model"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={createMutation.isPending}>
              {createMutation.isPending && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              Create
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function generateSlug(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}
