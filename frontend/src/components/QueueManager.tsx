import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getQueue, removeFromQueue, moveQueueItem, clearQueue } from '../api/system';
import { List, Trash2, ArrowUp, Activity } from 'lucide-react';
import toast from 'react-hot-toast';

export const QueueManager = () => {
  const queryClient = useQueryClient();

  const { data: queue, isLoading } = useQuery({
    queryKey: ['queue'],
    queryFn: getQueue,
    refetchInterval: 5000 // Auto-refresh queue every 5s
  });

  const removeMutation = useMutation({
    mutationFn: removeFromQueue,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['queue'] });
      toast.success('Item removed from queue');
    },
    onError: (err: any) => {
      const errorMessage = err.response?.data?.error || 'Failed to remove item';
      toast.error(errorMessage);
    }
  });

  const clearMutation = useMutation({
    mutationFn: clearQueue,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['queue'] });
      toast.success('Queue cleared');
    },
    onError: (err: any) => {
      const errorMessage = err.response?.data?.error || 'Failed to clear queue';
      toast.error(errorMessage);
    }
  });

  const moveMutation = useMutation({
    mutationFn: ({ index, direction }: { index: number; direction: 'top' | 'bottom' }) => 
      moveQueueItem(index, direction),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['queue'] });
      toast.success('Queue priority updated');
    },
    onError: (err: any) => {
      const errorMessage = err.response?.data?.error || 'Failed to move item';
      toast.error(errorMessage);
    }
  });

  if (isLoading) return <div className="hack-box p-6 animate-pulse">LOADING QUEUE DATA...</div>;

  return (
    <div className="hack-box flex flex-col h-full w-full">
      <div className="p-4 border-b border-hack-border bg-black/40 flex items-center justify-between">
        <div className="flex items-center gap-2 text-hack-primary font-mono text-sm tracking-wider uppercase">
          <List size={16} />
          <span>Active Scan Queue</span>
          <span className="hack-badge ml-2 text-xs">{queue?.length || 0}</span>
        </div>
        
        {queue && queue.length > 0 && (
          <button 
            onClick={() => { if(confirm('CLEAR ALL QUEUED ITEMS?')) clearMutation.mutate(); }}
            className="text-hack-danger hover:text-red-400 text-xs font-mono flex items-center gap-1 transition-colors"
          >
            <Trash2 size={12} /> PURGE ALL
          </button>
        )}
      </div>

      <div className="overflow-auto max-h-[400px]">
        {queue?.length === 0 ? (
          <div className="p-8 text-center text-hack-dim font-mono text-sm flex flex-col items-center gap-2">
            <Activity size={24} className="opacity-50" />
            <span>QUEUE IS EMPTY</span>
            <span className="text-xs opacity-50">System is processing tasks in real-time</span>
          </div>
        ) : (
          <table className="w-full text-left">
            <thead className="sticky top-0 bg-black z-10">
              <tr className="text-hack-dim text-[10px] uppercase tracking-wider border-b border-hack-border">
                <th className="px-4 py-2 font-normal">#</th>
                <th className="px-4 py-2 font-normal">Payload</th>
                <th className="px-4 py-2 font-normal text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-hack-border/30">
              {queue?.map((item, idx) => (
                <tr key={item.index} className="hover:bg-hack-primary/5 transition-colors font-mono text-sm group">
                  <td className="px-4 py-3 text-hack-dim">{idx + 1}</td>
                  <td className="px-4 py-3 text-white truncate max-w-[200px]" title={item.payload}>
                    {item.payload}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex justify-end gap-1 opacity-60 group-hover:opacity-100 transition-opacity">
                      {idx > 0 && (
                        <button 
                          onClick={() => moveMutation.mutate({ index: item.index, direction: 'top' })}
                          className="p-1 hover:text-hack-primary transition-colors"
                          title="Move to Top"
                        >
                          <ArrowUp size={14} />
                        </button>
                      )}
                      
                      <button 
                        onClick={() => removeMutation.mutate(item.index)}
                        className="p-1 hover:text-hack-danger transition-colors"
                        title="Remove"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
};
