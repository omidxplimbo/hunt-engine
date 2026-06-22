import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Toaster } from 'react-hot-toast';
import { MainLayout } from './layouts/MainLayout';
import { AuthProvider } from './context/AuthContext'; // 👈
import { ProtectedRoute } from './components/ProtectedRoute'; // 👈
import { AdminRoute } from './components/AdminRoute';
import Login from './pages/Login';
import Landing from './pages/Landing'; // 👈
import TargetsPage from './pages/TargetsPages';
import TargetAssets from './pages/TargetAssets';
import Settings from './pages/Settings';
import NucleiTemplates from './pages/NucleiTemplates';
import Dashboard from './pages/Dashboard'; // 👈 ایمپورت از فایل جدید
import Account from './pages/Account';


const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      {/* 👇 کل برنامه رو توی AuthProvider می‌پیچیم */}
      <AuthProvider>
        <BrowserRouter>
          <Toaster 
            position="bottom-right"
            toastOptions={{
              className: 'font-mono text-sm',
              style: {
                background: '#050505',
                color: '#e0e0e0',
                border: '1px solid #333',
              },
              success: {
                iconTheme: {
                  primary: '#00ff41',
                  secondary: '#050505',
                },
                style: {
                  border: '1px solid #00ff41',
                }
              },
              error: {
                iconTheme: {
                  primary: '#ff003c',
                  secondary: '#050505',
                },
                style: {
                  border: '1px solid #ff003c',
                }
              },
            }}
          />
          <Routes>
            <Route path="/" element={<Landing />} />
            <Route path="/login" element={<Login />} />

            <Route element={<ProtectedRoute />}>
              <Route element={<MainLayout />}>
                <Route path="/dashboard" element={<Dashboard />} />
                <Route path="/account" element={<Account />} />
                <Route path="/targets" element={<TargetsPage />} />
                <Route path="/targets/:id" element={<TargetAssets />} />
                <Route element={<AdminRoute />}>
                  <Route path="/nuclei-templates" element={<NucleiTemplates />} />
                  <Route path="/settings" element={<Settings />} />
                </Route>
              </Route>
            </Route>
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  );
}

export default App;