import { useState } from 'react';
import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { Target, Settings, LayoutDashboard, LogOut, Menu, X } from 'lucide-react';
import clsx from 'clsx';
import { useAuth } from '../context/AuthContext';
import MustacheLogo from '../components/MustacheLogo';

export const MainLayout = () => {
  const { logout } = useAuth();
  const navigate = useNavigate();

  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  // بستن منو وقتی روت عوض میشه (در موبایل)
  const handleNavClick = () => {
    setIsMobileMenuOpen(false);
  };

  return (
    <div className="flex h-screen bg-hack-bg bg-grid-pattern bg-[size:40px_40px] overflow-hidden">
      {/* Mobile Header */}
      <div className="md:hidden fixed top-0 left-0 right-0 h-16 bg-hack-panel/95 border-b border-hack-border flex items-center justify-between px-4 z-50 backdrop-blur-md">
        <div className="flex items-center gap-2">
           <div className="w-8 scale-75"><MustacheLogo /></div>
           <span className="font-display text-xl text-hack-primary tracking-widest">HUNTOS</span>
        </div>
        <button 
          onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
          className="text-hack-primary p-2 border border-hack-primary/30 bg-hack-primary/5 rounded hover:bg-hack-primary/10"
        >
          {isMobileMenuOpen ? <X size={20} /> : <Menu size={20} />}
        </button>
      </div>

      {/* Sidebar Overlay (Mobile Only) */}
      {isMobileMenuOpen && (
        <div 
          className="fixed inset-0 bg-black/80 z-40 md:hidden backdrop-blur-sm"
          onClick={() => setIsMobileMenuOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside className={clsx(
        "fixed md:static inset-y-0 left-0 z-50 w-72 bg-hack-panel/95 md:bg-hack-panel/90 border-r border-hack-primary/20 flex flex-col backdrop-blur-md transition-transform duration-300 ease-in-out md:translate-x-0 pt-16 md:pt-0",
        isMobileMenuOpen ? "translate-x-0" : "-translate-x-full"
      )}>
        {/* Decorative Line */}
        <div className="absolute top-0 left-0 w-full h-[2px] bg-gradient-to-r from-transparent via-hack-primary to-transparent opacity-50 hidden md:block" />

        <div className="p-6 border-b border-hack-border/50 flex flex-col items-center text-center hidden md:flex">
          <div className="mb-4 opacity-90 hover:opacity-100 transition-opacity duration-500">
             <MustacheLogo />
          </div>
          <h1 className="hack-title text-2xl !tracking-widest">
            HUNT<span className="text-white">OS</span> v1.0
          </h1>
          <div className="mt-2 flex items-center gap-2 text-[10px] text-hack-dim uppercase tracking-[0.2em]">
            <div className="w-2 h-2 rounded-full bg-hack-primary animate-pulse" />
            System Online
          </div>
        </div>

        <nav className="flex-1 p-4 space-y-2 overflow-y-auto">
          <div className="px-4 py-2 text-[10px] text-hack-dim uppercase tracking-widest border-b border-hack-border/30 mb-2">Modules</div>
          <div onClick={handleNavClick}>
            <NavItem to="/" icon={LayoutDashboard}>Dashboard</NavItem>
          </div>
          <div onClick={handleNavClick}>
            <NavItem to="/targets" icon={Target}>Targets</NavItem>
          </div>
          <div onClick={handleNavClick}>
            <NavItem to="/settings" icon={Settings}>System Config</NavItem>
          </div>
        </nav>

        <div className="p-4 border-t border-hack-border/50">
          <button
            onClick={handleLogout}
            className="w-full flex items-center gap-3 px-4 py-3 text-hack-danger/70 hover:text-hack-danger hover:bg-hack-danger/10 border border-transparent hover:border-hack-danger/30 transition-all group font-medium uppercase tracking-widest text-xs"
          >
            <LogOut size={16} />
            Disconnect
          </button>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 overflow-auto relative pt-16 md:pt-0 w-full">
        {/* Background Scanline Overlay */}
        <div className="fixed inset-0 pointer-events-none bg-[linear-gradient(rgba(18,16,16,0)_50%,rgba(0,0,0,0.1)_50%),linear-gradient(90deg,rgba(255,0,0,0.06),rgba(0,255,0,0.02),rgba(0,0,255,0.06))] bg-[length:100%_2px,3px_100%] z-0 opacity-20"></div>
        
        <div className="p-4 md:p-8 max-w-[1800px] mx-auto relative z-10">
          <Outlet />
        </div>
      </main>
    </div>
  );
};

const NavItem = ({ to, icon: Icon, children }: any) => {
  return (
    <NavLink
      to={to}
      className={({ isActive }) => clsx(
        "flex items-center gap-3 px-4 py-3 border transition-all font-mono text-sm tracking-wide",
        isActive 
          ? "bg-hack-primary/10 border-hack-primary/50 text-hack-primary shadow-[0_0_10px_rgba(0,255,65,0.1)]" 
          : "border-transparent text-hack-dim hover:text-hack-text hover:bg-white/5"
      )}
    >
      <Icon size={18} />
      {children}
    </NavLink>
  );
};