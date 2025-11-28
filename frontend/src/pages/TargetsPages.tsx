import { useQuery } from '@tanstack/react-query';
// دقت کن که تابع getTargets رو ایمپورت کنی، نه چیز دیگه‌ای
import { getTargets } from '../api/targets'; 
import { Plus, Search, Globe, Clock, Database } from 'lucide-react';

const Targets = () => {
  // استفاده از React Query برای گرفتن دیتا
  const { data, isLoading, isError } = useQuery({
    queryKey: ['targets'],
    queryFn: () => getTargets(1, 50), // فعلا صفحه ۱ رو میگیریم
  });

  if (isLoading) return <div className="p-8 text-gray-400">Loading targets...</div>;
  if (isError) return <div className="p-8 text-red-500">Error loading targets! Check API connection.</div>;

  return (
    <div>
      {/* Header Section */}
      <div className="flex justify-between items-center mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white">Targets</h1>
          <p className="text-gray-400 mt-1">Manage your scope and hunting objectives</p>
        </div>
        <button className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg flex items-center gap-2 transition-colors font-medium">
          <Plus size={18} />
          Add Target
        </button>
      </div>

      {/* Search & Filter Bar (فعلا دکوری) */}
      <div className="bg-gray-900 p-4 rounded-lg border border-gray-800 mb-6 flex gap-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" size={18} />
          <input 
            type="text" 
            placeholder="Search targets..." 
            className="w-full bg-gray-950 border border-gray-800 text-gray-200 pl-10 pr-4 py-2 rounded-md focus:outline-none focus:border-blue-500"
          />
        </div>
      </div>

      {/* Data Table */}
      <div className="bg-gray-900 rounded-lg border border-gray-800 overflow-hidden">
        <table className="w-full text-left">
          <thead>
            <tr className="bg-gray-800/50 text-gray-400 text-sm uppercase">
              <th className="px-6 py-4 font-semibold">Target Name</th>
              <th className="px-6 py-4 font-semibold">Assets Found</th>
              <th className="px-6 py-4 font-semibold">Last Scan</th>
              <th className="px-6 py-4 font-semibold">Status</th>
              <th className="px-6 py-4 font-semibold text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {data?.data.map((target) => (
              <tr key={target.id} className="hover:bg-gray-800/30 transition-colors">
                <td className="px-6 py-4">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-lg bg-gray-800 flex items-center justify-center text-blue-500">
                      <Globe size={20} />
                    </div>
                    <div>
                      <div className="font-medium text-white">{target.name}</div>
                      <div className="text-sm text-gray-500">{target.root_domain}</div>
                    </div>
                  </div>
                </td>
                <td className="px-6 py-4">
                  <div className="flex items-center gap-2 text-gray-300">
                    <Database size={16} className="text-gray-500" />
                    <span className="font-mono text-lg">{target.asset_count.toLocaleString()}</span>
                  </div>
                </td>
                <td className="px-6 py-4">
                  <div className="flex items-center gap-2 text-gray-400 text-sm">
                    <Clock size={16} />
                    {target.last_scan_at 
                      ? new Date(target.last_scan_at).toLocaleDateString() 
                      : 'Never'}
                  </div>
                </td>
                <td className="px-6 py-4">
                  <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                    target.in_scope 
                      ? 'bg-green-900/30 text-green-400 border border-green-900' 
                      : 'bg-gray-800 text-gray-400 border border-gray-700'
                  }`}>
                    {target.in_scope ? 'Active' : 'Paused'}
                  </span>
                </td>
                <td className="px-6 py-4 text-right">
                  <button className="text-blue-400 hover:text-blue-300 text-sm font-medium">View Assets</button>
                </td>
              </tr>
            ))}

            {data?.data.length === 0 && (
              <tr>
                <td colSpan={5} className="px-6 py-12 text-center text-gray-500">
                  No targets found. Create your first target to start hunting!
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default Targets;