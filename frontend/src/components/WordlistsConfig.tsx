import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Download,
  FileText,
  Link2,
  Loader2,
  RefreshCw,
  Trash2,
  Upload,
} from "lucide-react";
import clsx from "clsx";
import {
  deleteMyWordlist,
  downloadMyWordlist,
  getMyWordlists,
  uploadMyWordlistFile,
  uploadMyWordlistURL,
  type MyWordlist,
} from "../api/me";

const formatBytes = (value: number | undefined | null) => {
  const n = Number(value || 0);
  if (!Number.isFinite(n) || n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = n;
  let idx = 0;
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024;
    idx += 1;
  }
  return `${size.toFixed(size >= 10 || idx === 0 ? 0 : 1)} ${units[idx]}`;
};

const formatLimit = (value: number | undefined | null, unlimited: boolean) => {
  if (unlimited || !value || value <= 0) return "Unlimited";
  return formatBytes(value);
};

const formatDate = (value?: string) => {
  if (!value) return "-";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString();
};

const sourceClass = (source: string) => {
  switch (String(source || "").toLowerCase()) {
    case "url":
      return "border-hack-warning text-hack-warning bg-hack-warning/10";
    case "file":
      return "border-hack-primary text-hack-primary bg-hack-primary/10";
    default:
      return "border-hack-border text-hack-dim bg-black/30";
  }
};

const WordlistRow = ({
  item,
  onDownload,
  onDelete,
  busy,
}: {
  item: MyWordlist;
  onDownload: (item: MyWordlist) => void;
  onDelete: (item: MyWordlist) => void;
  busy: boolean;
}) => (
  <tr className="border-b border-hack-border/50 align-top">
    <td className="py-3 pr-3">
      <div className="flex items-center gap-2 text-white">
        <FileText className="h-4 w-4 text-hack-primary" />
        <span>{item.name}</span>
      </div>
      <div className="mt-1 break-all font-mono text-[10px] text-hack-dim">
        PureDNS path: {item.puredns_path}
      </div>
      <div className="mt-1 break-all font-mono text-[10px] text-hack-dim">
        SHA256: {item.sha256 || "-"}
      </div>
    </td>

    <td className="py-3 pr-3">
      <span
        className={clsx(
          "inline-flex border px-2 py-1 font-mono text-[10px] uppercase tracking-wider",
          sourceClass(item.source),
        )}
      >
        {item.source || "unknown"}
      </span>
    </td>

    <td className="py-3 pr-3 font-mono text-xs text-hack-dim">
      {formatBytes(item.size_bytes)}
    </td>

    <td className="py-3 pr-3 font-mono text-xs text-hack-dim">
      {item.lines ?? 0}
    </td>

    <td className="py-3 pr-3 font-mono text-xs text-hack-dim">
      {formatDate(item.created_at)}
    </td>

    <td className="py-3 pr-3">
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => onDownload(item)}
          disabled={busy}
          className="hack-btn-ghost border border-hack-border px-2 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white disabled:opacity-50"
        >
          <Download className="h-3 w-3" /> Download
        </button>
        <button
          type="button"
          onClick={() => onDelete(item)}
          disabled={busy}
          className="hack-btn-ghost border border-hack-danger/60 px-2 py-1 text-[10px] uppercase tracking-wider text-hack-danger disabled:opacity-50"
        >
          <Trash2 className="h-3 w-3" /> Delete
        </button>
      </div>
    </td>
  </tr>
);

