import { useState, useEffect } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createUser, updateUser } from '../api/users';
import { X, Loader2, UserPlus, Key, ShieldAlert } from 'lucide-react';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  user?: any;
}

export const UserModal = ({ isOpen, onClose, user }: Props) => {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState({ username: '', password: '', role: 'admin' });

  useEffect(() => {
    if (user) {
      setFormData({ username: user.username, password: '', role: user.role });
    } else {
      setFormData({ username: '', password: '', role: 'admin' });
    }
  }, [user, isOpen]);

  const mutation = useMutation({
    mutationFn: (data: any) => user ? updateUser(user.id, data) : createUser(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      onClose();
    },
  });

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
      <div className="hack-box w-full max-w-md relative animate-in fade-in zoom-in duration-200">
        {/* Header */}
        <div className="flex justify-between items-center p-4 border-b border-hack-border bg-black/20">
          <div className="flex items-center gap-2 text-hack-primary">
            <UserPlus size={18} />
            <h2 className="font-mono font-bold tracking-widest text-lg">{user ? 'MODIFY_USER' : 'ADD_USER'}</h2>
          </div>
          <button onClick={onClose} className="text-hack-dim hover:text-hack-danger transition-colors">
            <X size={18} />
          </button>
        </div>

        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(formData); }} className="p-6 space-y-5">
          
          <div className="space-y-1 group">
            <label className="text-[10px] uppercase text-hack-dim tracking-widest group-focus-within:text-hack-primary transition-colors">Username ID</label>
            <div className="relative">
                <input 
                    type="text" 
                    required 
                    className="hack-input w-full pl-8"
                    placeholder="agent_007"
                    value={formData.username} 
                    onChange={e => setFormData({...formData, username: e.target.value})} 
                />
                <div className="absolute left-2.5 top-2.5 text-hack-dim"><ShieldAlert size={12}/></div>
            </div>
          </div>

          <div className="space-y-1 group">
            <label className="text-[10px] uppercase text-hack-dim tracking-widest group-focus-within:text-hack-primary transition-colors">
                Access Key {user && <span className="text-hack-warning/70">(LEAVE BLANK TO KEEP)</span>}
            </label>
            <div className="relative">
                <input 
                    type="password" 
                    className="hack-input w-full pl-8"
                    placeholder="••••••••"
                    value={formData.password} 
                    onChange={e => setFormData({...formData, password: e.target.value})} 
                />
                <div className="absolute left-2.5 top-2.5 text-hack-dim"><Key size={12}/></div>
            </div>
          </div>

          <div className="space-y-1">
            <label className="text-[10px] uppercase text-hack-dim tracking-widest">Clearance Level</label>
            <select 
                className="hack-input w-full appearance-none cursor-pointer"
                value={formData.role} 
                onChange={e => setFormData({...formData, role: e.target.value})}
            >
              <option value="admin">LEVEL 5 (ADMIN)</option>
              <option value="viewer">LEVEL 1 (VIEWER)</option>
            </select>
          </div>

          <div className="pt-4 flex gap-3">
            <button type="button" onClick={onClose} className="flex-1 hack-btn-ghost border border-hack-border">
                CANCEL
            </button>
            <button type="submit" disabled={mutation.isPending} className="flex-1 hack-btn">
                {mutation.isPending ? <Loader2 className="animate-spin" size={16} /> : (user ? 'UPDATE_RECORD' : 'GRANT_ACCESS')}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};