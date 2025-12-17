import { createContext, useContext, useEffect, useState } from 'react';
import type { ReactNode } from 'react'; // 👈 اصلاح: ایمپورت به صورت تایپ
import { apiClient } from '../api/client';

interface AuthContextType {
  token: string | null;
  username: string | null;
  role: string | null;
  login: (token: string, username: string, role: string) => void;
  logout: () => void;
  isAuthenticated: boolean;
}

const AuthContext = createContext<AuthContextType | null>(null);

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [token, setToken] = useState<string | null>(localStorage.getItem('token'));
  const [username, setUsername] = useState<string | null>(localStorage.getItem('username'));
  const [role, setRole] = useState<string | null>(localStorage.getItem('role'));

  const login = (newToken: string, newUsername: string, newRole: string) => {
    localStorage.setItem('token', newToken);
    localStorage.setItem('username', newUsername);
    localStorage.setItem('role', newRole);
    setToken(newToken);
    setUsername(newUsername);
    setRole(newRole);
  };

  const logout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('username');
    localStorage.removeItem('role');
    setToken(null);
    setUsername(null);
    setRole(null);
  };

  // Hydrate role (and username) for older sessions that didn't store role
  useEffect(() => {
    if (!token) return;
    if (role) return;

    let cancelled = false;
    (async () => {
      try {
        const res = await apiClient.get<{ status: string; data: { username: string; role: string } }>('/me');
        if (cancelled) return;
        const fetchedRole = res.data?.data?.role;
        const fetchedUsername = res.data?.data?.username;
        if (fetchedRole) {
          localStorage.setItem('role', fetchedRole);
          setRole(fetchedRole);
        }
        if (fetchedUsername) {
          localStorage.setItem('username', fetchedUsername);
          setUsername(fetchedUsername);
        }
      } catch {
        // ignore - if token is invalid, ProtectedRoute will handle via logout flow elsewhere
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [token, role]);

  return (
    <AuthContext.Provider value={{ token, username, role, login, logout, isAuthenticated: !!token }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used within an AuthProvider');
  return context;
};