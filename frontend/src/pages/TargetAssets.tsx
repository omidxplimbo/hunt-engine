import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getTargetAssets, getTargetDetails, getTargetURLs } from '../api/targets';
import { ArrowLeft, Globe, CheckCircle, XCircle, Search, Monitor, Loader2, Network, ArrowUp, ArrowDown, Link2, FileText, Database, FileCode } from 'lucide-react';
import clsx from 'clsx';

const TargetAssets = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const targetId = Number(id);

  const [activeTab, setActiveTab] = useState<'assets' | 'urls'>('assets');
  const [page, setPage] = useState(1);
  const [filterLive, setFilterLive] = useState<boolean | undefined>(undefined);
  const [filterHttpx, setFilterHttpx] = useState<boolean | undefined>(undefined);
  const [filterDnsOnly, setFilterDnsOnly] = useState<boolean | undefined>(undefined);
  const [filterJsOnly, setFilterJsOnly] = useState<boolean>(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [sortBy, setSortBy] = useState<string>('value');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc');

  useEffect(() => {
    const timer = setTimeout(() => { setDebouncedSearch(searchTerm); setPage(1); }, 500);
    return () => clearTimeout(timer);
  }, [searchTerm]);

  useEffect(() => { setPage(1); setSearchTerm(""); }, [filterLive, filterHttpx, filterDnsOnly, filterJsOnly, activeTab]);

  const toggleHttpx = () => {
      if (!filterHttpx) { setFilterDnsOnly(undefined); setFilterHttpx(true); } else { setFilterHttpx(undefined); }
  };
  const toggleDnsOnly = () => {
      if (!filterDnsOnly) { setFilterHttpx(undefined); setFilterDnsOnly(true); } else { setFilterDnsOnly(undefined); }
  };
  const handleSort = (field: string) => {
    if (sortBy === field) { setSortOrder(prev => prev === 'asc' ? 'desc' : 'asc'); } else { setSortBy(field); setSortOrder('asc'); }
  };

  const SortableHeader = ({ field, label, className }: { field: string, label: string, className?: string }) => (
    <th className={clsx("px-6 py-3 cursor-pointer hover:text-hack-primary transition-colors select-none group border-b border-hack-border font-mono text-xs text-hack-dim uppercase tracking-wider", className)} onClick={() => handleSort(field)}>
      <div className={clsx("flex items-center gap-1", className?.includes("text-right") && "justify-end")}>
        {label}
        {sortBy === field && (sortOrder === 'asc' ? <ArrowUp size={12} className="text-hack-primary" /> : <ArrowDown size={12} className="text-hack-primary" />)}
      </div>
    </th>
  );

  const targetQuery = useQuery({ queryKey: ['target', targetId], queryFn: () => getTargetDetails(targetId), enabled: !!targetId });
  const assetsQuery = useQuery({ queryKey: ['assets', targetId, page, filterLive, filterHttpx, filterDnsOnly, debouncedSearch, sortBy, sortOrder], queryFn: () => getTargetAssets(targetId, page, 50, { is_live: filterLive, search: debouncedSearch, has_httpx: filterHttpx, dns_only: filterDnsOnly }, sortBy, sortOrder), enabled: !!targetId && activeTab === 'assets' });
  const urlsQuery = useQuery({ queryKey: ['urls', targetId, page, debouncedSearch, filterJsOnly], queryFn: () => getTargetURLs(targetId, page, 50, debouncedSearch, filterJsOnly), enabled: !!targetId && activeTab === 'urls' });

  const currentData = activeTab === 'assets' ? assetsQuery.data : urlsQuery.data;
  const isFetching = activeTab === 'assets' ? assetsQuery.isFetching : urlsQuery.isFetching;

  if (!targetId) return <div className="p-8 text-hack-danger font-mono"> FATAL ERROR: Invalid Target ID</div>;

  return (
    <div className="flex flex-col h-full space-y-6">
      <div className="flex flex-col md:flex-row items-start md:items-center gap-4 flex-shrink-0 border-b border-hack-border/50 pb-4">
        <button onClick={() => navigate('/targets')} className="p-2 hover:bg-white/5 rounded text-hack-dim hover:text-white transition-colors">
          <ArrowLeft size={20} />
        </button>
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="hack-title text-lg md:text-xl truncate">
              {targetQuery.isLoading ? 'LOADING...' : targetQuery.data?.name}
            </h1>
            <span className="hack-badge border-hack-primary/30 text-hack-primary bg-hack-primary/10 font-mono lowercase truncate max-w-[200px]">
              {targetQuery.data?.root_domain}
            </span>
            {isFetching && <Loader2 size={16} className="animate-spin text-hack-primary" />}
          </div>
        </div>
      </div>

      <div className="flex gap-2">
          <button onClick={() => setActiveTab('assets')} className={clsx("hack-btn flex-1 md:flex-none justify-center", activeTab === 'assets' ? "bg-hack-primary text-black" : "bg-transparent text-hack-dim border-hack-dim/30")}>
              <Database size={14} /> Assets
          </button>
          <button onClick={() => setActiveTab('urls')} className={clsx("hack-btn flex-1 md:flex-none justify-center", activeTab === 'urls' ? "bg-hack-primary text-black" : "bg-transparent text-hack-dim border-hack-dim/30")}>
              <Link2 size={14} /> Intel
          </button>
      </div>

      <div className="hack-box p-3 flex flex-col md:flex-row flex-wrap items-stretch md:items-center gap-4 justify-between flex-shrink-0">
        <div className="relative flex-1 min-w-[200px] flex items-center bg-black/40 border border-hack-border px-3 rounded-none">
          <Search className="text-hack-dim flex-shrink-0" size={14} />
          <input 
            type="text" 
            placeholder={activeTab === 'assets' ? "QUERY ASSETS..." : "QUERY INTEL..."}
            className="w-full bg-transparent border-none text-hack-primary pl-3 py-2 focus:ring-0 text-sm font-mono placeholder-hack-dim/50 min-w-0"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>

        {activeTab === 'assets' && (
            <div className="flex flex-wrap items-center gap-2 md:gap-4">
                <span className="text-[10px] uppercase text-hack-dim tracking-widest hidden md:inline">Filters:</span>
                <div className="flex gap-1">
                    <button onClick={() => setFilterLive(undefined)} className={clsx("hack-btn-ghost border border-transparent px-2", filterLive === undefined && "border-hack-dim/50 text-white")}>All</button>
                    <button onClick={() => setFilterLive(true)} className={clsx("hack-btn-ghost flex items-center gap-1 px-2", filterLive === true && "!text-hack-primary border border-hack-primary/50")}><CheckCircle size={12} /> Live</button>
                    <button onClick={() => setFilterLive(false)} className={clsx("hack-btn-ghost flex items-center gap-1 px-2", filterLive === false && "!text-hack-danger border border-hack-danger/50")}><XCircle size={12} /> Dead</button>
                </div>
                <div className="hidden md:block w-px bg-hack-border h-4"></div>
                <div className="flex gap-2 w-full md:w-auto">
                    <button onClick={toggleHttpx} className={clsx("hack-btn-ghost flex flex-1 md:flex-none justify-center items-center gap-1 border", filterHttpx ? "border-hack-primary text-hack-primary" : "border-hack-border")}><Monitor size={12} /> Web</button>
                    <button onClick={toggleDnsOnly} className={clsx("hack-btn-ghost flex flex-1 md:flex-none justify-center items-center gap-1 border", filterDnsOnly ? "border-hack-warning text-hack-warning" : "border-hack-border")}><Network size={12} /> DNS</button>
                </div>
            </div>
        )}

        {activeTab === 'urls' && (
            <div className="flex items-center gap-2 w-full md:w-auto">
                  <button onClick={() => setFilterJsOnly(!filterJsOnly)} className={clsx("hack-btn-ghost flex flex-1 md:flex-none justify-center items-center gap-1 border w-full", filterJsOnly ? "border-hack-warning text-hack-warning" : "border-hack-border")}>
                    <FileCode size={12} /> JS Only
                  </button>
            </div>
        )}
      </div>

      <div className="hack-box flex flex-col flex-1 overflow-hidden relative">
        <div className="absolute top-0 right-0 p-1 border-b border-l border-hack-primary/20 bg-hack-primary/5 text-[8px] text-hack-primary font-mono hidden md:block">DATA_GRID_V1</div>

        <div className="overflow-x-auto w-full flex-1">
            <table className="w-full text-left min-w-[1000px]"> 
            <thead className="sticky top-0 z-10 bg-black/90 backdrop-blur-sm">
                <tr>
                {activeTab === 'assets' ? (
                    <>
                    <SortableHeader field="value" label="Asset" className="w-[300px]" />
                    <th className="px-6 py-3 font-mono text-xs text-hack-dim uppercase tracking-wider border-b border-hack-border">Phase 1 IP</th>
                    <th className="px-6 py-3 font-mono text-xs text-hack-dim uppercase tracking-wider border-b border-hack-border">Phase 2 IP</th>
                    <SortableHeader field="status_code" label="Stat" className="w-[80px]" />
                    <SortableHeader field="title" label="Title" className="w-[250px]" />
                    <th className="px-6 py-3 font-mono text-xs text-hack-dim uppercase tracking-wider border-b border-hack-border">Stack</th>
                    <SortableHeader field="content_length" label="Size" className="w-[100px] text-right" />
                    </>
                ) : (
                    <>
                    <th className="px-6 py-3 font-mono text-xs text-hack-dim uppercase tracking-wider border-b border-hack-border w-[70%]">Resource Locator</th>
                    <th className="px-6 py-3 font-mono text-xs text-hack-dim uppercase tracking-wider border-b border-hack-border w-[15%]">Origin</th>
                    <th className="px-6 py-3 font-mono text-xs text-hack-dim uppercase tracking-wider border-b border-hack-border w-[15%] text-right">Timestamp</th>
                    </>
                )}
                </tr>
            </thead>
            <tbody className="divide-y divide-hack-border/30">
                {activeTab === 'assets' ? assetsQuery.data?.data.map((asset) => {
                    const statusCode = asset.status_code ?? 0;
                    let dnsxIps: string[] = [];
                    try { const p = JSON.parse(asset.dnsx_ip || "[]"); dnsxIps = Array.isArray(p) ? p : [p]; } catch { if (asset.dnsx_ip) dnsxIps = [asset.dnsx_ip]; }
                    let httpxIps: string[] = [];
                    try { const p = JSON.parse(asset.host_ip || "[]"); httpxIps = Array.isArray(p) ? p : [p]; } catch { if (asset.host_ip) httpxIps = [asset.host_ip]; }
                    let techs: string[] = [];
                    if (Array.isArray(asset.technologies)) { techs = asset.technologies as string[]; } else if (typeof asset.technologies === 'string') { try { techs = JSON.parse(asset.technologies); } catch {} }

                    return (
                    <tr key={asset.id} className="hover:bg-hack-primary/5 transition-colors font-mono text-sm group">
                        <td className="px-6 py-3 align-top">
                        <div className="flex items-start gap-3">
                            <Globe size={14} className={asset.is_live ? "text-hack-primary mt-1 flex-shrink-0" : "text-hack-dim mt-1 flex-shrink-0"} />
                            <div className="min-w-0"><a href={`http://${asset.value}`} target="_blank" rel="noreferrer" className="text-gray-300 hover:text-hack-primary transition-colors break-all group-hover:underline underline-offset-4">{asset.value}</a></div>
                        </div>
                        </td>
                        <td className="px-6 py-3 align-top"><div className="flex flex-col gap-1">{dnsxIps.length > 0 && dnsxIps[0] !== "" ? dnsxIps.map((ip, idx) => <span key={idx} className="text-[10px] text-hack-secondary">{ip}</span>) : <span className="text-hack-dim">-</span>}</div></td>
                        <td className="px-6 py-3 align-top"><div className="flex flex-col gap-1">{httpxIps.length > 0 && httpxIps[0] !== "" ? httpxIps.map((ip, idx) => <span key={idx} className="text-[10px] text-hack-dim">{ip}</span>) : <span className="text-hack-dim">-</span>}</div></td>
                        <td className="px-6 py-3 align-top">{asset.is_live ? (statusCode > 0 ? <span className={`px-1.5 py-0.5 text-[10px] font-bold border ${statusCode >= 200 && statusCode < 300 ? 'border-hack-primary text-hack-primary' : statusCode >= 300 && statusCode < 400 ? 'border-hack-warning text-hack-warning' : 'border-hack-danger text-hack-danger'}`}>{statusCode}</span> : <span className="text-hack-dim">-</span>) : <span className="text-[10px] text-hack-dim border border-hack-dim px-1">DEAD</span>}</td>
                        <td className="px-6 py-3 align-top"><span className="text-xs text-hack-text block line-clamp-2 opacity-80 min-w-[200px]" title={asset.title}>{asset.title || '-'}</span></td>
                        <td className="px-6 py-3 align-top"><div className="flex flex-wrap gap-1 max-h-[60px] overflow-y-auto min-w-[150px]">{asset.web_server && <span className="px-1 py-0.5 bg-white/5 text-[9px] border border-white/10 text-hack-text whitespace-nowrap">{asset.web_server}</span>}{techs.map((tech, i) => <span key={i} className="px-1 py-0.5 bg-hack-primary/5 text-[9px] border border-hack-primary/20 text-hack-primary whitespace-nowrap">{tech}</span>)}</div></td>
                        <td className="px-6 py-3 align-top text-right text-xs text-hack-dim">{asset.content_length ? `${(asset.content_length / 1024).toFixed(1)} KB` : '-'}</td>
                    </tr>
                    )}) : urlsQuery.data?.data.map((url) => (
                    <tr key={url.id} className="hover:bg-hack-primary/5 transition-colors group font-mono text-sm">
                        <td className="px-6 py-3 align-top">
                            <div className="flex items-start gap-3">
                                {url.value.endsWith('.js') ? <FileCode size={14} className="text-hack-warning mt-1 flex-shrink-0" /> : <FileText size={14} className="text-hack-dim mt-1 group-hover:text-hack-primary flex-shrink-0" />}
                                <a href={url.value} target="_blank" rel="noreferrer" className={clsx("transition-colors break-all block leading-relaxed hover:underline underline-offset-4 min-w-0", url.value.endsWith('.js') ? "text-hack-warning hover:text-white" : "text-hack-dim hover:text-hack-primary")}>{url.value}</a>
                            </div>
                        </td>
                        <td className="px-6 py-3 align-top"><span className="text-[10px] text-hack-dim border border-hack-border px-1 uppercase">{url.source}</span></td>
                        <td className="px-6 py-3 align-top text-right text-xs text-hack-dim whitespace-nowrap">{new Date(url.created_at).toLocaleString()}</td>
                    </tr>
                    ))}
            </tbody>
            </table>
        </div>
        
        <div className="border-t border-hack-border p-2 flex justify-between items-center bg-black/60 text-xs font-mono sticky bottom-0">
           <span className="text-hack-dim px-2">PAGE {page} // RECORDS: {currentData?.total_count?.toLocaleString() || 0}</span>
           <div className="flex gap-1">
             <button disabled={page === 1 || isFetching} onClick={() => setPage(p => Math.max(1, p - 1))} className="hack-btn-ghost hover:bg-white/5 disabled:opacity-30">PREV</button>
             <button disabled={!currentData?.data || currentData.data.length < 50 || isFetching} onClick={() => setPage(p => p + 1)} className="hack-btn-ghost hover:bg-white/5 disabled:opacity-30">NEXT</button>
           </div>
        </div>
      </div>
    </div>
  );
};

export default TargetAssets;