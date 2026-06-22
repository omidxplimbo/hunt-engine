import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { loginUser, type LoginPayload } from '../api/auth';
import { useAuth } from '../context/AuthContext';
import { useNavigate } from 'react-router-dom';
import { Lock, User, Terminal, ChevronRight, ShieldAlert } from 'lucide-react';

const Login = () => {
  const [formData, setFormData] = useState<LoginPayload>({ username: '', password: '' });
  const { login } = useAuth();
  const navigate = useNavigate();
  const [errorMsg, setErrorMsg] = useState('');

  const mutation = useMutation({
    mutationFn: loginUser,
    onSuccess: (data) => {
      login(data.token, data.username, data.role);
      navigate('/dashboard');
    },
    onError: (err: any) => {
      const apiErr = err?.response?.data?.error;
      setErrorMsg(apiErr || '>> ACCESS DENIED: Invalid credentials provided.');
    }
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    mutation.mutate(formData);
  };

  return (
    <div className="min-h-screen bg-hack-bg flex items-center justify-center p-4 bg-grid-pattern bg-[size:40px_40px] relative overflow-hidden">
      {/* Decorative Background Elements */}
      <div className="absolute inset-0 bg-gradient-to-b from-transparent via-hack-bg/80 to-hack-bg pointer-events-none"></div>
      
      {/* 👇 این اعداد بزرگ در موبایل مخفی می‌شوند (hidden md:block) */}
      <div className="hidden md:block absolute top-10 left-10 text-hack-dim/10 font-display text-9xl select-none animate-pulse">01</div>
      <div className="hidden md:block absolute bottom-10 right-10 text-hack-dim/10 font-display text-9xl select-none animate-pulse delay-700">10</div>

      {/* Main Login Box */}
      <div className="hack-box w-full max-w-md p-1 relative group animate-in fade-in zoom-in duration-500">
        
        {/* Animated Corner Borders */}
        <div className="absolute -top-1 -left-1 w-6 h-6 border-t-2 border-l-2 border-hack-primary opacity-60 group-hover:opacity-100 group-hover:w-8 group-hover:h-8 transition-all duration-500"></div>
        <div className="absolute -top-1 -right-1 w-6 h-6 border-t-2 border-r-2 border-hack-primary opacity-60 group-hover:opacity-100 group-hover:w-8 group-hover:h-8 transition-all duration-500"></div>
        <div className="absolute -bottom-1 -left-1 w-6 h-6 border-b-2 border-l-2 border-hack-primary opacity-60 group-hover:opacity-100 group-hover:w-8 group-hover:h-8 transition-all duration-500"></div>
        <div className="absolute -bottom-1 -right-1 w-6 h-6 border-b-2 border-r-2 border-hack-primary opacity-60 group-hover:opacity-100 group-hover:w-8 group-hover:h-8 transition-all duration-500"></div>

        <div className="bg-hack-panel/95 p-6 md:p-8 backdrop-blur-xl border border-hack-border/50 relative overflow-hidden">
          
          {/* Scanline Effect inside box */}
          <div className="absolute inset-0 bg-[linear-gradient(rgba(18,16,16,0)_50%,rgba(0,0,0,0.1)_50%),linear-gradient(90deg,rgba(255,0,0,0.06),rgba(0,255,0,0.02),rgba(0,0,255,0.06))] bg-[length:100%_2px,3px_100%] pointer-events-none opacity-20"></div>

          <div className="text-center mb-8 md:mb-10 relative z-10">
            <div className="w-14 h-14 md:w-16 md:h-16 mx-auto bg-hack-primary/10 border border-hack-primary/30 rounded-full flex items-center justify-center mb-4 shadow-[0_0_15px_rgba(0,255,65,0.2)]">
                <Terminal size={28} className="text-hack-primary animate-pulse md:w-8 md:h-8" />
            </div>
            <h1 className="hack-title text-2xl md:text-3xl mb-1 tracking-[0.2em]">SECURE_LOGIN</h1>
            <div className="flex items-center justify-center gap-2 mt-2">
                <span className="w-2 h-2 bg-hack-danger rounded-full animate-ping"></span>
                <p className="text-hack-dim text-[9px] md:text-[10px] font-mono uppercase tracking-widest">System Locked // Auth Required</p>
            </div>
          </div>

          <form onSubmit={handleSubmit} className="space-y-6 relative z-10">
            {errorMsg && (
              <div className="bg-hack-danger/10 border-l-2 border-hack-danger text-hack-danger text-xs font-mono p-3 flex items-center gap-2 animate-pulse">
                <ShieldAlert size={14} />
                <span className="font-bold">{errorMsg}</span>
              </div>
            )}

            <div className="space-y-5">
              <div className="relative group/input">
                <label className="text-[9px] uppercase text-hack-dim tracking-widest absolute -top-2.5 left-2 bg-hack-panel px-1 group-focus-within/input:text-hack-primary transition-colors">Operator ID</label>
                <div className="relative">
                    <User className="absolute left-3 top-1/2 -translate-y-1/2 text-hack-dim group-focus-within/input:text-hack-primary transition-colors" size={16} />
                    <input 
                    type="text" 
                    className="hack-input w-full pl-10 py-3 bg-black/50 border-hack-dim/30 focus:border-hack-primary"
                    placeholder="USERNAME"
                    value={formData.username}
                    onChange={e => setFormData({...formData, username: e.target.value})}
                    autoComplete="off"
                    />
                </div>
              </div>

              <div className="relative group/input">
                <label className="text-[9px] uppercase text-hack-dim tracking-widest absolute -top-2.5 left-2 bg-hack-panel px-1 group-focus-within/input:text-hack-primary transition-colors">Access Key</label>
                <div className="relative">
                    <Lock className="absolute left-3 top-1/2 -translate-y-1/2 text-hack-dim group-focus-within/input:text-hack-primary transition-colors" size={16} />
                    <input 
                    type="password" 
                    className="hack-input w-full pl-10 py-3 bg-black/50 border-hack-dim/30 focus:border-hack-primary"
                    placeholder="••••••••"
                    value={formData.password}
                    onChange={e => setFormData({...formData, password: e.target.value})}
                    />
                </div>
              </div>
            </div>

            <button 
              type="submit" 
              disabled={mutation.isPending}
              className="hack-btn w-full py-4 mt-4 group/btn relative overflow-hidden"
            >
              <div className="absolute inset-0 w-full h-full bg-hack-primary/10 translate-x-[-100%] group-hover/btn:translate-x-[100%] transition-transform duration-700 ease-in-out"></div>
              {mutation.isPending ? (
                  <span className="animate-pulse"> VERIFYING IDENTITY...</span>
              ) : (
                  <span className="flex items-center justify-center gap-2 text-sm">
                      INITIALIZE SESSION <ChevronRight size={16} className="group-hover/btn:translate-x-1 transition-transform" />
                  </span>
              )}
            </button>
          </form>
          
          <div className="mt-6 text-center">
            <p className="text-[8px] md:text-[9px] text-hack-dim/50 font-mono">
                UNAUTHORIZED ACCESS IS PROHIBITED AND WILL BE PROSECUTED.
                <br />SESSION IP LOGGED.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Login;
