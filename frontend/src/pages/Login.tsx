import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
// 👇👇👇 اصلاح مهم: استفاده از type برای LoginPayload
import { loginUser, type LoginPayload } from '../api/auth';
import { useAuth } from '../context/AuthContext';
import { useNavigate } from 'react-router-dom';
import { Lock, User, Loader2 } from 'lucide-react';

const Login = () => {
  const [formData, setFormData] = useState<LoginPayload>({ username: '', password: '' });
  const { login } = useAuth();
  const navigate = useNavigate();
  const [errorMsg, setErrorMsg] = useState('');

  const mutation = useMutation({
    mutationFn: loginUser,
    onSuccess: (data) => {
      login(data.token, data.username);
      navigate('/'); // هدایت به داشبورد
    },
    onError: () => {
      setErrorMsg('Invalid username or password');
    }
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    mutation.mutate(formData);
  };

  return (
    <div className="min-h-screen bg-gray-950 flex items-center justify-center p-4">
      <div className="bg-gray-900 border border-gray-800 rounded-2xl shadow-2xl w-full max-w-md p-8">
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-white mb-2">Hunter<span className="text-blue-500">Pro</span></h1>
          <p className="text-gray-400">Login to access the command center</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-6">
          {errorMsg && (
            <div className="bg-red-900/20 border border-red-900/50 text-red-400 text-sm p-3 rounded-lg text-center">
              {errorMsg}
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-400 mb-2">Username</label>
            <div className="relative">
              <User className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" size={18} />
              <input 
                type="text" 
                className="w-full bg-gray-950 border border-gray-800 rounded-lg pl-10 pr-4 py-3 text-white focus:ring-2 focus:ring-blue-600 outline-none transition-all"
                placeholder="Enter username"
                value={formData.username}
                onChange={e => setFormData({...formData, username: e.target.value})}
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-400 mb-2">Password</label>
            <div className="relative">
              <Lock className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" size={18} />
              <input 
                type="password" 
                className="w-full bg-gray-950 border border-gray-800 rounded-lg pl-10 pr-4 py-3 text-white focus:ring-2 focus:ring-blue-600 outline-none transition-all"
                placeholder="••••••••"
                value={formData.password}
                onChange={e => setFormData({...formData, password: e.target.value})}
              />
            </div>
          </div>

          <button 
            type="submit" 
            disabled={mutation.isPending}
            className="w-full bg-blue-600 hover:bg-blue-700 text-white font-bold py-3 rounded-lg transition-colors flex items-center justify-center gap-2"
          >
            {mutation.isPending ? <Loader2 className="animate-spin" /> : 'Sign In'}
          </button>
        </form>
      </div>
    </div>
  );
};

export default Login;