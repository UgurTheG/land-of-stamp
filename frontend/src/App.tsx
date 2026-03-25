import { lazy, Suspense } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router';
import { AuthProvider } from './context/AuthContext';
import { ThemeProvider } from './context/ThemeContext';
import { useTheme } from './hooks/useTheme';
import { Toaster } from 'sonner';
import Navbar from './components/layout/Navbar';
import ProtectedRoute from './components/layout/ProtectedRoute';

const LandingPage = lazy(() => import('./pages/LandingPage'));
const LoginPage = lazy(() => import('./pages/LoginPage'));
const UserDashboard = lazy(() => import('./pages/UserDashboard'));
const AdminDashboard = lazy(() => import('./pages/AdminDashboard'));
const ScanPage = lazy(() => import('./pages/ScanPage'));
const ClaimPage = lazy(() => import('./pages/ClaimPage'));

function AppShell() {
  const { theme } = useTheme();

  return (
    <>
      <Toaster theme={theme} position="top-right" richColors closeButton />
      {/* ── Animated background ── */}
      <div className="app-bg" aria-hidden="true">
        <div className="app-bg-orb-1" />
        <div className="app-bg-orb-2" />
        <div className="app-bg-grid" />
        <div className="app-bg-vignette" />
      </div>

      <div className="relative z-10 min-h-screen text-white">
        <Navbar />
        <Suspense fallback={<div className="flex items-center justify-center h-[60vh] text-zinc-400">Loading…</div>}>
          <Routes>
            <Route path="/" element={<LandingPage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route
              path="/dashboard"
              element={
                <ProtectedRoute requiredRole="user">
                  <UserDashboard />
                </ProtectedRoute>
              }
            />
            <Route
              path="/scan"
              element={
                <ProtectedRoute requiredRole="user">
                  <ScanPage />
                </ProtectedRoute>
              }
            />
            <Route
              path="/claim/:token"
              element={
                <ProtectedRoute requiredRole="user">
                  <ClaimPage />
                </ProtectedRoute>
              }
            />
            <Route
              path="/admin"
              element={
                <ProtectedRoute requiredRole="admin">
                  <AdminDashboard />
                </ProtectedRoute>
              }
            />
          </Routes>
        </Suspense>
      </div>
    </>
  );
}

function App() {
  return (
    <BrowserRouter>
      <ThemeProvider>
        <AuthProvider>
          <AppShell />
        </AuthProvider>
      </ThemeProvider>
    </BrowserRouter>
  );
}

export default App;
