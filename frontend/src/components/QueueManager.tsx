import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getQueue, removeFromQueue, moveQueueItem, clearQueue, type QueueItem } from '../api/system';
import { List, Trash2, ArrowUp, ArrowDown, Activity } from 'lucide-react';
import toast from 'react-hot-toast';

interface Props {
  title?: string;
  description?: string;
  compact?: boolean;
}

const describeQueueItem = (item: QueueItem) => {
  if (item.target_name || item.root_domain || item.module) {
    return `${item.module || 'JOB'}:${item.target_name || item.root_domain || item.target_id || 'UNKNOWN'}`;
  }
  return item.payload;
};

export const QueueManager = ({
  title = 'My Scan Queue',
  description = 'Queued scans for the current account.',
  compact = false,
}: Props) => {
  const queryClient = useQueryClient();

  const { data: queue = [], isLoading } = useQuery({
    queryKey: ['queue'],
    queryFn: getQueue,
    refetchInterval: 5000,
  });

  const removeMutation = useMutation({
    mutationFn: removeFromQueue,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['queue'] });
      toast.success('Item removed from queue');
    },
    onError: (err: any) => toast.error(err.response?.data?.error || 'Failed to remove item'),
  });

  const clearMutation = useMutation({
    mutationFn: clearQueue,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['queue'] });
      toast.success('Queue cleared');
    },
    onError: (err: any) => toast.error(err.response?.data?.error || 'Failed to clear queue'),
  });

  const moveMutation = useMutation({
    mutationFn: ({ index, direction }: { index: number; direction: 'top' | 'bottom' }) => moveQueueItem(index, direction),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['queue'] });
      toast.success('Queue priority updated');
    },
    onError: (err: any) => toast.error(err.response?.data?.error || 'Failed to move item'),
  });

  if (isLoading) {
    return <div className="font-mono text-hack-dim">LOADING QUEUE DATA...</div>;
  }

  return (
    <div className="border border-hack-border bg-black/30 p-5">
      <div className="mb-4 flex items-start justify-between gap-4">
        <div>
          <h3 className="font-mono text-lg uppercase tracking-wider text-hack-primary flex items-center gap-2">
            <List size={18} /> {title} <span className="text-hack-dim">{queue.length}</span>
          </h3>
          <p className="mt-1 text-xs text-hack-dim font-mono">{description}</p>
        </div>

        {queue.length > 0 && (
          <button
            onClick={() => {
              if (confirm('CLEAR YOUR QUEUED ITEMS?')) clearMutation.mutate();
            }}
            className="text-hack-danger hover:text-red-400 text-xs font-mono flex items-center gap-1 transition-colors"
          >
            <Trash2 size={14} /> PURGE
          </button>
        )}
      </div>

      {queue.length === 0 ? (
        <div className="flex items-center gap-3 border border-dashed border-hack-border p-4 text-hack-dim font-mono text-sm">
          <Activity size={16} /> QUEUE IS EMPTY
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-left font-mono text-sm">
            <thead>
              <tr className="border-b border-hack-border text-hack-dim uppercase text-xs tracking-wider">
                <th className="py-2 pr-3">#</th>
                <th className="py-2 pr-3">Target / Job</th>
                {!compact && <th className="py-2 pr-3">Owner</th>}
                <th className="py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {queue.map((item, idx) => (
                <tr key={`${item.index}-${item.payload}`} className="border-b border-hack-border/50">
                  <td className="py-3 pr-3 text-hack-primary">{idx + 1}</td>
                  <td className="py-3 pr-3">
                    <div className="text-white">{describeQueueItem(item)}</div>
                    <div className="mt-1 text-[11px] text-hack-dim break-all">{item.payload}</div>
                  </td>
                  {!compact && <td className="py-3 pr-3 text-hack-dim">{item.owner_username || '-'}</td>}
                  <td className="py-3 text-right">
                    <div className="inline-flex items-center gap-2">
                      {idx > 0 && (
                        <button
                          onClick={() => moveMutation.mutate({ index: item.index, direction: 'top' })}
                          className="p-1 hover:text-hack-primary transition-colors"
                          title="Move to top"
                        >
                          <ArrowUp size={16} />
                        </button>
                      )}
                      {idx < queue.length - 1 && (
                        <button
                          onClick={() => moveMutation.mutate({ index: item.index, direction: 'bottom' })}
                          className="p-1 hover:text-hack-primary transition-colors"
                          title="Move to bottom"
                        >
                          <ArrowDown size={16} />
                        </button>
                      )}
                      <button
                        onClick={() => removeMutation.mutate(item.index)}
                        className="p-1 hover:text-hack-danger transition-colors"
                        title="Remove"
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
