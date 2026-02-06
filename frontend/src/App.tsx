import { Routes, Route, Navigate } from 'react-router-dom';
import Layout from './components/Layout';
import Login from './pages/Login';
import Register from './pages/Register';
import Dashboard from './pages/Dashboard';
import Search from './pages/Search';
import Library from './pages/Library';
import PaperDetail from './pages/PaperDetail';
import Reader from './pages/Reader';

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />

      <Route element={<Layout />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/search" element={<Search />} />
        <Route path="/library" element={<Library />} />
        <Route path="/paper/:id" element={<PaperDetail />} />
      </Route>

      <Route path="/read/:id" element={<Reader />} />

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
