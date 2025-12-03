import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createTarget, type CreateTargetPayload } from '../api/targets';import { X, Loader2 } from 'lucide-react';
import clsx from 'clsx';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

const MODULES = [
  { id: 'DISCOVERY', label: 'Phase 1: Discovery (Subdomains)' },
  { id: 'PROBING', label: 'Phase 2: Probing (HTTP/Tech)' },
  { id: 'CRAWLING', label: 'Phase 3: Crawling (URLs/JS)' }, // 👈 اضافه شد
];

export const CreateTargetModal = ({ isOpen, onClose }: Props) => {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState<CreateTargetPayload>({
    name: '',
    root_domain: '',
    description: '',
    frequency: 720, // پیش‌فرض ۱۲ ساعت
    modules: ['DISCOVERY', 'PROBING', 'CRAWLING'],
    use_alterx: true, // 👈 پیش‌فرض روشن
  });

  // استفاده از React Query Mutation برای ارسال درخواست
  const mutation = useMutation({
    mutationFn: createTarget,
    onSuccess: () => {
      // رفرش کردن لیست تارگت‌ها بعد از موفقیت
      queryClient.invalidateQueries({ queryKey: ['targets'] });
      onClose();
      // ریست کردن فرم
      setFormData({
        name: '',
        root_domain: '',
        description: '',
        frequency: 720,
        modules: ['DISCOVERY', 'PROBING', 'CRAWLING'],
        use_alterx: true, // 👈 پیش‌فرض روشن

      });
    },
  });

  if (!isOpen) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    mutation.mutate(formData);
  };

  const toggleModule = (moduleId: string) => {
    setFormData(prev => {
      const exists = prev.modules.includes(moduleId);
      return {
        ...prev,
        modules: exists
          ? prev.modules.filter(m => m !== moduleId)
          : [...prev.modules, moduleId]
      };
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-xl shadow-2xl w-full max-w-md overflow-hidden">
        {/* Header */}
        <div className="flex justify-between items-center p-5 border-b border-gray-800 bg-gray-800/50">
          <h2 className="text-lg font-bold text-white">Add New Target</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-white transition-colors">
            <X size={20} />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="p-6 space-y-5">
          {/* Name */}
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">Target Name</label>
            <input
              type="text"
              required
              className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-600 focus:border-transparent outline-none"
              placeholder="e.g., Google Bug Bounty"
              value={formData.name}
              onChange={e => setFormData({ ...formData, name: e.target.value })}
            />
          </div>

          {/* Domain */}
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">Root Domain</label>
            <input
              type="text"
              required
              className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-600 focus:border-transparent outline-none"
              placeholder="e.g., google.com"
              value={formData.root_domain}
              onChange={e => setFormData({ ...formData, root_domain: e.target.value })}
            />
          </div>

          {/* Frequency */}
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">Scan Frequency (Minutes)</label>
            <input
              type="number"
              min="0"
              className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-600 focus:border-transparent outline-none"
              value={formData.frequency}
              onChange={e => setFormData({ ...formData, frequency: parseInt(e.target.value) || 0 })}
            />
            <p className="text-xs text-gray-500 mt-1">Set 0 to disable auto-scanning.</p>
          </div>

          {/* Alterx Toggle */}
          <div className="flex items-center gap-3 p-3 bg-gray-950 border border-gray-800 rounded-lg">
             <input 
                type="checkbox" 
                id="use_alterx"
                checked={formData.use_alterx}
                onChange={e => setFormData({...formData, use_alterx: e.target.checked})}
                className="w-4 h-4 rounded border-gray-700 bg-gray-900 text-blue-600 focus:ring-blue-600 cursor-pointer"
             />
             <div className="flex flex-col">
                <label htmlFor="use_alterx" className="text-sm font-medium text-gray-300 cursor-pointer">Enable Alterx (Mutation)</label>
                <span className="text-xs text-gray-500">Generates permutations to find hidden subdomains (Slower).</span>
             </div>
          </div>
          {/* Modules */}
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-2">Active Modules</label>
            <div className="space-y-2">
              {MODULES.map(mod => (
                <div
                  key={mod.id}
                  onClick={() => toggleModule(mod.id)}
                  className={clsx(
                    "flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-all",
                    formData.modules.includes(mod.id)
                      ? "bg-blue-900/20 border-blue-600 text-blue-100"
                      : "bg-gray-950 border-gray-800 text-gray-400 hover:border-gray-700"
                  )}
                >
                  <div className={clsx(
                    "w-4 h-4 rounded border flex items-center justify-center",
                    formData.modules.includes(mod.id) ? "bg-blue-600 border-blue-600" : "border-gray-600"
                  )}>
                    {formData.modules.includes(mod.id) && <div className="w-2 h-2 bg-white rounded-sm" />}
                  </div>
                  <span className="text-sm font-medium">{mod.label}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Actions */}
          <div className="pt-2 flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-gray-300 font-medium transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={mutation.isPending}
              className="flex-1 px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-700 text-white font-medium transition-colors flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {mutation.isPending ? (
                <>
                  <Loader2 size={18} className="animate-spin" />
                  Creating...
                </>
              ) : (
                'Create Target'
              )}
            </button>
          </div>

          {mutation.isError && (
            <div className="p-3 bg-red-900/20 border border-red-900/50 rounded-lg text-red-400 text-sm text-center">
              Error creating target. Check if domain is unique.
            </div>
          )}
        </form>
      </div>
    </div>
  );
};