import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getTargetAssets, getTargetDetails } from '../api/targets';
import { ArrowLeft, Globe, CheckCircle, XCircle, Search, Monitor, Loader2, ArrowUp, ArrowDown } from 'lucide-react';
import clsx from 'clsx';

const TargetAssets = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const targetId = Number(id);

  const [page, setPage] = useState(1);
  const [filterLive, setFilterLive] = useState<boolean | undefined>(undefined);
  const [filterHttpx, setFilterHttpx] = useState<boolean | undefined>(undefined);
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  // 👇 استیت‌های مرتب‌سازی
  const [sortBy, setSortBy] = useState<string>('value');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc');

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchTerm);
      setPage(1);
    }, 500);
    return () => clearTimeout(timer);
  }, [searchTerm]);

  // تابع کمکی برای تغییر مرتب‌سازی
  const handleSort = (field: string) => {
    if (sortBy === field) {
      // اگر روی همون فیلد کلیک کرد، جهت رو عوض کن
      setSortOrder(prev => prev === 'asc' ? 'desc' : 'asc');
    } else {
      // اگر فیلد جدید بود، اون رو ست کن و صعودی
      setSortBy(field);
      setSortOrder('asc');
    }
  };

  // کامپوننت کوچک برای هدر جدول (با قابلیت نمایش آیکون مرتب‌سازی)
  const SortableHeader = ({ field, label, className }: { field: string, label: string, className?: string }) => (
    <th 
      className={clsx("px-6 py-4 font-semibold cursor-pointer hover:text-white transition-colors select-none group", className)}
      onClick={() => handleSort(field)}
    >
      <div className={clsx("flex items-center gap-1", className?.includes("text-right") && "justify-end")}>
        {label}
        {sortBy === field && (
          sortOrder === 'asc' ? <ArrowUp size={14} className="text-blue-400" /> : <ArrowDown size={14} className="text-blue-400" />
        )}
        {/* آیکون مخفی برای راهنمایی */}
        {sortBy !== field && <ArrowUp size={14} className="text-gray-700 opacity-0 group-hover:opacity-100 transition-opacity" />}
      </div>
    </th>
  );

  const targetQuery = useQuery({
    queryKey: ['target', targetId],
    queryFn: () => getTargetDetails(targetId),
    enabled: !!targetId,
  });

  const assetsQuery = useQuery({
    // 👇 اضافه کردن پارامترهای سورت به کلید کوئری
    queryKey: ['assets', targetId, page, filterLive, filterHttpx, debouncedSearch, sortBy, sortOrder],
    queryFn: () => getTargetAssets(
      targetId, page, 50, 
      { is_live: filterLive, search: debouncedSearch, has_httpx: filterHttpx },
      sortBy, sortOrder // 👈 ارسال به تابع API
    ),
    enabled: !!targetId,
    placeholderData: (previousData) => previousData,
  });

  if (!targetId) return <div>Invalid Target ID</div>;

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center gap-4 mb-8 flex-shrink-0">
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
            {assetsQuery.isFetching && <Loader2 size={16} className="animate-spin text-blue-400" />}
          </div>
          <p className="text-gray-400 text-sm mt-1">Asset Explorer</p>
        </div>
      </div>

      {/* Filters */}
      <div className="bg-gray-900 p-4 rounded-lg border border-gray-800 mb-6 flex flex-wrap items-center gap-4 justify-between flex-shrink-0">
        <div className="relative flex-1 min-w-[200px] max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" size={18} />
          <input 
            type="text" 
            placeholder="Search assets..." 
            className="w-full bg-gray-950 border border-gray-800 text-gray-200 pl-10 pr-4 py-2 rounded-md focus:outline-none focus:border-blue-500 transition-colors"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>
        {/* ... (فیلترها مثل قبل) ... */}
        <div className="flex items-center gap-3">
          <span className="text-sm font-medium text-gray-400">Filters:</span>
          <div className="flex bg-gray-950 rounded-lg p-1 border border-gray-800 gap-1">
             <button onClick={() => setFilterLive(undefined)} className={clsx("px-3 py-1.5 rounded text-sm font-medium transition-colors", filterLive === undefined ? "bg-gray-800 text-white" : "text-gray-400 hover:text-gray-200")}>All</button>
             <button onClick={() => setFilterLive(true)} className={clsx("px-3 py-1.5 rounded text-sm font-medium transition-colors flex items-center gap-1.5", filterLive === true ? "bg-green-900/30 text-green-400 border border-green-800" : "text-gray-400 hover:text-gray-200")}><CheckCircle size={14} /> Live</button>
             <button onClick={() => setFilterLive(false)} className={clsx("px-3 py-1.5 rounded text-sm font-medium transition-colors flex items-center gap-1.5", filterLive === false ? "bg-red-900/30 text-red-400 border border-red-800" : "text-gray-400 hover:text-gray-200")}><XCircle size={14} /> Dead</button>
             <div className="w-px bg-gray-800 mx-1 h-6 self-center"></div>
             <button onClick={() => setFilterHttpx(prev => !prev ? true : undefined)} className={clsx("px-3 py-1.5 rounded text-sm font-medium transition-colors flex items-center gap-2 border", filterHttpx ? "bg-blue-900/30 text-blue-400 border-blue-800" : "bg-transparent border-transparent text-gray-400 hover:bg-gray-900 hover:text-gray-200")}><Monitor size={14} /> Has Web</button>
          </div>
        </div>
      </div>

      {/* Table */}
      <div className={clsx("bg-gray-900 rounded-lg border border-gray-800 flex flex-col overflow-hidden transition-opacity duration-200", assetsQuery.isFetching ? "opacity-70" : "opacity-100")}>
        {assetsQuery.isLoading ? (
          <div className="p-12 text-center text-gray-500">Loading assets...</div>
        ) : (
          <div className="overflow-x-auto w-full">
            <table className="w-full text-left min-w-[1400px]">
              <thead>
                <tr className="bg-gray-800/50 text-gray-400 text-sm uppercase border-b border-gray-800">
                  {/* 👇 استفاده از کامپوننت هدر قابل مرتب‌سازی */}
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
                   // ... (لاجیک رندر ردیف مثل قبل) ...
                   const statusCode = asset.status_code ?? 0;
                   let dnsxIps: string[] = []; try { const p = JSON.parse(asset.dnsx_ip||"[]"); dnsxIps = Array.isArray(p)?p:[p]; } catch { if(asset.dnsx_ip) dnsxIps=[asset.dnsx_ip] }
                   let httpxIps: string[] = []; try { const p = JSON.parse(asset.host_ip||"[]"); httpxIps = Array.isArray(p)?p:[p]; } catch { if(asset.host_ip) httpxIps=[asset.host_ip] }
                   let techs: string[] = []; if(Array.isArray(asset.technologies)) techs=asset.technologies as string[]; else if(typeof asset.technologies==='string') try{techs=JSON.parse(asset.technologies)}catch{}

                   return (
                    <tr key={asset.id} className="hover:bg-gray-800/30 transition-colors">
                        <td className="px-6 py-4 align-top">
                            <div className="flex items-start gap-3">
                                <Globe size={18} className={asset.is_live ? "text-blue-500 mt-1" : "text-gray-600 mt-1"} />
                                <a href={`http://${asset.value}`} target="_blank" rel="noreferrer" className="font-medium text-gray-200 hover:text-blue-400 transition-colors break-all">{asset.value}</a>
                            </div>
                        </td>
                        <td className="px-6 py-4 align-top"><div className="flex flex-col gap-1.5">{dnsxIps.length>0&&dnsxIps[0]!==""?dnsxIps.map((ip,i)=><span key={i} className="text-xs font-mono text-cyan-400 bg-cyan-950/30 px-2 py-0.5 rounded border border-cyan-900 w-fit">{ip}</span>):<span className="text-gray-600 text-xs">-</span>}</div></td>
                        <td className="px-6 py-4 align-top"><div className="flex flex-col gap-1.5">{httpxIps.length>0&&httpxIps[0]!==""?httpxIps.map((ip,i)=><span key={i} className="text-xs font-mono text-gray-400 bg-gray-950 px-2 py-0.5 rounded border border-gray-800 w-fit">{ip}</span>):<span className="text-gray-600 text-xs">-</span>}</div></td>
                        <td className="px-6 py-4 align-top">
                            {asset.is_live ? (statusCode>0 ? <span className={`px-2 py-1 rounded text-xs font-bold ${statusCode>=200&&statusCode<300?'bg-green-900/30 text-green-400 border border-green-800':statusCode>=300&&statusCode<400?'bg-yellow-900/30 text-yellow-400 border border-yellow-800':'bg-red-900/30 text-red-400 border border-red-800'}`}>{statusCode}</span> : <span className="text-gray-600 text-xs">-</span>) : <span className="px-2 py-1 rounded text-xs font-bold bg-gray-800 text-gray-500 border border-gray-700">DEAD</span>}
                        </td>
                        <td className="px-6 py-4 align-top"><span className="text-sm text-gray-300 block line-clamp-2" title={asset.title}>{asset.title||'-'}</span></td>
                        <td className="px-6 py-4 align-top">
                            <div className="flex flex-wrap gap-1 max-h-[100px] overflow-y-auto">
                                {asset.web_server && <span className="px-1.5 py-0.5 rounded bg-indigo-900/30 text-indigo-300 text-[10px] border border-indigo-800 whitespace-nowrap">{asset.web_server}</span>}
                                {techs.map((t,i)=><span key={i} className="px-1.5 py-0.5 rounded bg-purple-900/30 text-purple-300 text-[10px] border border-purple-800 whitespace-nowrap">{t}</span>)}
                            </div>
                        </td>
                        <td className="px-6 py-4 align-top text-right text-sm text-gray-500 font-mono">{asset.content_length?`${(asset.content_length/1024).toFixed(1)} KB`:'-'}</td>
                    </tr>
                   );
                })}
                {assetsQuery.data?.data.length === 0 && (
                  <tr><td colSpan={7} className="px-6 py-12 text-center text-gray-500">No assets found matching criteria.</td></tr>
                )}
              </tbody>
            </table>
          </div>
        )}
        
        {/* Pagination Footer (مثل قبل) */}
        <div className="border-t border-gray-800 p-4 flex justify-between items-center bg-gray-900 mt-auto flex-shrink-0">
           <span className="text-sm text-gray-400">Page {page} • Total: {assetsQuery.data?.total_count?.toLocaleString() || 0}</span>
           <div className="flex gap-2">
             <button disabled={page===1} onClick={()=>setPage(p=>Math.max(1,p-1))} className="px-3 py-1 rounded border border-gray-700 text-gray-300 text-sm disabled:opacity-50 hover:bg-gray-800">Previous</button>
             <button disabled={!assetsQuery.data?.data || assetsQuery.data.data.length<50} onClick={()=>setPage(p=>p+1)} className="px-3 py-1 rounded border border-gray-700 text-gray-300 text-sm disabled:opacity-50 hover:bg-gray-800">Next</button>
           </div>
        </div>
      </div>
    </div>
  );
};

export default TargetAssets;