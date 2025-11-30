import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { Target, Settings, LayoutDashboard, LogOut } from 'lucide-react';
import clsx from 'clsx';
// 👇 ایمپورت‌های جدید
import { useAuth } from '../context/AuthContext';
import MustacheLogo from '../components/MustacheLogo';

export const MainLayout = () => {
  // 👇 گرفتن تابع لاگ‌اوت و نویگیت
  const { logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout(); // پاک کردن توکن‌ها از کانتکست و لوکال استوریج
    navigate('/login'); // هدایت به صفحه لاگین
  };

  return (
    <div className="flex h-screen bg-gray-950">
      {/* Sidebar */}
      <aside className="w-72 bg-gray-900 border-r border-gray-800 flex flex-col">
        {/* 👇 بخشدر هدر سایدبار: لوگو و اسم تیم */}
        <div className="p-6 border-b border-gray-800 flex flex-col items-center text-center">
          <div className="mb-3 p-2 bg-gray-950 rounded-lg border border-green-900/30 shadow-[0_0_15px_rgba(34,197,94,0.2)]">
             <MustacheLogo />
          </div>
          <h1 className="text-lg font-bold text-white leading-tight">
            Mustache Security
            <span className="block text-sm text-green-500 font-mono mt-1">Researcher Team</span>
          </h1>
        </div>

        {/* Navigation Links */}
        <nav className="flex-1 p-4 space-y-2 overflow-y-auto">
          <NavItem to="/" icon={LayoutDashboard}>Dashboard</NavItem>
          <NavItem to="/targets" icon={Target}>Targets</NavItem>
          <NavItem to="/settings" icon={Settings}>Settings</NavItem>
        </nav>

        {/* 👇 بخش فوتر سایدبار: دکمه لاگ‌اوت */}
        <div className="p-4 border-t border-gray-800">
          <button
            onClick={handleLogout}
            className="w-full flex items-center gap-3 px-4 py-3 text-gray-400 hover:text-red-400 hover:bg-red-950/30 rounded-lg transition-colors group font-medium"
          >
            <LogOut size={20} className="group-hover:text-red-500 transition-colors" />
            Logout
          </button>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 overflow-auto">
        <div className="p-8 max-w-[1600px] mx-auto">
          <Outlet />
        </div>
      </main>
    </div>
  );
};

// Helper component for nav links (بدون تغییر)
const NavItem = ({ to, icon: Icon, children }: any) => {
  return (
    <NavLink
      to={to}
      className={({ isActive }) => clsx(
        "flex items-center gap-3 px-4 py-3 rounded-lg transition-colors font-medium",
        isActive 
          ? "bg-blue-600 text-white shadow-lg shadow-blue-900/20" 
          : "text-gray-400 hover:text-white hover:bg-gray-800"
      )}
    >
      <Icon size={20} />
      {children}
    </NavLink>
  );
};