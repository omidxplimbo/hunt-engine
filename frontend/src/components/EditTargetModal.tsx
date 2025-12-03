import { useState, useEffect } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { updateTarget, type UpdateTargetPayload } from '../api/targets';
import type { Target } from '../types/target';
import { X, Loader2 } from 'lucide-react';
import clsx from 'clsx'; // 👈 برای مدیریت کلاس‌های شرطی

interface Props {
  isOpen: boolean;
  onClose: () => void;
  target: Target | null;
}

// لیست ماژول‌های قابل انتخاب (شامل فاز جدید Crawling)
const MODULES = [
  { id: 'DISCOVERY', label: 'Discovery' },
  { id: 'PROBING', label: 'Probing' },
  { id: 'CRAWLING', label: 'Crawling' },
];

export const EditTargetModal = ({ isOpen, onClose, target }: Props) => {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState<UpdateTargetPayload>({});

  useEffect(() => {
    if (target) {
      // تلاش برای خواندن ماژول‌های فعال فعلی
      // اگر دیتای scan_modules از سمت بک‌اند بیاید آن را پارس می‌کنیم
      // در غیر این صورت پیش‌فرض همه را فعال در نظر می‌گیریم
      let existingModules: string[] = ['DISCOVERY', 'PROBING', 'CRAWLING'];
      
      // از آنجایی که ممکن است تایپ Target هنوز در فرانت آپدیت نشده باشد، از any استفاده می‌کنیم
      const t = target as any;
      if (t.scan_modules) {
          try {
              const parsed = typeof t.scan_modules === 'string' ? JSON.parse(t.scan_modules) : t.scan_modules;
              if (Array.isArray(parsed)) existingModules = parsed;
          } catch (e) { 
              console.error("Failed to parse modules", e); 
          }
      }

      setFormData({
        name: target.name,
        description: target.description,
        frequency: target.frequency,
        in_scope: target.in_scope,
        use_alterx: target.use_alterx,
        modules: existingModules, // 👈 لیست ماژول‌ها در فرم قرار می‌گیرد
      });
    }
  }, [target]);

  const mutation = useMutation({
    mutationFn: (data: UpdateTargetPayload) => updateTarget(target!.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['targets'] });
      onClose();
    },
  });

  if (!isOpen || !target) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    mutation.mutate(formData);
  };

  // تابع تغییر وضعیت ماژول‌ها
  const toggleModule = (moduleId: string) => {
    setFormData(prev => {
      const currentModules = prev.modules || [];
      const exists = currentModules.includes(moduleId);
      return {
        ...prev,
        modules: exists
          ? currentModules.filter(m => m !== moduleId)
          : [...currentModules, moduleId]
      };
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-xl shadow-2xl w-full max-w-md overflow-hidden">
        {/* Header */}
        <div className="flex justify-between items-center p-5 border-b border-gray-800 bg-gray-800/50">
          <h2 className="text-lg font-bold text-white">Edit Target: {target.name}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-white transition-colors">
            <X size={20} />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-5">
          {/* Name */}
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">Target Name</label>
            <input
              type="text"
              className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-600 outline-none"
              value={formData.name || ''}
              onChange={e => setFormData({ ...formData, name: e.target.value })}
            />
          </div>

          {/* Frequency */}
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">Frequency (Minutes)</label>
            <input
              type="number"
              className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-600 outline-none"
              value={formData.frequency || 0}
              onChange={e => setFormData({ ...formData, frequency: parseInt(e.target.value) })}
            />
          </div>

          {/* Modules Selection - بخش جدید برای انتخاب فازها */}
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-2">Active Phases</label>
            <div className="flex gap-2">
                {MODULES.map(mod => (
                    <div
                        key={mod.id}
                        onClick={() => toggleModule(mod.id)}
                        className={clsx(
                            "cursor-pointer px-3 py-1.5 rounded-md text-xs font-bold border transition-colors select-none",
                            (formData.modules || []).includes(mod.id)
                                ? "bg-blue-900/30 text-blue-400 border-blue-800"
                                : "bg-gray-950 text-gray-500 border-gray-800 hover:border-gray-700"
                        )}
                    >
                        {mod.label}
                    </div>
                ))}
            </div>
          </div>

          {/* Alterx Toggle */}
          <div className="flex items-center gap-3 p-3 bg-gray-950 border border-gray-800 rounded-lg">
             <input 
                type="checkbox" 
                id="edit_use_alterx"
                // اگر مقدار undefined بود (هنوز ست نشده)، پیش‌فرض true باشه
                checked={formData.use_alterx ?? true}
                onChange={e => setFormData({...formData, use_alterx: e.target.checked})}
                className="w-4 h-4 rounded border-gray-700 bg-gray-900 text-blue-600 focus:ring-blue-600 cursor-pointer"
             />
             <div className="flex flex-col">
                <label htmlFor="edit_use_alterx" className="text-sm font-medium text-gray-300 cursor-pointer">Enable Alterx</label>
                <span className="text-xs text-gray-500">Uncheck to skip heavy mutation phase.</span>
             </div>
          </div>

          {/* In Scope Toggle */}
          <div className="flex items-center gap-3">
             <input 
                type="checkbox" 
                id="in_scope"
                checked={formData.in_scope ?? true}
                onChange={e => setFormData({...formData, in_scope: e.target.checked})}
                className="w-4 h-4 rounded border-gray-700 bg-gray-900 text-blue-600 focus:ring-blue-600 cursor-pointer"
             />
             <label htmlFor="in_scope" className="text-sm text-gray-300 cursor-pointer">Active (In Scope)</label>
          </div>

          <div className="pt-2 flex gap-3">
            <button type="button" onClick={onClose} className="flex-1 px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-gray-300 font-medium transition-colors">Cancel</button>
            <button type="submit" disabled={mutation.isPending} className="flex-1 px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-700 text-white font-medium transition-colors">
              {mutation.isPending ? <Loader2 size={18} className="animate-spin mx-auto" /> : 'Save Changes'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};