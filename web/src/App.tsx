import { BrowserRouter, Routes, Route } from 'react-router';
import { ProtectedRoute, AdminRoute } from '@/components/auth/protected-route';
import { AppShell } from '@/components/layout/app-shell';
import LoginPage from '@/pages/login';
import RegisterPage from '@/pages/register';
import DashboardPage from '@/pages/dashboard';
import GamesPage from '@/pages/games';
import ServersPage from '@/pages/servers';
import CreateServerPage from '@/pages/create-server';
import ServerDetailPage from '@/pages/server-detail';

/** Placeholder for routes not yet implemented. */
function Placeholder({ title }: { title: string }) {
  return (
    <div className="flex items-center justify-center py-20">
      <p className="text-lg text-muted-foreground">{title} -- coming soon</p>
    </div>
  );
}

function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* Public routes */}
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />

        {/* Protected routes */}
        <Route element={<ProtectedRoute />}>
          <Route element={<AppShell />}>
            <Route index element={<DashboardPage />} />
            <Route path="games" element={<GamesPage />} />
            <Route path="servers" element={<ServersPage />} />
            <Route path="servers/create" element={<CreateServerPage />} />
            <Route path="servers/:name" element={<ServerDetailPage />} />

            {/* Admin routes */}
            <Route path="admin" element={<AdminRoute />}>
              <Route path="users" element={<Placeholder title="User Management" />} />
              <Route path="invites" element={<Placeholder title="Invite Management" />} />
            </Route>
          </Route>
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
