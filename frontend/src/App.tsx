import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MainLayout } from './layouts/MainLayout';
import { AuthProvider } from './context/AuthContext'; // 👈
import { ProtectedRoute } from './components/ProtectedRoute'; // 👈
import Login from './pages/Login'; // 👈
import TargetsPage from './pages/TargetsPages';
import TargetAssets from './pages/TargetAssets';
import Settings from './pages/Settings';
import Dashboard from './pages/Dashboard'; // 👈 ایمپورت از فایل جدید


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
          <Routes>
            {/* روت عمومی لاگین */}
            <Route path="/login" element={<Login />} />

            {/* 👇 روت‌های محافظت شده (نیاز به لاگین دارن) */}
            <Route element={<ProtectedRoute />}>
              <Route path="/" element={<MainLayout />}>
                <Route index element={<Dashboard />} />
                <Route path="targets" element={<TargetsPage />} />
                <Route path="targets/:id" element={<TargetAssets />} />
                <Route path="settings" element={<Settings />} />
              </Route>
            </Route>
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  );
}

export default App;