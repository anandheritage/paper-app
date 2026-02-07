import { useState, useEffect } from 'react';
import { Routes, Route, Navigate, useParams } from 'react-router-dom';
import { useAuthStore } from './stores/authStore';
import Layout from './components/Layout';
import Login from './pages/Login';
import Landing from './pages/Landing';
import Dashboard from './pages/Dashboard';
import Search from './pages/Search';
import Library from './pages/Library';
import PaperDetail from './pages/PaperDetail';
import Discover from './pages/Discover';

// Redirect legacy /read/:id URLs to /paper/:id
function ReadRedirect() {
  const { id } = useParams<{ id: string }>();
  return <Navigate to={`/paper/${id}`} replace />;
}

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
 */
function useHydration() {
  const [hydrated, setHydrated] = useState(useAuthStore.persist.hasHydrated());

  useEffect(() => {
    if (hydrated) return;
    const unsub = useAuthStore.persist.onFinishHydration(() => setHydrated(true));
    return unsub;
  }, [hydrated]);

  return hydrated;
}

export default function App() {
  const hydrated = useHydration();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  if (!hydrated) {
    return <LoadingScreen />;
  }

  return (
    <Routes>
      {/* Landing page for guests, redirect to dashboard for logged-in users */}
      {!isAuthenticated && (
        <Route path="/" element={<Landing />} />
      )}

      {/* Auth pages — redirect to home if already logged in */}
      <Route
        path="/login"
        element={isAuthenticated ? <Navigate to="/" replace /> : <Login />}
      />
      {/* /register redirects to the unified login page */}
      <Route path="/register" element={<Navigate to="/login" replace />} />

      {/* Main app shell (authenticated) */}
      <Route element={<Layout />}>
        {isAuthenticated && (
          <Route path="/" element={<Dashboard />} />
        )}
        <Route
          path="/search"
          element={isAuthenticated ? <Search /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/library"
          element={isAuthenticated ? <Library /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/discover"
          element={isAuthenticated ? <Discover /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/paper/:id"
          element={isAuthenticated ? <PaperDetail /> : <Navigate to="/login" replace />}
        />
      </Route>

      {/* Legacy /read/:id URLs redirect to paper detail */}
      <Route path="/read/:id" element={<ReadRedirect />} />

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
