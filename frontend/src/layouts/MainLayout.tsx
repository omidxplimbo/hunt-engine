import { Link, Outlet, useLocation } from 'react-router-dom'; // 👈 React حذف شد
import { LayoutDashboard, Target, Settings, Shield } from 'lucide-react';
import clsx from 'clsx';

const SidebarItem = ({ to, icon: Icon, label }: { to: string; icon: any; label: string }) => {
  const location = useLocation();
  const isActive = location.pathname.startsWith(to);

  return (
    <Link
      to={to}
      className={clsx(
        'flex items-center gap-3 px-4 py-3 rounded-lg transition-colors mb-1',
        isActive
          ? 'bg-blue-600 text-white shadow-lg shadow-blue-900/20'
          : 'text-gray-400 hover:bg-gray-800 hover:text-gray-100'
      )}
    >
      <Icon size={20} />
      <span className="font-medium">{label}</span>
    </Link>
  );
};

export const MainLayout = () => {
  return (
    <div className="flex h-screen bg-gray-950 text-gray-100 font-sans overflow-hidden">
      {/* Sidebar */}
      <aside className="w-64 bg-gray-900 border-r border-gray-800 flex flex-col">
        <div className="p-6 border-b border-gray-800 flex items-center gap-3">
          <div className="w-8 h-8 bg-blue-600 rounded-lg flex items-center justify-center">
            <Shield size={20} className="text-white" />
          </div>
          <h1 className="text-xl font-bold tracking-tight text-white">Hunter<span className="text-blue-500">Pro</span></h1>
        </div>

        <nav className="flex-1 p-4 overflow-y-auto">
          <div className="mb-6">
            <p className="px-4 text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">Main</p>
            <SidebarItem to="/" icon={LayoutDashboard} label="Dashboard" />
            <SidebarItem to="/targets" icon={Target} label="Targets" />
          </div>
          
          <div>
            <p className="px-4 text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">System</p>
            <SidebarItem to="/settings" icon={Settings} label="Settings" />
          </div>
        </nav>

        <div className="p-4 border-t border-gray-800">
          <div className="flex items-center gap-3 px-4 py-2">
            <div className="w-8 h-8 rounded-full bg-gradient-to-tr from-blue-500 to-purple-500"></div>
            <div>
              <p className="text-sm font-medium text-white">Omid</p>
              <p className="text-xs text-gray-500">Security Researcher</p>
            </div>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 overflow-auto bg-gray-950">
        <div className="p-8 max-w-7xl mx-auto">
          <Outlet />
        </div>
      </main>
    </div>
  );
};