import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getUsers, deleteUser } from '../api/users';
import { UserModal } from '../components/UserModal';
import { User, Plus, Edit2, Trash2, Shield } from 'lucide-react';

const Settings = () => {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<any>(null);
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({ queryKey: ['users'], queryFn: getUsers });
  const deleteMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] })
  });

  return (
    <div>
      <div className="flex justify-between items-center mb-8">
        <h1 className="text-2xl font-bold text-white">Settings</h1>
        <button onClick={() => { setEditingUser(null); setIsModalOpen(true); }} className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg flex items-center gap-2">
          <Plus size={18} /> Add User
        </button>
      </div>

      <div className="bg-gray-900 rounded-lg border border-gray-800 overflow-hidden">
        <div className="p-4 border-b border-gray-800 font-semibold text-gray-300 flex items-center gap-2">
            <Shield size={18} className="text-blue-500"/> User Management
        </div>
        <table className="w-full text-left">
          <thead>
            <tr className="bg-gray-800/50 text-gray-400 text-sm uppercase">
              <th className="px-6 py-4">Username</th>
              <th className="px-6 py-4">Role</th>
              <th className="px-6 py-4">Created At</th>
              <th className="px-6 py-4 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {isLoading ? <tr><td className="p-4 text-center" colSpan={4}>Loading...</td></tr> : 
             data?.data.map((user) => (
              <tr key={user.id} className="hover:bg-gray-800/30">
                <td className="px-6 py-4 text-white font-medium flex items-center gap-2">
                    <div className="w-8 h-8 rounded-full bg-gray-800 flex items-center justify-center text-gray-400"><User size={16}/></div>
                    {user.username}
                </td>
                <td className="px-6 py-4 text-gray-300">
                    <span className="bg-blue-900/30 text-blue-300 px-2 py-0.5 rounded text-xs border border-blue-800">{user.role}</span>
                </td>
                <td className="px-6 py-4 text-gray-400 text-sm">{new Date(user.created_at).toLocaleDateString()}</td>
                <td className="px-6 py-4 text-right flex justify-end gap-3">
                  <button onClick={() => { setEditingUser(user); setIsModalOpen(true); }} className="text-gray-400 hover:text-white"><Edit2 size={18} /></button>
                  <button onClick={() => { if(confirm('Delete user?')) deleteMutation.mutate(user.id) }} className="text-red-400 hover:text-red-300"><Trash2 size={18} /></button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <UserModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} user={editingUser} />
    </div>
  );
};

export default Settings;