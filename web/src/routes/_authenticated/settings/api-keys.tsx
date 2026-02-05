import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getApiKeys, createApiKey, deleteApiKey } from "@/api/auth";
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
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { useState } from "react";
import { formatRelativeTime } from "@/lib/utils";
import { Plus, Trash2, Copy, Check, Key } from "lucide-react";
import type { APIKey } from "@/api/types";

export const Route = createFileRoute("/_authenticated/settings/api-keys")({
  component: ApiKeysPage,
});

function ApiKeysPage() {
  const queryClient = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [keyToDelete, setKeyToDelete] = useState<APIKey | null>(null);

  const { data: keys, isLoading } = useQuery({
    queryKey: ["api-keys"],
    queryFn: getApiKeys,
  });

  const createMutation = useMutation({
    mutationFn: (name: string) =>
      createApiKey({ name, scopes: ["read", "write"] }),
    onSuccess: (data) => {
      setCreatedKey(data.key);
      setNewKeyName("");
      queryClient.invalidateQueries({ queryKey: ["api-keys"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteApiKey,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["api-keys"] });
      setKeyToDelete(null);
    },
  });

  const handleDeleteConfirm = () => {
    if (keyToDelete) {
      deleteMutation.mutate(keyToDelete.id);
    }
  };

  const handleCopy = async () => {
    if (createdKey) {
      await navigator.clipboard.writeText(createdKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    if (newKeyName.trim()) {
      createMutation.mutate(newKeyName.trim());
    }
  };

  return (
    <div className="space-y-6 max-w-4xl">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">API Keys</h1>
        <p className="text-muted-foreground">
          Manage API keys for SDK authentication
        </p>
      </div>

      {/* Created key banner */}
      {createdKey && (
        <Card className="border-green-500 bg-green-50 dark:bg-green-950">
          <CardContent className="pt-6">
            <div className="space-y-2">
              <p className="text-sm font-medium">
                Your new API key has been created. Copy it now - you won't be
                able to see it again.
              </p>
              <div className="flex gap-2">
                <Input
                  readOnly
                  value={createdKey}
                  className="font-mono bg-background"
                />
                <Button onClick={handleCopy} variant="outline">
                  {copied ? (
                    <Check className="h-4 w-4" />
                  ) : (
                    <Copy className="h-4 w-4" />
                  )}
                </Button>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setCreatedKey(null)}
              >
                Dismiss
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Create form */}
      {showCreate ? (
        <Card>
          <CardHeader>
            <CardTitle>Create API Key</CardTitle>
            <CardDescription>
              Create a new API key for use with the SDK
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleCreate} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="name">Name</Label>
                <Input
                  id="name"
                  placeholder="e.g., Training Server"
                  value={newKeyName}
                  onChange={(e) => setNewKeyName(e.target.value)}
                  required
                />
              </div>
              <div className="flex gap-2">
                <Button type="submit" disabled={createMutation.isPending}>
                  Create Key
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setShowCreate(false)}
                >
                  Cancel
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      ) : (
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Create API Key
        </Button>
      )}

      {/* Keys list */}
      <Card>
        <CardHeader>
          <CardTitle>Your API Keys</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-12 w-full" />
              <Skeleton className="h-12 w-full" />
            </div>
          ) : keys && keys.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Key</TableHead>
                  <TableHead>Scopes</TableHead>
                  <TableHead>Last Used</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {keys.map((key) => (
                  <TableRow key={key.id}>
                    <TableCell className="font-medium">{key.name}</TableCell>
                    <TableCell className="font-mono text-sm">
                      {key.key_prefix}...
                    </TableCell>
                    <TableCell>
                      <div className="flex gap-1">
                        {key.scopes.map((scope) => (
                          <Badge
                            key={scope}
                            variant="outline"
                            className="text-xs"
                          >
                            {scope}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {key.last_used_at
                        ? formatRelativeTime(key.last_used_at)
                        : "Never"}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {formatRelativeTime(key.created_at)}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setKeyToDelete(key)}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <div className="text-center py-8 text-muted-foreground">
              <Key className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No API keys yet</p>
              <p className="text-sm">Create one to start using the SDK</p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Delete confirmation dialog */}
      <AlertDialog
        open={!!keyToDelete}
        onOpenChange={(open) => !open && setKeyToDelete(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete API Key</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete "{keyToDelete?.name}"? This action
              cannot be undone and any applications using this key will lose
              access.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDeleteConfirm}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {deleteMutation.isPending ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
