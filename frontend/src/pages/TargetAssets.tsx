import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getTargetAssets, getTargetDetails } from '../api/targets';
import { ArrowLeft, Globe, CheckCircle, XCircle, Search, Monitor } from 'lucide-react';
import clsx from 'clsx';

const TargetAssets = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const targetId = Number(id);

  const [page, setPage] = useState(1);
  const [filterLive, setFilterLive] = useState<boolean | undefined>(undefined);
  // 👇 استیت جدید برای فیلتر HTTPX
  const [filterHttpx, setFilterHttpx] = useState<boolean | undefined>(undefined);
  
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  // Debounce Logic
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchTerm);
      setPage(1);
    }, 500);
    return () => clearTimeout(timer);
  }, [searchTerm]);

  const targetQuery = useQuery({
    queryKey: ['target', targetId],
    queryFn: () => getTargetDetails(targetId),
    enabled: !!targetId,
  });

  const assetsQuery = useQuery({
    // 👇 اضافه شدن filterHttpx به کلید کش
    queryKey: ['assets', targetId, page, filterLive, filterHttpx, debouncedSearch],
    queryFn: () => getTargetAssets(targetId, page, 50, { 
      is_live: filterLive, 
      search: debouncedSearch,
      has_httpx: filterHttpx // 👈 ارسال به API
    }),
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
          </div>
          <p className="text-gray-400 text-sm mt-1">Asset Explorer</p>
        </div>
      </div>

      {/* Filters & Search Toolbar */}
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

        <div className="flex items-center gap-3">
          <span className="text-sm font-medium text-gray-400">Filters:</span>
          <div className="flex bg-gray-950 rounded-lg p-1 border border-gray-800 gap-1">
            
            {/* فیلترهای وضعیت */}
            <div className="flex bg-gray-900 rounded-md p-0.5 border border-gray-800">
                <button onClick={() => setFilterLive(undefined)} className={clsx("px-3 py-1.5 rounded text-sm font-medium transition-colors", filterLive === undefined ? "bg-gray-800 text-white shadow-sm" : "text-gray-400 hover:text-gray-200")}>All</button>
                <button onClick={() => setFilterLive(true)} className={clsx("px-3 py-1.5 rounded text-sm font-medium transition-colors flex items-center gap-1.5", filterLive === true ? "bg-green-900/30 text-green-400 border border-green-800" : "text-gray-400 hover:text-gray-200")}><CheckCircle size={14} /> Live</button>
                <button onClick={() => setFilterLive(false)} className={clsx("px-3 py-1.5 rounded text-sm font-medium transition-colors flex items-center gap-1.5", filterLive === false ? "bg-red-900/30 text-red-400 border border-red-800" : "text-gray-400 hover:text-gray-200")}><XCircle size={14} /> Dead</button>
            </div>

            <div className="w-px bg-gray-800 mx-1 h-6 self-center"></div>

            {/* 👇 فیلتر جدید: Has HTTPX */}
            <button
              onClick={() => setFilterHttpx(prev => !prev ? true : undefined)}
              className={clsx(
                "px-3 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-2 border",
                filterHttpx 
                  ? "bg-blue-900/30 text-blue-400 border-blue-800" 
                  : "bg-transparent border-transparent text-gray-400 hover:bg-gray-900 hover:text-gray-200"
              )}
            >
              <Monitor size={14} /> Has Web
            </button>

          </div>
        </div>
      </div>

      {/* Table Container */}
      <div className="bg-gray-900 rounded-lg border border-gray-800 flex flex-col overflow-hidden">
        {assetsQuery.isLoading ? (
          <div className="p-12 text-center text-gray-500">Loading assets...</div>
        ) : (
          <div className="overflow-x-auto w-full">
            <table className="w-full text-left min-w-[1400px]"> 
              <thead>
                <tr className="bg-gray-800/50 text-gray-400 text-sm uppercase border-b border-gray-800">
                  <th className="px-6 py-4 font-semibold w-[300px]">Asset / Domain</th>
                  <th className="px-6 py-4 font-semibold w-[200px]">DNSX IP (Phase 1)</th>
                  <th className="px-6 py-4 font-semibold w-[200px]">HTTPX IP (Phase 2)</th>
                  <th className="px-6 py-4 font-semibold w-[100px]">Status</th>
                  <th className="px-6 py-4 font-semibold w-[250px]">Title</th>
                  <th className="px-6 py-4 font-semibold min-w-[200px]">Tech Stack</th>
                  <th className="px-6 py-4 font-semibold text-right w-[100px]">Size</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800">
                {assetsQuery.data?.data.map((asset) => {
                  const statusCode = asset.status_code ?? 0;
                  
                  // 1. پارس کردن DNSX IPs
                  let dnsxIps: string[] = [];
                  try {
                     const parsed = JSON.parse(asset.dnsx_ip || "[]");
                     dnsxIps = Array.isArray(parsed) ? parsed : [parsed];
                  } catch {
                     if (asset.dnsx_ip) dnsxIps = [asset.dnsx_ip];
                  }

                  // 2. پارس کردن HTTPX IPs
                  let httpxIps: string[] = [];
                  try {
                     const parsed = JSON.parse(asset.host_ip || "[]");
                     httpxIps = Array.isArray(parsed) ? parsed : [parsed];
                  } catch {
                     if (asset.host_ip) httpxIps = [asset.host_ip];
                  }

                  // 3. پارس کردن Technologies
                  let techs: string[] = [];
                   if (Array.isArray(asset.technologies)) {
                      techs = asset.technologies as string[];
                   } else if (typeof asset.technologies === 'string') {
                      try { techs = JSON.parse(asset.technologies); } catch {}
                   }

                  return (
                  <tr key={asset.id} className="hover:bg-gray-800/30 transition-colors">
                    {/* Col 1: Domain */}
                    <td className="px-6 py-4 align-top">
                      <div className="flex items-start gap-3">
                        <Globe size={18} className={asset.is_live ? "text-blue-500 mt-1" : "text-gray-600 mt-1"} />
                        <div>
                          <a 
                            href={`http://${asset.value}`} 
                            target="_blank" 
                            rel="noreferrer"
                            className="font-medium text-gray-200 hover:text-blue-400 transition-colors break-all"
                          >
                            {asset.value}
                          </a>
                        </div>
                      </div>
                    </td>
                    
                    {/* Col 2: DNSX IPs */}
                    <td className="px-6 py-4 align-top">
                      <div className="flex flex-col gap-1.5">
                        {dnsxIps.length > 0 && dnsxIps[0] !== "" ? (
                          dnsxIps.map((ip, idx) => (
                            <span key={idx} className="text-xs font-mono text-cyan-400 bg-cyan-950/30 px-2 py-0.5 rounded border border-cyan-900 w-fit">
                              {ip}
                            </span>
                          ))
                        ) : (
                          <span className="text-gray-600 text-xs">-</span>
                        )}
                      </div>
                    </td>

                    {/* Col 3: HTTPX IPs */}
                    <td className="px-6 py-4 align-top">
                      <div className="flex flex-col gap-1.5">
                        {httpxIps.length > 0 && httpxIps[0] !== "" ? (
                          httpxIps.map((ip, idx) => (
                            <span key={idx} className="text-xs font-mono text-gray-400 bg-gray-950 px-2 py-0.5 rounded border border-gray-800 w-fit">
                              {ip}
                            </span>
                          ))
                        ) : (
                          <span className="text-gray-600 text-xs">-</span>
                        )}
                      </div>
                    </td>

                    {/* Col 4: Status */}
                    <td className="px-6 py-4 align-top">
                      {asset.is_live ? (
                        statusCode > 0 ? (
                          <div className="flex items-center gap-2">
                            <span className={`px-2 py-1 rounded text-xs font-bold ${
                              statusCode >= 200 && statusCode < 300 ? 'bg-green-900/30 text-green-400 border border-green-800' :
                              statusCode >= 300 && statusCode < 400 ? 'bg-yellow-900/30 text-yellow-400 border border-yellow-800' :
                              'bg-red-900/30 text-red-400 border border-red-800'
                            }`}>
                              {statusCode}
                            </span>
                          </div>
                        ) : (
                          <span className="text-gray-600 text-xs">-</span>
                        )
                      ) : (
                        <span className="px-2 py-1 rounded text-xs font-bold bg-gray-800 text-gray-500 border border-gray-700">
                          DEAD
                        </span>
                      )}
                    </td>

                    {/* Col 5: Title */}
                    <td className="px-6 py-4 align-top">
                      <span className="text-sm text-gray-300 block line-clamp-2" title={asset.title}>
                        {asset.title || '-'}
                      </span>
                    </td>

                    {/* Col 6: Tech Stack */}
                    <td className="px-6 py-4 align-top">
                      <div className="flex flex-wrap gap-1 max-h-[100px] overflow-y-auto">
                        {asset.web_server && (
                          <span className="px-1.5 py-0.5 rounded bg-indigo-900/30 text-indigo-300 text-[10px] border border-indigo-800 whitespace-nowrap">
                            {asset.web_server}
                          </span>
                        )}
                        {techs.map((tech, i) => (
                          <span key={i} className="px-1.5 py-0.5 rounded bg-purple-900/30 text-purple-300 text-[10px] border border-purple-800 whitespace-nowrap">
                            {tech}
                          </span>
                        ))}
                      </div>
                    </td>

                    {/* Col 7: Size */}
                    <td className="px-6 py-4 align-top text-right text-sm text-gray-500 font-mono">
                      {asset.content_length ? `${(asset.content_length / 1024).toFixed(1)} KB` : '-'}
                    </td>
                  </tr>
                )})}
                {assetsQuery.data?.data.length === 0 && (
                  <tr><td colSpan={7} className="px-6 py-12 text-center text-gray-500">No assets found matching "{searchTerm}"</td></tr>
                )}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination Footer */}
        <div className="border-t border-gray-800 p-4 flex justify-between items-center bg-gray-900 mt-auto">
           <span className="text-sm text-gray-400">
             Page {page} • Total: {assetsQuery.data?.total_count?.toLocaleString() || 0}
           </span>
           <div className="flex gap-2">
             <button 
               disabled={page === 1}
               onClick={() => setPage(p => Math.max(1, p - 1))}
               className="px-3 py-1 rounded border border-gray-700 text-gray-300 text-sm disabled:opacity-50 hover:bg-gray-800"
             >
               Previous
             </button>
             <button 
               disabled={!assetsQuery.data?.data || assetsQuery.data.data.length < 50}
               onClick={() => setPage(p => p + 1)}
               className="px-3 py-1 rounded border border-gray-700 text-gray-300 text-sm disabled:opacity-50 hover:bg-gray-800"
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