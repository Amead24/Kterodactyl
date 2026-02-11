import { useState } from 'react';
import { Copy, Check, Send } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useCreateInvite } from '@/hooks/use-admin';
import type { InviteResponse } from '@/types/api';

export default function InvitesPage() {
  const createInvite = useCreateInvite();
  const [email, setEmail] = useState('');
  const [lastInvite, setLastInvite] = useState<InviteResponse | null>(null);
  const [copied, setCopied] = useState(false);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!email.trim()) return;

    createInvite.mutate(
      { email: email.trim() },
      {
        onSuccess: (data) => {
          setLastInvite(data);
          setEmail('');
          setCopied(false);
        },
      },
    );
  }

  function handleCopy(text: string) {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  const registrationLink = lastInvite
    ? `${window.location.origin}/register?token=${lastInvite.token}`
    : '';

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Create Invite</h1>
        <p className="text-muted-foreground">
          Create an invite for a new user. Share the token or registration link
          with them.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>New Invitation</CardTitle>
          <CardDescription>
            Enter the email address of the person you want to invite.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex items-end gap-3">
            <div className="flex-1 space-y-2">
              <Label htmlFor="email">Email address</Label>
              <Input
                id="email"
                type="email"
                placeholder="user@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <Button type="submit" disabled={createInvite.isPending}>
              <Send className="mr-2 size-4" />
              {createInvite.isPending ? 'Creating...' : 'Create Invite'}
            </Button>
          </form>
        </CardContent>
      </Card>

      {lastInvite && (
        <Card>
          <CardHeader>
            <CardTitle>Invite Created</CardTitle>
            <CardDescription>
              Share this registration link or token with{' '}
              <span className="font-medium text-foreground">
                {lastInvite.email}
              </span>
              . Expires{' '}
              {new Date(lastInvite.expiresAt).toLocaleDateString(undefined, {
                year: 'numeric',
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit',
              })}
              .
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>Registration Link</Label>
              <div className="flex gap-2">
                <Input readOnly value={registrationLink} className="font-mono text-sm" />
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleCopy(registrationLink)}
                  className="shrink-0"
                >
                  {copied ? (
                    <Check className="mr-1 size-4" />
                  ) : (
                    <Copy className="mr-1 size-4" />
                  )}
                  {copied ? 'Copied' : 'Copy'}
                </Button>
              </div>
            </div>
            <div className="space-y-2">
              <Label>Token</Label>
              <div className="flex gap-2">
                <Input
                  readOnly
                  value={lastInvite.token}
                  className="font-mono text-sm"
                />
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleCopy(lastInvite.token)}
                  className="shrink-0"
                >
                  <Copy className="mr-1 size-4" />
                  Copy
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
