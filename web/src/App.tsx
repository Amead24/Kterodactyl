import { BrowserRouter, Routes, Route } from 'react-router';
import { ProtectedRoute, AdminRoute } from '@/components/auth/protected-route';
import { AppShell } from '@/components/layout/app-shell';
import LoginPage from '@/pages/login';
import RegisterPage from '@/pages/register';
import DashboardPage from '@/pages/dashboard';
import GamesPage from '@/pages/games';

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
            <Route path="servers" element={<Placeholder title="My Servers" />} />
            <Route path="servers/create" element={<Placeholder title="Create Server" />} />
            <Route path="servers/:name" element={<Placeholder title="Server Detail" />} />

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
