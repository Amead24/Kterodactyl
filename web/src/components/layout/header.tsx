import { LogOut } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { Separator } from '@/components/ui/separator';
import { useAuthStore } from '@/stores/auth-store';
import { useLogout } from '@/hooks/use-auth';

/** Application header with branding, user info, and logout. */
export function Header() {
  const user = useAuthStore((s) => s.user);
  const logout = useLogout();

  return (
    <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
      <SidebarTrigger className="-ml-1" />
      <Separator orientation="vertical" className="mr-2 !h-4" />
      <span className="text-lg font-semibold">Kterodactyl</span>

      <div className="ml-auto flex items-center gap-3">
        {user && (
          <span className="text-sm text-muted-foreground">
            {user.username}
            <span className="ml-1 text-xs opacity-70">({user.role})</span>
          </span>
        )}
        <Button variant="ghost" size="sm" onClick={logout}>
          <LogOut className="mr-1 size-4" />
          Logout
        </Button>
      </div>
    </header>
  );
}
