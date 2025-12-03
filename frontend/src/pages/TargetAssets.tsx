import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getTargetAssets, getTargetDetails, getTargetURLs } from '../api/targets';
import { 
  ArrowLeft, Globe, CheckCircle, XCircle, Search, Monitor, 
  Loader2, Network, ArrowUp, ArrowDown, Link2, FileText, Database, FileCode 
} from 'lucide-react'; // 👈 آیکون FileCode اضافه شد
import clsx from 'clsx';

const TargetAssets = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const targetId = Number(id);

  // Tab State
  const [activeTab, setActiveTab] = useState<'assets' | 'urls'>('assets');

  // Filters State
  const [page, setPage] = useState(1);
  const [filterLive, setFilterLive] = useState<boolean | undefined>(undefined);
  const [filterHttpx, setFilterHttpx] = useState<boolean | undefined>(undefined);
  const [filterDnsOnly, setFilterDnsOnly] = useState<boolean | undefined>(undefined);
  const [filterJsOnly, setFilterJsOnly] = useState<boolean>(false); // 👈 استیت جدید برای JS
  
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  const [sortBy, setSortBy] = useState<string>('value');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc');

  // Debounce Logic
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchTerm);
      setPage(1);
    }, 500);
    return () => clearTimeout(timer);
  }, [searchTerm]);

  // Reset page on filter change
  useEffect(() => {
    setPage(1);
    setSearchTerm("");
  }, [filterLive, filterHttpx, filterDnsOnly, filterJsOnly, activeTab]);

  // Filter Handlers
  const toggleHttpx = () => {
      if (!filterHttpx) {
          setFilterDnsOnly(undefined);
          setFilterHttpx(true);
      } else {
          setFilterHttpx(undefined);
      }
  };

  const toggleDnsOnly = () => {
      if (!filterDnsOnly) {
          setFilterHttpx(undefined);
          setFilterDnsOnly(true);
      } else {
          setFilterDnsOnly(undefined);
      }
  };

  const handleSort = (field: string) => {
    if (sortBy === field) {
      setSortOrder(prev => prev === 'asc' ? 'desc' : 'asc');
    } else {
      setSortBy(field);
      setSortOrder('asc');
    }
  };

  const SortableHeader = ({ field, label, className }: { field: string, label: string, className?: string }) => (
    <th 
      className={clsx("px-6 py-4 font-semibold cursor-pointer hover:text-white transition-colors select-none group", className)}
      onClick={() => handleSort(field)}
    >
      <div className={clsx("flex items-center gap-1", className?.includes("text-right") && "justify-end")}>
        {label}
        {sortBy === field && (sortOrder === 'asc' ? <ArrowUp size={14} className="text-blue-400" /> : <ArrowDown size={14} className="text-blue-400" />)}
        {sortBy !== field && <ArrowUp size={14} className="text-gray-700 opacity-0 group-hover:opacity-100 transition-opacity" />}
      </div>
    </th>
  );

  // Queries
  const targetQuery = useQuery({
    queryKey: ['target', targetId],
    queryFn: () => getTargetDetails(targetId),
    enabled: !!targetId,
  });

  const assetsQuery = useQuery({
    queryKey: ['assets', targetId, page, filterLive, filterHttpx, filterDnsOnly, debouncedSearch, sortBy, sortOrder],
    queryFn: () => getTargetAssets(
      targetId, page, 50, 
      { is_live: filterLive, search: debouncedSearch, has_httpx: filterHttpx, dns_only: filterDnsOnly },
      sortBy, sortOrder
    ),
    enabled: !!targetId && activeTab === 'assets', 
  });

  const urlsQuery = useQuery({
    // 👇 کلید کش شامل فیلتر JS می‌شود
    queryKey: ['urls', targetId, page, debouncedSearch, filterJsOnly],
    queryFn: () => getTargetURLs(targetId, page, 50, debouncedSearch, filterJsOnly),
    enabled: !!targetId && activeTab === 'urls', 
  });

  const currentData = activeTab === 'assets' ? assetsQuery.data : urlsQuery.data;
  const isFetching = activeTab === 'assets' ? assetsQuery.isFetching : urlsQuery.isFetching;

  if (!targetId) return <div className="p-8 text-red-500">Invalid Target ID</div>;

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center gap-4 mb-6 flex-shrink-0">
        <button 
          onClick={() => navigate('/targets')}
          className="p-2 hover:bg-gray-800 rounded-lg text-gray-400 hover:text-white transition-colors"
        >
          <ArrowLeft size={24} />
        </button>
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold text-white">
              {targetQuery.isLoading ? 'Loading...' : targetQuery.data?.name}
            </h1>
            <span className="px-2 py-0.5 rounded-md bg-blue-900/30 text-blue-400 text-xs border border-blue-800 font-mono">
              {targetQuery.data?.root_domain}
            </span>
            {isFetching && <Loader2 size={16} className="animate-spin text-blue-400" />}
          </div>
          <p className="text-gray-400 text-sm mt-1">Asset Explorer & Content Discovery</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 bg-gray-900/50 p-1 rounded-lg w-fit mb-6 border border-gray-800">
          <button
            onClick={() => setActiveTab('assets')}
            className={clsx(
                "px-4 py-2 rounded-md text-sm font-medium flex items-center gap-2 transition-all",
                activeTab === 'assets' ? "bg-gray-800 text-white shadow-sm ring-1 ring-white/10" : "text-gray-400 hover:text-gray-200"
            )}
          >
              <Database size={16} /> Subdomains / Assets
          </button>
          <button
            onClick={() => setActiveTab('urls')}
            className={clsx(
                "px-4 py-2 rounded-md text-sm font-medium flex items-center gap-2 transition-all",
                activeTab === 'urls' ? "bg-gray-800 text-white shadow-sm ring-1 ring-white/10" : "text-gray-400 hover:text-gray-200"
            )}
          >
              <Link2 size={16} /> Crawled URLs
          </button>
      </div>

      {/* Filters & Search Toolbar */}
      <div className="bg-gray-900 p-4 rounded-lg border border-gray-800 mb-6 flex flex-wrap items-center gap-4 justify-between flex-shrink-0">
        <div className="relative flex-1 min-w-[200px] max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" size={18} />
          <input 
            type="text" 
            placeholder={activeTab === 'assets' ? "Search subdomains..." : "Search URLs (e.g. login, admin)..."}
            className="w-full bg-gray-950 border border-gray-800 text-gray-200 pl-10 pr-4 py-2 rounded-md focus:outline-none focus:border-blue-500 transition-colors"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>

        {/* Filters for ASSETS Tab */}
        {activeTab === 'assets' && (
            <div className="flex items-center gap-3">
            <span className="text-sm font-medium text-gray-400">Filters:</span>
            <div className="flex bg-gray-950 rounded-lg p-1 border border-gray-800 gap-1">
                <div className="flex bg-gray-900 rounded-md p-0.5 border border-gray-800">
                    <button onClick={() => setFilterLive(undefined)} className={clsx("px-3 py-1.5 rounded text-sm font-medium transition-colors", filterLive === undefined ? "bg-gray-800 text-white shadow-sm" : "text-gray-400 hover:text-gray-200")}>All</button>
                    <button onClick={() => setFilterLive(true)} className={clsx("px-3 py-1.5 rounded text-sm font-medium transition-colors flex items-center gap-1.5", filterLive === true ? "bg-green-900/30 text-green-400 border border-green-800" : "text-gray-400 hover:text-gray-200")}><CheckCircle size={14} /> Live</button>
                    <button onClick={() => setFilterLive(false)} className={clsx("px-3 py-1.5 rounded text-sm font-medium transition-colors flex items-center gap-1.5", filterLive === false ? "bg-red-900/30 text-red-400 border border-red-800" : "text-gray-400 hover:text-gray-200")}><XCircle size={14} /> Dead</button>
                </div>
                <div className="w-px bg-gray-800 mx-1 h-6 self-center"></div>
                <button onClick={toggleHttpx} className={clsx("px-3 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-2 border", filterHttpx ? "bg-blue-900/30 text-blue-400 border-blue-800" : "bg-transparent border-transparent text-gray-400 hover:bg-gray-900 hover:text-gray-200")}><Monitor size={14} /> Has Web</button>
                <button onClick={toggleDnsOnly} className={clsx("px-3 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-2 border", filterDnsOnly ? "bg-purple-900/30 text-purple-400 border-purple-800" : "bg-transparent border-transparent text-gray-400 hover:bg-gray-900 hover:text-gray-200")}><Network size={14} /> DNS Only</button>
            </div>
            </div>
        )}

        {/* Filters for URLs Tab */}
        {activeTab === 'urls' && (
            <div className="flex items-center gap-3">
              <span className="text-sm font-medium text-gray-400">Filters:</span>
              <div className="flex bg-gray-950 rounded-lg p-1 border border-gray-800 gap-1">
                  {/* 👇 دکمه جدید برای فایل‌های JS */}
                  <button 
                    onClick={() => setFilterJsOnly(!filterJsOnly)} 
                    className={clsx(
                      "px-3 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-2 border", 
                      filterJsOnly 
                        ? "bg-yellow-900/30 text-yellow-400 border-yellow-800" 
                        : "bg-transparent border-transparent text-gray-400 hover:bg-gray-900 hover:text-gray-200"
                    )}
                  >
                    <FileCode size={14} /> JS Files Only
                  </button>
              </div>
            </div>
        )}
      </div>

      {/* Main Content Area */}
      <div className={clsx(
          "bg-gray-900 rounded-lg border border-gray-800 flex flex-col overflow-hidden transition-opacity duration-200 flex-1",
          isFetching ? "opacity-60 pointer-events-none" : "opacity-100"
      )}>
        {activeTab === 'assets' ? (
            // ASSETS TABLE
            <div className="overflow-auto w-full flex-1">
                <table className="w-full text-left min-w-[1400px]"> 
                <thead className="sticky top-0 z-10">
                    <tr className="bg-gray-800/90 backdrop-blur text-gray-400 text-sm uppercase border-b border-gray-800">
                    <SortableHeader field="value" label="Asset / Domain" className="w-[300px]" />
                    <th className="px-6 py-4 font-semibold w-[200px]">DNSX IP</th>
                    <th className="px-6 py-4 font-semibold w-[200px]">HTTPX IP</th>
                    <SortableHeader field="status_code" label="Status" className="w-[100px]" />
                    <SortableHeader field="title" label="Title" className="w-[250px]" />
                    <th className="px-6 py-4 font-semibold min-w-[200px]">Tech Stack</th>
                    <SortableHeader field="content_length" label="Size" className="w-[100px] text-right" />
                    </tr>
                </thead>
                <tbody className="divide-y divide-gray-800">
                    {assetsQuery.data?.data.map((asset) => {
                    const statusCode = asset.status_code ?? 0;
                    let dnsxIps: string[] = [];
                    try { const p = JSON.parse(asset.dnsx_ip || "[]"); dnsxIps = Array.isArray(p) ? p : [p]; } catch { if (asset.dnsx_ip) dnsxIps = [asset.dnsx_ip]; }
                    let httpxIps: string[] = [];
                    try { const p = JSON.parse(asset.host_ip || "[]"); httpxIps = Array.isArray(p) ? p : [p]; } catch { if (asset.host_ip) httpxIps = [asset.host_ip]; }
                    let techs: string[] = [];
                    if (Array.isArray(asset.technologies)) { techs = asset.technologies as string[]; } else if (typeof asset.technologies === 'string') { try { techs = JSON.parse(asset.technologies); } catch {} }

                    return (
                    <tr key={asset.id} className="hover:bg-gray-800/30 transition-colors">
                        <td className="px-6 py-4 align-top">
                        <div className="flex items-start gap-3">
                            <Globe size={18} className={asset.is_live ? "text-blue-500 mt-1" : "text-gray-600 mt-1"} />
                            <div><a href={`http://${asset.value}`} target="_blank" rel="noreferrer" className="font-medium text-gray-200 hover:text-blue-400 transition-colors break-all">{asset.value}</a></div>
                        </div>
                        </td>
                        <td className="px-6 py-4 align-top"><div className="flex flex-col gap-1.5">{dnsxIps.length > 0 && dnsxIps[0] !== "" ? dnsxIps.map((ip, idx) => <span key={idx} className="text-xs font-mono text-cyan-400 bg-cyan-950/30 px-2 py-0.5 rounded border border-cyan-900 w-fit">{ip}</span>) : <span className="text-gray-600 text-xs">-</span>}</div></td>
                        <td className="px-6 py-4 align-top"><div className="flex flex-col gap-1.5">{httpxIps.length > 0 && httpxIps[0] !== "" ? httpxIps.map((ip, idx) => <span key={idx} className="text-xs font-mono text-gray-400 bg-gray-950 px-2 py-0.5 rounded border border-gray-800 w-fit">{ip}</span>) : <span className="text-gray-600 text-xs">-</span>}</div></td>
                        <td className="px-6 py-4 align-top">{asset.is_live ? (statusCode > 0 ? <span className={`px-2 py-1 rounded text-xs font-bold ${statusCode >= 200 && statusCode < 300 ? 'bg-green-900/30 text-green-400 border border-green-800' : statusCode >= 300 && statusCode < 400 ? 'bg-yellow-900/30 text-yellow-400 border border-yellow-800' : 'bg-red-900/30 text-red-400 border border-red-800'}`}>{statusCode}</span> : <span className="text-gray-600 text-xs">-</span>) : <span className="px-2 py-1 rounded text-xs font-bold bg-gray-800 text-gray-500 border border-gray-700">DEAD</span>}</td>
                        <td className="px-6 py-4 align-top"><span className="text-sm text-gray-300 block line-clamp-2" title={asset.title}>{asset.title || '-'}</span></td>
                        <td className="px-6 py-4 align-top"><div className="flex flex-wrap gap-1 max-h-[100px] overflow-y-auto">{asset.web_server && <span className="px-1.5 py-0.5 rounded bg-indigo-900/30 text-indigo-300 text-[10px] border border-indigo-800 whitespace-nowrap">{asset.web_server}</span>}{techs.map((tech, i) => <span key={i} className="px-1.5 py-0.5 rounded bg-purple-900/30 text-purple-300 text-[10px] border border-purple-800 whitespace-nowrap">{tech}</span>)}</div></td>
                        <td className="px-6 py-4 align-top text-right text-sm text-gray-500 font-mono">{asset.content_length ? `${(asset.content_length / 1024).toFixed(1)} KB` : '-'}</td>
                    </tr>
                    )})}
                    {assetsQuery.data?.data.length === 0 && <tr><td colSpan={7} className="px-6 py-12 text-center text-gray-500">No assets found matching criteria.</td></tr>}
                </tbody>
                </table>
            </div>
        ) : (
            // URLS TABLE
            <div className="overflow-auto w-full flex-1">
                <table className="w-full text-left"> 
                <thead className="sticky top-0 z-10">
                    <tr className="bg-gray-800/90 backdrop-blur text-gray-400 text-sm uppercase border-b border-gray-800">
                    <th className="px-6 py-4 font-semibold w-[70%]">URL</th>
                    <th className="px-6 py-4 font-semibold w-[15%]">Source</th>
                    <th className="px-6 py-4 font-semibold w-[15%] text-right">Found At</th>
                    </tr>
                </thead>
                <tbody className="divide-y divide-gray-800">
                    {urlsQuery.data?.data.map((url) => (
                    <tr key={url.id} className="hover:bg-gray-800/30 transition-colors group">
                        <td className="px-6 py-3 align-top">
                            <div className="flex items-start gap-3">
                                {/* آیکون متغیر بر اساس نوع فایل */}
                                {url.value.endsWith('.js') ? (
                                    <FileCode size={16} className="text-yellow-500 mt-1 flex-shrink-0" />
                                ) : (
                                    <FileText size={16} className="text-gray-600 mt-1 group-hover:text-blue-500 transition-colors flex-shrink-0" />
                                )}
                                <a 
                                  href={url.value} 
                                  target="_blank" 
                                  rel="noreferrer" 
                                  className={clsx(
                                      "transition-colors break-all font-mono text-sm block leading-relaxed",
                                      url.value.endsWith('.js') ? "text-yellow-100 hover:text-yellow-300" : "text-gray-300 hover:text-blue-400"
                                  )}
                                >
                                    {url.value}
                                </a>
                            </div>
                        </td>
                        <td className="px-6 py-3 align-top">
                            <span className="px-2 py-0.5 rounded-md bg-gray-800 text-gray-400 text-xs border border-gray-700 font-mono uppercase">
                                {url.source}
                            </span>
                        </td>
                        <td className="px-6 py-3 align-top text-right text-sm text-gray-500 font-mono">
                            {new Date(url.created_at).toLocaleString()}
                        </td>
                    </tr>
                    ))}
                    {urlsQuery.data?.data.length === 0 && (
                      <tr>
                        <td colSpan={3} className="px-6 py-12 text-center text-gray-500">
                          {searchTerm ? 'No URLs found for this search.' : filterJsOnly ? 'No JS files found.' : 'No URLs found yet. Try running the Crawling phase.'}
                        </td>
                      </tr>
                    )}
                </tbody>
                </table>
            </div>
        )}
        
        {/* Footer */}
        <div className="border-t border-gray-800 p-4 flex justify-between items-center bg-gray-900 mt-auto flex-shrink-0">
           <span className="text-sm text-gray-400">
             Page {page} • Total: {currentData?.total_count?.toLocaleString() || 0}
           </span>
           <div className="flex gap-2">
             <button 
                disabled={page === 1 || isFetching} 
                onClick={() => setPage(p => Math.max(1, p - 1))} 
                className="px-3 py-1 rounded border border-gray-700 text-gray-300 text-sm disabled:opacity-50 hover:bg-gray-800 disabled:cursor-not-allowed"
             >
                Previous
             </button>
             <button 
                disabled={!currentData?.data || currentData.data.length < 50 || isFetching} 
                onClick={() => setPage(p => p + 1)} 
                className="px-3 py-1 rounded border border-gray-700 text-gray-300 text-sm disabled:opacity-50 hover:bg-gray-800 disabled:cursor-not-allowed"
             >
                Next
             </button>
           </div>
        </div>
      </div>
    </div>
  );
};

export default TargetAssets;