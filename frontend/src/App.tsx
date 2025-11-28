import { BrowserRouter, Routes, Route } from 'react-router-dom'; // 👈 Navigate حذف شد
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MainLayout } from './layouts/MainLayout';
import Target from './pages/TargetsPages'; // 👈

const Dashboard = () => <div className="text-2xl font-bold">Dashboard Coming Soon...</div>;
const Targets = () => <div className="text-2xl font-bold">Targets List (Next Step)</div>;

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
            {/* 👇 استفاده از کامپوننت واقعی Targets */}
            <Route path="targets" element={<Targets />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;