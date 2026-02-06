import { useState, useEffect } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuthStore } from './stores/authStore';
import Layout from './components/Layout';
import Login from './pages/Login';
import Register from './pages/Register';
import Dashboard from './pages/Dashboard';
import Search from './pages/Search';
import Library from './pages/Library';
import PaperDetail from './pages/PaperDetail';
import Reader from './pages/Reader';

function LoadingScreen() {
  return (
    <div className="min-h-screen bg-surface-50 dark:bg-surface-950 flex items-center justify-center">
      <div className="flex flex-col items-center gap-3">
        <div className="h-8 w-8 border-3 border-primary-600 border-t-transparent rounded-full animate-spin" />
        <p className="text-sm text-surface-400">Loading...</p>
      </div>
    </div>
  );
}

/**
 * Hook that waits for Zustand persist to finish rehydrating from localStorage.
 * Uses the persist API (persist.hasHydrated / persist.onFinishHydration) instead
 * of storing _hasHydrated inside the store — avoids the circular-reference bug
 * where onRehydrateStorage fires before the store variable is assigned.
 */
function useHydration() {
  const [hydrated, setHydrated] = useState(useAuthStore.persist.hasHydrated());

  useEffect(() => {
    if (hydrated) return;
    // Subscribe to finish event in case hydration hasn't completed yet
    const unsub = useAuthStore.persist.onFinishHydration(() => setHydrated(true));
    return unsub;
  }, [hydrated]);

  return hydrated;
}

export default function App() {
  const hydrated = useHydration();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  // Wait for auth state to load from localStorage before rendering routes
  if (!hydrated) {
    return <LoadingScreen />;
  }

  return (
    <Routes>
      {/* Auth pages — redirect to home if already logged in */}
      <Route
        path="/login"
        element={isAuthenticated ? <Navigate to="/" replace /> : <Login />}
      />
      <Route
        path="/register"
        element={isAuthenticated ? <Navigate to="/" replace /> : <Register />}
      />

      {/* Main app shell */}
      <Route element={<Layout />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/search" element={<Search />} />
        <Route
          path="/library"
          element={isAuthenticated ? <Library /> : <Navigate to="/login" replace />}
        />
        <Route path="/paper/:id" element={<PaperDetail />} />
      </Route>

      {/* Reader — works for everyone, auth features are gated inside */}
      <Route path="/read/:id" element={<Reader />} />

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
