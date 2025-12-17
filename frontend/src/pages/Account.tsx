import { useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { KeyRound, User2 } from 'lucide-react';
import { changeMyPassword, getMe } from '../api/me';

const Account = () => {
  const { data: me, isLoading, isError } = useQuery({
    queryKey: ['me'],
    queryFn: getMe,
    refetchInterval: 30000,
  });

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [message, setMessage] = useState<string | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const createdAt = useMemo(() => {
    if (!me?.createdAt) return '-';
    const d = new Date(me.createdAt);
    return Number.isNaN(d.getTime()) ? me.createdAt : d.toLocaleString();
  }, [me?.createdAt]);

  const changePwMutation = useMutation({
    mutationFn: changeMyPassword,
    onSuccess: (res) => {
      setErrorMsg(null);
      setMessage(res?.message || 'Password changed');
      setCurrentPassword('');
      setNewPassword('');
    },
    onError: (err: any) => {
      setMessage(null);
      const apiErr = err?.response?.data?.error;
      setErrorMsg(apiErr || 'Failed to change password');
    },
  });

  if (isLoading) return <div className="p-4 md:p-8 text-hack-dim font-mono animate-pulse"> Loading account data...</div>;
  if (isError || !me) {
    return (
      <div className="p-4 md:p-8 border border-hack-danger text-hack-danger bg-hack-danger/5 font-mono text-sm md:text-base">
        SYSTEM ERROR: Failed to fetch account data.
      </div>
    );
  }

  return (
    <div className="space-y-6 md:space-y-8">
      <div className="flex items-center gap-2 border-b border-hack-border/50 pb-4">
        <User2 className="text-hack-primary" size={20} />
        <h1 className="hack-title text-xl md:text-2xl">ACCOUNT</h1>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Profile */}
        <div className="hack-box p-4 md:p-6">
          <div className="text-[10px] uppercase tracking-[0.2em] text-hack-dim mb-4">Profile</div>
          <div className="space-y-3 font-mono text-sm">
            <Row label="Username" value={me.username} />
            <Row label="Role" value={me.role} />
            <Row label="Created" value={createdAt} />
          </div>
        </div>

        {/* Change password */}
        <div className="hack-box p-4 md:p-6">
          <div className="flex items-center gap-2 text-hack-primary mb-4">
            <KeyRound size={16} />
            <div className="text-[10px] uppercase tracking-[0.2em] text-hack-dim">Change Password</div>
          </div>

          {message && (
            <div className="mb-4 border border-hack-primary/40 bg-hack-primary/5 text-hack-primary font-mono text-xs p-3">
              {message}
            </div>
          )}
          {errorMsg && (
            <div className="mb-4 border border-hack-danger/40 bg-hack-danger/5 text-hack-danger font-mono text-xs p-3">
              {errorMsg}
            </div>
          )}

          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault();
              setMessage(null);
              setErrorMsg(null);
              changePwMutation.mutate({ current_password: currentPassword, new_password: newPassword });
            }}
          >
            <div className="space-y-1">
              <label className="text-[10px] uppercase text-hack-dim tracking-widest">Current Password</label>
              <input
                type="password"
                className="hack-input w-full"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                required
              />
            </div>

            <div className="space-y-1">
              <label className="text-[10px] uppercase text-hack-dim tracking-widest">New Password</label>
              <input
                type="password"
                className="hack-input w-full"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
                minLength={6}
              />
            </div>

            <button type="submit" className="hack-btn w-full" disabled={changePwMutation.isPending}>
              {changePwMutation.isPending ? 'UPDATING...' : 'UPDATE PASSWORD'}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
};

const Row = ({ label, value }: { label: string; value: string }) => (
  <div className="flex items-center justify-between gap-4 border-b border-hack-border/30 pb-2">
    <div className="text-hack-dim text-xs tracking-wider">{label}</div>
    <div className="text-white text-sm">{value}</div>
  </div>
);

export default Account;


