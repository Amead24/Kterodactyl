import { Outlet } from 'react-router';
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar';
import { AppSidebar } from '@/components/layout/sidebar-nav';
import { Header } from '@/components/layout/header';

/**
 * Main layout wrapper: sidebar on left, header on top, content area below.
 * Renders child routes via Outlet. Responsive -- sidebar collapses on mobile.
 */
export function AppShell() {
  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <Header />
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </SidebarInset>
    </SidebarProvider>
  );
}
