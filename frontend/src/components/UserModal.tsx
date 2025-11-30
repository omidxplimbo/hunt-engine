import { useState, useEffect } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createUser, updateUser } from '../api/users';
import { X, Loader2 } from 'lucide-react';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  user?: any; // اگر ویرایش باشد، یوزر پاس داده می‌شود
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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-xl shadow-2xl w-full max-w-md">
        <div className="flex justify-between items-center p-5 border-b border-gray-800">
          <h2 className="text-lg font-bold text-white">{user ? 'Edit User' : 'Add User'}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-white"><X size={20} /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(formData); }} className="p-6 space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">Username</label>
            <input type="text" required className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white focus:ring-blue-600 outline-none"
              value={formData.username} onChange={e => setFormData({...formData, username: e.target.value})} />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">Password {user && '(Leave blank to keep)'}</label>
            <input type="password" className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white focus:ring-blue-600 outline-none"
              value={formData.password} onChange={e => setFormData({...formData, password: e.target.value})} />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">Role</label>
            <select className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white focus:ring-blue-600 outline-none"
              value={formData.role} onChange={e => setFormData({...formData, role: e.target.value})}>
              <option value="admin">Admin</option>
              <option value="viewer">Viewer</option>
            </select>
          </div>
          <button type="submit" disabled={mutation.isPending} className="w-full bg-blue-600 hover:bg-blue-700 text-white py-2 rounded-lg font-medium flex justify-center">
            {mutation.isPending ? <Loader2 className="animate-spin" /> : (user ? 'Update' : 'Create')}
          </button>
        </form>
      </div>
    </div>
  );
};