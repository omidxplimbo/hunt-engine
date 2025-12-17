import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getUsers, deleteUser, updateUser } from '../api/users';
import { UserModal } from '../components/UserModal';
import { User, Plus, Edit2, Trash2, Shield, Power } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { Navigate } from 'react-router-dom';

const Settings = () => {
  const { role } = useAuth();
  if (role !== 'admin') return <Navigate to="/" replace />;

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<any>(null);
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({ queryKey: ['users'], queryFn: getUsers });
  const deleteMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] })
  });
  const toggleActiveMutation = useMutation({
    mutationFn: ({ id, is_active }: { id: number; is_active: boolean }) => updateUser(id, { is_active }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] })
  });

  return (
    <div className="space-y-6">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center mb-8 border-b border-hack-border/50 pb-4 gap-4">
        <div>
          <h1 className="hack-title text-xl md:text-2xl">SYSTEM CONFIGURATION</h1>
          <p className="text-hack-dim text-xs font-mono mt-1 tracking-wider">Access Control & Parameters</p>
        </div>
        <button onClick={() => { setEditingUser(null); setIsModalOpen(true); }} className="hack-btn w-full md:w-auto">
          <Plus size={16} /> New Admin
        </button>
      </div>

      <div className="hack-box overflow-hidden relative flex flex-col">
        {/* Header Decor */}
        <div className="p-3 border-b border-hack-border bg-black/40 flex items-center justify-between">
            <div className="flex items-center gap-2 text-hack-primary font-mono text-sm tracking-wider">
                <Shield size={16} />
                <span className="uppercase">User_Privileges_DB</span>
            </div>
            <div className="flex gap-1">
                <div className="w-2 h-2 bg-hack-danger rounded-full animate-pulse"></div>
                <div className="w-2 h-2 bg-hack-warning rounded-full"></div>
                <div className="w-2 h-2 bg-hack-primary rounded-full"></div>
            </div>
        </div>

        <div className="overflow-x-auto">
            <table className="w-full text-left min-w-[600px]">
            <thead>
                <tr className="bg-hack-bg/50 text-hack-dim text-[10px] uppercase tracking-[0.2em] border-b border-hack-border">
                <th className="px-6 py-4 font-normal">Identity</th>
                <th className="px-6 py-4 font-normal">Clearance Level</th>
                <th className="px-6 py-4 font-normal">Status</th>
                <th className="px-6 py-4 font-normal">Registration Date</th>
                <th className="px-6 py-4 font-normal text-right">Override</th>
                </tr>
            </thead>
            <tbody className="divide-y divide-hack-border/30">
                {isLoading ? 
                <tr><td className="p-8 text-center font-mono text-hack-dim animate-pulse" colSpan={5}> DECRYPTING USER DATA...</td></tr> : 
                data?.data.map((user) => (
                <tr key={user.id} className="hover:bg-hack-primary/5 transition-colors group font-mono text-sm">
                    <td className="px-6 py-4 text-white">
                        <div className="flex items-center gap-3">
                            <div className="w-8 h-8 bg-black border border-hack-border flex items-center justify-center text-hack-dim group-hover:text-hack-primary group-hover:border-hack-primary/50 transition-all flex-shrink-0">
                                <User size={14}/>
                            </div>
                            <span className="tracking-wide">{user.username}</span>
                        </div>
                    </td>
                    <td className="px-6 py-4">
                        <span className={`hack-badge ${user.role === 'admin' ? 'border-hack-primary text-hack-primary bg-hack-primary/10' : 'border-hack-dim text-hack-dim'}`}>
                            {user.role}
                        </span>
                    </td>
                    <td className="px-6 py-4">
                      <span className={`hack-badge ${user.is_active ? 'border-hack-primary/60 text-hack-primary bg-hack-primary/5' : 'border-hack-danger/60 text-hack-danger bg-hack-danger/5'}`}>
                        {user.is_active ? 'active' : 'deactive'}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-hack-dim text-xs tracking-wider">
                        {new Date(user.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4 text-right">
                    <div className="flex justify-end gap-2 opacity-60 group-hover:opacity-100 transition-opacity">
                        <button
                          onClick={() => toggleActiveMutation.mutate({ id: user.id, is_active: !user.is_active })}
                          className={`p-2 transition-colors ${user.is_active ? 'hover:text-hack-warning' : 'hover:text-hack-primary'}`}
                          title={user.is_active ? 'DEACTIVATE' : 'ACTIVATE'}
                        >
                          <Power size={16} />
                        </button>
                        <button onClick={() => { setEditingUser(user); setIsModalOpen(true); }} className="p-2 hover:text-hack-primary transition-colors" title="MODIFY">
                            <Edit2 size={16} />
                        </button>
                        <button onClick={() => { if(confirm('>> EXECUTE DELETION PROTOCOL?')) deleteMutation.mutate(user.id) }} className="p-2 hover:text-hack-danger transition-colors" title="TERMINATE">
                            <Trash2 size={16} />
                        </button>
                    </div>
                    </td>
                </tr>
                ))}
            </tbody>
            </table>
        </div>
      </div>

      <UserModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} user={editingUser} />
    </div>
  );
};

export default Settings;