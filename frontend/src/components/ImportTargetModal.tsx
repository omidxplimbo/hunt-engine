import { useState, useRef } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { importTargets, type ImportTargetPayload, type TargetExportData } from '../api/targets';
import { X, Loader2, Upload, FileText, AlertCircle, CheckCircle } from 'lucide-react';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

export const ImportTargetModal = ({ isOpen, onClose }: Props) => {
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [fileContent, setFileContent] = useState<TargetExportData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [skipExisting, setSkipExisting] = useState(true);

  const mutation = useMutation({
    mutationFn: (payload: ImportTargetPayload) => importTargets(payload),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['targets'] });
      // نمایش نتیجه
      const createdCount = (data?.data?.created && Array.isArray(data.data.created)) ? data.data.created.length : 0;
      const skippedCount = (data?.data?.skipped && Array.isArray(data.data.skipped)) ? data.data.skipped.length : 0;
      const errorsCount = (data?.data?.errors && Array.isArray(data.data.errors)) ? data.data.errors.length : 0;
      
      let message = `Import completed: ${createdCount} created`;
      if (skippedCount > 0) message += `, ${skippedCount} skipped`;
      if (errorsCount > 0) message += `, ${errorsCount} errors`;
      
      alert(message);
      onClose();
      reset();
    },
    onError: (err: any) => {
      const errorMessage = err.response?.data?.message || err.response?.data?.error || err.message || 'Failed to import targets';
      setError(errorMessage);
      console.error('Import error:', err.response?.data || err);
    },
  });

  const reset = () => {
    setFileContent(null);
    setError(null);
    setSkipExisting(true);
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setError(null);
    const reader = new FileReader();
    reader.onload = (event) => {
      try {
        const content = JSON.parse(event.target?.result as string) as TargetExportData;
        
        // اعتبارسنجی ساختار
        if (!content.version || !content.targets || !Array.isArray(content.targets)) {
          setError('Invalid export file format');
          return;
        }

        if (content.version !== '1.0') {
          setError(`Unsupported export format version: ${content.version}`);
          return;
        }

        setFileContent(content);
      } catch (err) {
        setError('Invalid JSON file');
      }
    };
    reader.onerror = () => {
      setError('Failed to read file');
    };
    reader.readAsText(file);
  };

  const handleImport = () => {
    if (!fileContent) {
      setError('Please select a file first');
      return;
    }

    mutation.mutate({
      data: fileContent,
      skip_existing: skipExisting,
    });
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
      <div className="hack-box w-full max-w-md relative animate-in fade-in zoom-in duration-200">
        <div className="absolute top-0 left-0 w-full h-[1px] bg-gradient-to-r from-transparent via-hack-primary to-transparent opacity-50"></div>
        
        <div className="flex justify-between items-center p-4 border-b border-hack-border">
          <div className="flex items-center gap-2 text-hack-primary">
            <Upload size={18} />
            <h2 className="font-mono font-bold tracking-widest text-lg">IMPORT_TARGETS</h2>
          </div>
          <button onClick={() => { onClose(); reset(); }} className="text-hack-dim hover:text-hack-danger transition-colors">
            <X size={18} />
          </button>
        </div>

        <div className="p-6 space-y-5">
          <div className="space-y-2">
            <label className="text-[10px] uppercase text-hack-dim tracking-widest">Select Export File</label>
            <div className="relative">
              <input
                ref={fileInputRef}
                type="file"
                accept=".json"
                onChange={handleFileSelect}
                className="hidden"
                id="file-input"
              />
              <label
                htmlFor="file-input"
                className="hack-input w-full cursor-pointer flex items-center gap-2 justify-center py-3 border-dashed"
              >
                <FileText size={18} className="text-hack-dim" />
                <span className="text-sm text-hack-text">Choose JSON file...</span>
              </label>
            </div>
          </div>

          {fileContent && (
            <div className="p-3 border border-hack-primary/30 bg-hack-primary/5 rounded space-y-2">
              <div className="flex items-center gap-2 text-hack-primary">
                <CheckCircle size={16} />
                <span className="text-xs font-mono font-bold">File Loaded</span>
              </div>
              <div className="text-xs text-hack-dim space-y-1">
                <div>Version: {fileContent.version}</div>
                <div>Export Date: {new Date(fileContent.export_date).toLocaleString()}</div>
                <div>Targets: {fileContent.targets?.length || 0}</div>
              </div>
            </div>
          )}

          {error && (
            <div className="p-3 border border-hack-danger/50 bg-hack-danger/10 rounded flex items-start gap-2">
              <AlertCircle size={16} className="text-hack-danger flex-shrink-0 mt-0.5" />
              <span className="text-xs text-hack-danger">{error}</span>
            </div>
          )}

          <div className="p-3 border border-hack-border bg-black/30 flex items-center gap-2">
            <input
              type="checkbox"
              id="skip_existing"
              checked={skipExisting}
              onChange={(e) => setSkipExisting(e.target.checked)}
              className="accent-hack-primary h-4 w-4"
            />
            <label htmlFor="skip_existing" className="text-xs text-hack-text cursor-pointer">
              Skip existing targets (by root_domain)
            </label>
          </div>

          <div className="pt-2 flex gap-3">
            <button
              type="button"
              onClick={() => { onClose(); reset(); }}
              className="flex-1 hack-btn-ghost border border-hack-border"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleImport}
              disabled={!fileContent || mutation.isPending}
              className="flex-1 hack-btn"
            >
              {mutation.isPending ? <Loader2 size={16} className="animate-spin" /> : 'IMPORT'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

