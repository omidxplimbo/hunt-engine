import { useEffect, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createUser, updateUser } from '../api/users';
import { X, Loader2, UserPlus, Key, ShieldAlert, ListOrdered } from 'lucide-react';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  user?: any;
}

const defaultForm = {
  username: '',
  password: '',
  role: 'viewer',
  is_active: true,
  max_concurrent_scans: 1,
  wordlist_max_file_size_mb: 10,
  wordlist_max_total_size_mb: 100,
};

export const UserModal = ({ isOpen, onClose, user }: Props) => {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState(defaultForm);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  useEffect(() => {
    if (user) {
      setFormData({
        username: user.username || '',
        password: '',
        role: user.role || 'viewer',
        is_active: user.is_active ?? true,
        max_concurrent_scans: Number(user.max_concurrent_scans || 1),
        wordlist_max_file_size_mb: Math.round(Number(user.wordlist_max_file_size_bytes || 0) / 1048576),
        wordlist_max_total_size_mb: Math.round(Number(user.wordlist_max_total_size_bytes || 0) / 1048576),
      });
    } else {
      setFormData(defaultForm);
    }
    setErrorMsg(null);
  }, [user, isOpen]);

  const mutation = useMutation({
    mutationFn: (data: any) => {
      const payload: any = {
        username: data.username,
        role: data.role,
        is_active: data.is_active,
        max_concurrent_scans: Math.max(1, Number(data.max_concurrent_scans || 1)),
      };

      if (data.password) payload.password = data.password;

      return user ? updateUser(user.id, payload) : createUser(payload);
    },
    onSuccess: async () => {
      setErrorMsg(null);
      await queryClient.invalidateQueries({ queryKey: ['users'] });
      await queryClient.refetchQueries({ queryKey: ['users'] });
      onClose();
    },
    onError: (err: any) => {
      const apiErr = err?.response?.data?.error || err?.response?.data?.message;
      setErrorMsg(apiErr || 'Failed to save user');
    },
  });

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
      <div className="flex max-h-[90vh] w-full max-w-lg flex-col overflow-hidden border border-hack-primary/60 bg-hack-bg shadow-[0_0_30px_rgba(0,255,65,0.2)]">
        <div className="flex items-center justify-between border-b border-hack-border p-5">
          <h2 className="font-mono text-xl uppercase tracking-wider text-hack-primary flex items-center gap-2">
            <UserPlus size={22} /> {user ? 'MODIFY_USER' : 'ADD_USER'}
          </h2>
          <button onClick={onClose} className="text-hack-dim hover:text-white">
            <X size={20} />
          </button>
        </div>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            setErrorMsg(null);
            mutation.mutate(formData);
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <div className="min-h-0 flex-1 space-y-5 overflow-y-auto p-6 pr-7">
          {errorMsg && (
            <div className="border border-hack-danger/60 bg-hack-danger/10 p-3 text-sm text-hack-danger font-mono">
              {errorMsg}
            </div>
          )}

          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-widest text-hack-dim">Username ID</span>
            <div className="relative">
              <ShieldAlert size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-hack-dim" />
              <input
                value={formData.username}
                onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                className="w-full bg-black/40 border border-hack-border px-10 py-3 font-mono text-white focus:border-hack-primary focus:outline-none"
                placeholder="agent_007"
                required
              />
            </div>
          </label>

          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-widest text-hack-dim">
              Access Key {user && <span className="text-hack-dim">(leave blank to keep)</span>}
            </span>
            <div className="relative">
              <Key size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-hack-dim" />
              <input
                type="password"
                value={formData.password}
                onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                className="w-full bg-black/40 border border-hack-border px-10 py-3 font-mono text-white focus:border-hack-primary focus:outline-none"
                placeholder={user ? '********' : 'minimum 6 chars'}
                required={!user}
                minLength={user ? undefined : 6}
              />
            </div>
          </label>

          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-widest text-hack-dim">Clearance Level</span>
            <select
              value={formData.role}
              onChange={(e) => setFormData({ ...formData, role: e.target.value })}
              className="w-full bg-black/40 border border-hack-border px-3 py-3 font-mono uppercase text-hack-primary focus:border-hack-primary focus:outline-none"
            >
              <option value="admin">LEVEL 5 (ADMIN)</option>
              <option value="viewer">LEVEL 1 (VIEWER)</option>
            </select>
          </label>

          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-widest text-hack-dim">Scan Queue Slots</span>
            <div className="relative">
              <ListOrdered size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-hack-dim" />
              <input
                type="number"
                min={1}
                value={formData.max_concurrent_scans}
                onChange={(e) =>
                  setFormData({ ...formData, max_concurrent_scans: Math.max(1, Number(e.target.value || 1)) })
                }
                className="w-full bg-black/40 border border-hack-border px-10 py-3 font-mono text-white focus:border-hack-primary focus:outline-none"
              />
            </div>
            <p className="mt-2 text-[11px] leading-relaxed text-hack-dim font-mono">
              Regular users can run this many scans at the same time. Extra executions wait in that user&apos;s queue.
              Admin accounts keep unlimited per-user slots, but this value is saved for audit/config visibility.
            </p>
          </label>

          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-widest text-hack-dim">Account Status</span>
            <select
              value={formData.is_active ? 'active' : 'deactive'}
              onChange={(e) => setFormData({ ...formData, is_active: e.target.value === 'active' })}
              className="w-full bg-black/40 border border-hack-border px-3 py-3 font-mono uppercase text-hack-primary focus:border-hack-primary focus:outline-none"
            >
              <option value="active">ACTIVE</option>
              <option value="deactive">DEACTIVE</option>
            </select>
          </label>

            <div>
              <span className="mb-2 block text-xs uppercase tracking-widest text-hack-dim">
                WORDLIST FILE LIMIT / MB
              </span>
              <input
                type="number"
                min="0"
                value={formData.wordlist_max_file_size_mb}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    wordlist_max_file_size_mb: Math.max(0, Number(e.target.value || 0)),
                  })
                }
                className="w-full border border-hack-border bg-black/60 px-4 py-3 font-mono text-hack-primary outline-none focus:border-hack-primary"
                placeholder="0 = unlimited"
              />
              <p className="mt-2 text-xs leading-relaxed text-hack-dim">
                Maximum size for each uploaded custom PureDNS wordlist. Use 0 for unlimited.
              </p>
            </div>

            <div>
              <span className="mb-2 block text-xs uppercase tracking-widest text-hack-dim">
                TOTAL WORDLIST STORAGE / MB
              </span>
              <input
                type="number"
                min="0"
                value={formData.wordlist_max_total_size_mb}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    wordlist_max_total_size_mb: Math.max(0, Number(e.target.value || 0)),
                  })
                }
                className="w-full border border-hack-border bg-black/60 px-4 py-3 font-mono text-hack-primary outline-none focus:border-hack-primary"
                placeholder="0 = unlimited"
              />
              <p className="mt-2 text-xs leading-relaxed text-hack-dim">
                Maximum total storage for all custom wordlists owned by this user. Use 0 for unlimited.
              </p>
            </div>


          </div>

          <div className="flex shrink-0 gap-3 border-t border-hack-border/50 bg-hack-bg p-6 pt-4">
            <button type="button" onClick={onClose} className="hack-btn-ghost flex-1 border border-hack-border py-3">
              CANCEL
            </button>
            <button type="submit" disabled={mutation.isPending} className="hack-btn flex-1 py-3">
              {mutation.isPending ? <Loader2 className="animate-spin mx-auto" size={18} /> : user ? 'UPDATE_RECORD' : 'GRANT_ACCESS'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