const WordlistsConfig = () => {
  const queryClient = useQueryClient();
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [url, setUrl] = useState("");
  const [message, setMessage] = useState<string | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const query = useQuery({
    queryKey: ["me", "wordlists"],
    queryFn: getMyWordlists,
    staleTime: 15_000,
  });

  const data = query.data;
  const wordlists = data?.wordlists || [];

  const usagePercent = useMemo(() => {
    if (!data || data.unlimited || !data.max_total_size_bytes) return 0;
    return Math.min(
      100,
      Math.round(
        (Number(data.current_total_size_bytes || 0) /
          Number(data.max_total_size_bytes || 1)) *
          100,
      ),
    );
  }, [data]);

  const refreshAllWordlistQueries = () => {
    queryClient.invalidateQueries({ queryKey: ["me", "wordlists"] });
    queryClient.invalidateQueries({ queryKey: ["wordlists"] });
  };

  const fileMutation = useMutation({
    mutationFn: async () => {
      if (!selectedFile) throw new Error("Select a .txt wordlist first");
      if (!selectedFile.name.toLowerCase().endsWith(".txt")) {
        throw new Error("Only .txt wordlists are allowed");
      }
      return uploadMyWordlistFile(selectedFile);
    },
    onSuccess: (row) => {
      setSelectedFile(null);
      setErrorMsg(null);
      setMessage(`Wordlist uploaded: ${row.name}`);
      refreshAllWordlistQueries();
    },
    onError: (err: any) => {
      setMessage(null);
      setErrorMsg(
        err?.response?.data?.message ||
          err?.message ||
          "Failed to upload wordlist",
      );
    },
  });

  const urlMutation = useMutation({
    mutationFn: async () => {
      const value = url.trim();
      if (!value) throw new Error("Enter a public .txt URL first");
      return uploadMyWordlistURL(value);
    },
    onSuccess: (row) => {
      setUrl("");
      setErrorMsg(null);
      setMessage(`Wordlist imported from URL: ${row.name}`);
      refreshAllWordlistQueries();
    },
    onError: (err: any) => {
      setMessage(null);
      setErrorMsg(
        err?.response?.data?.message ||
          err?.message ||
          "Failed to import wordlist from URL",
      );
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteMyWordlist(id),
    onSuccess: () => {
      setErrorMsg(null);
      setMessage("Wordlist deleted");
      refreshAllWordlistQueries();
    },
    onError: (err: any) => {
      setMessage(null);
      setErrorMsg(
        err?.response?.data?.message ||
          err?.message ||
          "Failed to delete wordlist",
      );
    },
  });

  const downloadMutation = useMutation({
    mutationFn: (item: MyWordlist) => downloadMyWordlist(item.id, item.name),
    onError: (err: any) => {
      setMessage(null);
      setErrorMsg(
        err?.response?.data?.message ||
          err?.message ||
          "Failed to download wordlist",
      );
    },
  });

  const busy =
    fileMutation.isPending ||
    urlMutation.isPending ||
    deleteMutation.isPending ||
    downloadMutation.isPending;

  return (
    <div className="border border-hack-border bg-black/30 p-5">
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="flex items-center gap-2 font-mono text-lg uppercase tracking-wider text-hack-primary">
            <Upload className="h-5 w-5" /> Custom PureDNS Wordlists
          </h2>
          <p className="mt-1 max-w-4xl text-sm text-hack-dim">
            Upload account-scoped .txt wordlists for PureDNS. These wordlists are
            stored per user and appear in the PureDNS wordlist selector when
            creating or editing targets.
          </p>
        </div>

        <button
          type="button"
          onClick={() => refreshAllWordlistQueries()}
          className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white"
        >
          <RefreshCw className="h-3 w-3" /> Refresh
        </button>
      </div>

      {message && (
        <div className="mb-3 border border-hack-primary/60 bg-hack-primary/10 p-3 font-mono text-sm text-hack-primary">
          {message}
        </div>
      )}

      {errorMsg && (
        <div className="mb-3 border border-hack-danger/60 bg-hack-danger/10 p-3 font-mono text-sm text-hack-danger">
          {errorMsg}
        </div>
      )}

      <div className="mb-4 grid gap-3 lg:grid-cols-4">
        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Files
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {wordlists.length}
          </div>
        </div>

        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Current Usage
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {formatBytes(data?.current_total_size_bytes)}
          </div>
        </div>

        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Max File Size
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {formatLimit(data?.max_file_size_bytes, Boolean(data?.unlimited))}
          </div>
        </div>

        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Total Limit
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {formatLimit(data?.max_total_size_bytes, Boolean(data?.unlimited))}
          </div>
          {!data?.unlimited && data?.max_total_size_bytes ? (
            <div className="mt-2 h-1.5 bg-black">
              <div
                className="h-1.5 bg-hack-primary"
                style={{ width: `${usagePercent}%` }}
              />
            </div>
          ) : null}
        </div>
      </div>

      <div className="mb-4 grid gap-3 lg:grid-cols-2">
        <div className="border border-hack-border bg-black/20 p-4">
          <div className="mb-2 font-mono text-sm uppercase tracking-wider text-hack-primary">
            Upload .txt file
          </div>
          <div className="flex flex-col gap-2 sm:flex-row">
            <input
              type="file"
              accept=".txt,text/plain"
              onChange={(event) => setSelectedFile(event.target.files?.[0] || null)}
              className="w-full border border-hack-border bg-black px-3 py-2 text-sm text-hack-dim file:mr-3 file:border-0 file:bg-hack-primary file:px-3 file:py-1 file:font-mono file:text-xs file:uppercase file:text-black"
            />
            <button
              type="button"
              disabled={busy || !selectedFile}
              onClick={() => {
                setMessage(null);
                setErrorMsg(null);
                fileMutation.mutate();
              }}
              className="hack-btn justify-center px-4 py-2 disabled:opacity-50"
            >
              {fileMutation.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Upload className="h-4 w-4" />
              )}
              Upload
            </button>
          </div>
          <p className="mt-2 text-xs text-hack-dim">
            Only .txt files are accepted. Admin users are unlimited unless limits
            are explicitly enforced by account settings.
          </p>
        </div>

        <div className="border border-hack-border bg-black/20 p-4">
          <div className="mb-2 font-mono text-sm uppercase tracking-wider text-hack-primary">
            Import from public URL
          </div>
          <div className="flex flex-col gap-2 sm:flex-row">
            <input
              value={url}
              onChange={(event) => setUrl(event.target.value)}
              placeholder="https://example.com/wordlist.txt"
              className="w-full border border-hack-border bg-black px-3 py-2 font-mono text-sm text-white outline-none focus:border-hack-primary"
            />
            <button
              type="button"
              disabled={busy || !url.trim()}
              onClick={() => {
                setMessage(null);
                setErrorMsg(null);
                urlMutation.mutate();
              }}
              className="hack-btn justify-center px-4 py-2 disabled:opacity-50"
            >
              {urlMutation.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Link2 className="h-4 w-4" />
              )}
              Import
            </button>
          </div>
          <p className="mt-2 text-xs text-hack-dim">
            Private, localhost, loopback, and internal IP URLs are blocked to
            prevent SSRF.
          </p>
        </div>
      </div>

      {query.isLoading ? (
        <div className="border border-hack-border bg-black/20 p-6 text-center font-mono text-hack-dim">
          Loading custom wordlists...
        </div>
      ) : wordlists.length === 0 ? (
        <div className="border border-hack-border bg-black/20 p-6 text-center">
          <div className="font-mono text-hack-dim">
            No custom wordlists uploaded yet.
          </div>
          <div className="mt-2 text-sm text-hack-dim">
            Upload a .txt file or import one from a public URL.
          </div>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[920px] text-left text-sm">
            <thead>
              <tr className="border-b border-hack-border font-mono text-xs uppercase tracking-wider text-hack-dim">
                <th className="py-2 pr-3">Wordlist</th>
                <th className="py-2 pr-3">Source</th>
                <th className="py-2 pr-3">Size</th>
                <th className="py-2 pr-3">Lines</th>
                <th className="py-2 pr-3">Created</th>
                <th className="py-2 pr-3">Actions</th>
              </tr>
            </thead>
            <tbody>
              {wordlists.map((item) => (
                <WordlistRow
                  key={item.id}
                  item={item}
                  busy={busy}
                  onDownload={(row) => downloadMutation.mutate(row)}
                  onDelete={(row) => {
                    if (
                      window.confirm(
                        `Delete wordlist "${row.name}"? This removes the stored file too.`,
                      )
                    ) {
                      deleteMutation.mutate(row.id);
                    }
                  }}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default WordlistsConfig;
