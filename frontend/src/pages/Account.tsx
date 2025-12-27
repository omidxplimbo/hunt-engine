import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { KeyRound, User2, Key, Plus, Save, Trash2, Eye, EyeOff } from 'lucide-react';
import { changeMyPassword, getMe, getMySubfinderProviders, putMySubfinderProviders, type SubfinderProviderItem } from '../api/me';

const Account = () => {
  const { data: me, isLoading, isError } = useQuery({
    queryKey: ['me'],
    queryFn: getMe,
    refetchInterval: 30000,
  });

  const subfinderProvidersQuery = useQuery({
    queryKey: ['me', 'subfinder', 'providers'],
    queryFn: getMySubfinderProviders,
    staleTime: 60_000,
  });

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [message, setMessage] = useState<string | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const [sfMessage, setSfMessage] = useState<string | null>(null);
  const [sfError, setSfError] = useState<string | null>(null);
  const [showKeys, setShowKeys] = useState(false);
  const [sfRows, setSfRows] = useState<Array<{ provider: string; apiKey: string }>>([]);

  const createdAt = useMemo(() => {
    if (!me?.createdAt) return '-';
    const d = new Date(me.createdAt);
    return Number.isNaN(d.getTime()) ? me.createdAt : d.toLocaleString();
  }, [me?.createdAt]);

  // Initialize subfinder rows from API
  useEffect(() => {
    const data = subfinderProvidersQuery.data;
    if (!data) return;
    const rows = data
      .map((p) => {
        const apiKey = Array.isArray(p.entries) && typeof p.entries[0] === 'string' ? (p.entries[0] as string) : '';
        return { provider: p.provider || '', apiKey };
      })
      .filter((r) => r.provider.trim() !== '');
    setSfRows(rows);
  }, [subfinderProvidersQuery.data]);

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

  const saveSubfinderMutation = useMutation({
    mutationFn: async () => {
      const providers: SubfinderProviderItem[] = sfRows
        .map((r) => ({
          provider: r.provider.trim().toLowerCase(),
          entries: r.apiKey.trim() ? [r.apiKey.trim()] : [],
        }))
        .filter((p) => p.provider !== '' && p.entries.length > 0); // don't store empty entries
      return putMySubfinderProviders(providers);
    },
    onSuccess: (res) => {
      setSfError(null);
      setSfMessage(res?.message || 'Saved');
      subfinderProvidersQuery.refetch();
    },
    onError: (err: any) => {
      setSfMessage(null);
      const apiErr = err?.response?.data?.error;
      setSfError(apiErr || 'Failed to save subfinder providers');
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

      {/* Subfinder provider API keys */}
      <div className="hack-box p-4 md:p-6">
        <div className="flex items-center justify-between gap-3 mb-4">
          <div className="flex items-center gap-2 text-hack-primary">
            <Key size={16} />
            <div className="text-[10px] uppercase tracking-[0.2em] text-hack-dim">Subfinder Provider API Keys</div>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setShowKeys((v) => !v)}
              className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white"
              title={showKeys ? 'Hide keys' : 'Show keys'}
            >
              {showKeys ? <EyeOff size={14} /> : <Eye size={14} />}
            </button>
            <button
              type="button"
              onClick={() => setSfRows((prev) => [...prev, { provider: '', apiKey: '' }])}
              className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white"
              title="Add provider key"
            >
              <Plus size={14} /> Add
            </button>
            <button
              type="button"
              onClick={() => {
                setSfMessage(null);
                setSfError(null);
                saveSubfinderMutation.mutate();
              }}
              className="hack-btn border border-hack-primary/60 px-3 py-1 text-[10px] uppercase tracking-wider"
              disabled={saveSubfinderMutation.isPending}
              title="Save keys"
            >
              <Save size={14} /> {saveSubfinderMutation.isPending ? 'Saving...' : 'Save'}
            </button>
          </div>
        </div>

        <div className="text-xs font-mono text-hack-dim mb-4">
          These keys are stored <span className="text-white">per-user</span> and are only used when <span className="text-white">Subfinder</span> runs for targets owned by you.
        </div>

        {sfMessage && (
          <div className="mb-4 border border-hack-primary/40 bg-hack-primary/5 text-hack-primary font-mono text-xs p-3">
            {sfMessage}
          </div>
        )}
        {sfError && (
          <div className="mb-4 border border-hack-danger/40 bg-hack-danger/5 text-hack-danger font-mono text-xs p-3">
            {sfError}
          </div>
        )}

        {subfinderProvidersQuery.isLoading ? (
          <div className="text-hack-dim font-mono text-sm animate-pulse"> Loading subfinder providers...</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left min-w-[600px]">
              <thead>
                <tr className="bg-hack-bg/50 text-hack-dim text-[10px] uppercase tracking-[0.2em] border-b border-hack-border">
                  <th className="px-4 py-3 font-normal">Provider</th>
                  <th className="px-4 py-3 font-normal">API Key</th>
                  <th className="px-4 py-3 font-normal text-right">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-hack-border/30 font-mono text-sm">
                {sfRows.length === 0 ? (
                  <tr>
                    <td colSpan={3} className="px-4 py-6 text-hack-dim text-xs">
                      No provider keys configured. Click <span className="text-white">Add</span> to add one.
                    </td>
                  </tr>
                ) : (
                  sfRows.map((row, idx) => (
                    <tr key={idx} className="hover:bg-hack-primary/5 transition-colors">
                      <td className="px-4 py-3 align-top">
                        <input
                          className="hack-input w-full"
                          placeholder="e.g. shodan"
                          value={row.provider}
                          onChange={(e) => {
                            const v = e.target.value;
                            setSfRows((prev) => prev.map((r, i) => (i === idx ? { ...r, provider: v } : r)));
                          }}
                        />
                      </td>
                      <td className="px-4 py-3 align-top">
                        <input
                          className="hack-input w-full"
                          placeholder="API key"
                          type={showKeys ? 'text' : 'password'}
                          value={row.apiKey}
                          onChange={(e) => {
                            const v = e.target.value;
                            setSfRows((prev) => prev.map((r, i) => (i === idx ? { ...r, apiKey: v } : r)));
                          }}
                        />
                      </td>
                      <td className="px-4 py-3 align-top text-right">
                        <button
                          type="button"
                          className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-hack-danger hover:border-hack-danger/40"
                          title="Remove row"
                          onClick={() => setSfRows((prev) => prev.filter((_, i) => i !== idx))}
                        >
                          <Trash2 size={14} />
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        )}
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


