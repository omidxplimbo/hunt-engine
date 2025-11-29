import { useState, useEffect } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { updateTarget, type UpdateTargetPayload } from '../api/targets';
import type { Target } from '../types/target';
import { X, Loader2 } from 'lucide-react';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  target: Target | null;
}

export const EditTargetModal = ({ isOpen, onClose, target }: Props) => {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState<UpdateTargetPayload>({});

  useEffect(() => {
    if (target) {
      setFormData({
        name: target.name,
        description: target.description,
        frequency: target.frequency,
        in_scope: target.in_scope,
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

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-800 rounded-xl shadow-2xl w-full max-w-md overflow-hidden">
        <div className="flex justify-between items-center p-5 border-b border-gray-800 bg-gray-800/50">
          <h2 className="text-lg font-bold text-white">Edit Target: {target.name}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-white transition-colors">
            <X size={20} />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">Target Name</label>
            <input
              type="text"
              className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-600 outline-none"
              value={formData.name || ''}
              onChange={e => setFormData({ ...formData, name: e.target.value })}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">Frequency (Minutes)</label>
            <input
              type="number"
              className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-white focus:ring-2 focus:ring-blue-600 outline-none"
              value={formData.frequency || 0}
              onChange={e => setFormData({ ...formData, frequency: parseInt(e.target.value) })}
            />
          </div>

          <div className="flex items-center gap-3">
             <input 
                type="checkbox" 
                id="in_scope"
                checked={formData.in_scope}
                onChange={e => setFormData({...formData, in_scope: e.target.checked})}
                className="w-4 h-4 rounded border-gray-700 bg-gray-900 text-blue-600 focus:ring-blue-600"
             />
             <label htmlFor="in_scope" className="text-sm text-gray-300">Active (In Scope)</label>
          </div>

          <div className="pt-2 flex gap-3">
            <button type="button" onClick={onClose} className="flex-1 px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-gray-300">Cancel</button>
            <button type="submit" disabled={mutation.isPending} className="flex-1 px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-700 text-white">
              {mutation.isPending ? <Loader2 size={18} className="animate-spin mx-auto" /> : 'Save Changes'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};