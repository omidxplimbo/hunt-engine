import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MainLayout } from './layouts/MainLayout';
// 👇 ایمپورت صفحه واقعی تارگت‌ها
import TargetsPage from './pages/TargetsPages';
import TargetAssets from './pages/TargetAssets';

// کامپوننت موقت داشبورد
const Dashboard = () => <div className="text-2xl font-bold text-white">Dashboard Coming Soon...</div>;

// ❌ خط مربوط به const Targets = ... حذف شد تا خطا ندهد

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
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<MainLayout />}>
            <Route index element={<Dashboard />} />
            {/* 👇 روت لیست تارگت‌ها (با کامپوننت درست) */}
            <Route path="targets" element={<TargetsPage />} />
            
            {/* 👇 روت جدید برای دیدن دارایی‌ها (که فراموش شده بود) */}
            <Route path="targets/:id" element={<TargetAssets />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;