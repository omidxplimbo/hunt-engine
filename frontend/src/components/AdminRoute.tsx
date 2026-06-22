import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export const AdminRoute = () => {
  const { isAuthenticated, role } = useAuth();

  if (!isAuthenticated) return <Navigate to="/login" replace />;

  // if role is not hydrated yet, treat as non-admin
  if (role !== 'admin') return <Navigate to="/dashboard" replace />;

  return <Outlet />;
};


