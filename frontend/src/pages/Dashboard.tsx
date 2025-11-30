import { useQuery } from '@tanstack/react-query';
import { getDashboardStats } from '../api/stats';
import { Target, Activity, Zap, Database } from 'lucide-react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';

const Dashboard = () => {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: getDashboardStats,
    refetchInterval: 10000,
  });

  if (isLoading) return <div className="p-8 text-gray-400">Loading dashboard...</div>;
  
  // 👇 مدیریت خطای بهتر
  if (isError || !data) {
      return (
          <div className="p-8 text-red-500 border border-red-900 bg-red-900/10 rounded-lg m-4">
              Error loading stats. Please check if the backend is running.
          </div>
      );
  }

  // 👇 استفاده ایمن از دیتا
  const stats = data;
  const COLORS = ['#0088FE', '#00C49F', '#FFBB28', '#FF8042', '#8884d8'];

  // چک کردن اینکه آیا آرایه‌ها وجود دارند (جلوگیری از کرش map)
  const pieData = stats.assets_by_status || [];
  const barData = stats.top_technologies || [];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white mb-6">Command Center</h1>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard title="Total Targets" value={stats.total_targets || 0} icon={Target} color="text-blue-500" />
        <StatCard title="Total Assets" value={stats.total_assets || 0} icon={Database} color="text-purple-500" />
        <StatCard title="Live Assets" value={stats.live_assets || 0} icon={Activity} color="text-green-500" />
        <StatCard title="Fresh (24h)" value={stats.fresh_assets || 0} icon={Zap} color="text-yellow-500" />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mt-8">
        {/* Pie Chart */}
        <div className="bg-gray-900 p-6 rounded-xl border border-gray-800">
          <h3 className="text-lg font-semibold text-white mb-4">Asset Status Distribution</h3>
          <div className="h-[300px] w-full">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={pieData as any[]}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={80}
                  paddingAngle={5}
                  dataKey="count"
                >
                  {pieData.map((_, index) => (
                    <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip 
                    contentStyle={{ backgroundColor: '#1f2937', borderColor: '#374151', color: '#fff' }}
                    itemStyle={{ color: '#fff' }}
                />
              </PieChart>
            </ResponsiveContainer>
          </div>
           <div className="flex flex-wrap gap-3 justify-center mt-2">
              {pieData.map((entry, index) => (
                  <div key={entry.name} className="flex items-center gap-1 text-xs text-gray-400">
                      <div className="w-2 h-2 rounded-full" style={{ backgroundColor: COLORS[index % COLORS.length] }}></div>
                      <span>{entry.name}: {entry.count}</span>
                  </div>
              ))}
           </div>
        </div>

        {/* Bar Chart */}
        <div className="bg-gray-900 p-6 rounded-xl border border-gray-800">
          <h3 className="text-lg font-semibold text-white mb-4">Top 10 Technologies</h3>
          <div className="h-[300px] w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart 
                data={barData as any[]} 
                layout="vertical" 
                margin={{ left: 20 }}
              >
                <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
                <XAxis type="number" stroke="#9CA3AF" />
                <YAxis dataKey="name" type="category" stroke="#9CA3AF" width={100} />
                <Tooltip 
                    cursor={{fill: '#374151', opacity: 0.4}}
                    contentStyle={{ backgroundColor: '#1f2937', borderColor: '#374151', color: '#fff' }}
                />
                <Bar dataKey="count" fill="#3B82F6" radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    </div>
  );
};

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const StatCard = ({ title, value, icon: Icon, color }: any) => (
  <div className="bg-gray-900 p-6 rounded-xl border border-gray-800 flex items-center justify-between">
    <div>
      <p className="text-gray-400 text-sm font-medium">{title}</p>
      <h2 className="text-3xl font-bold text-white mt-1">{value.toLocaleString()}</h2>
    </div>
    <div className={`p-3 rounded-lg bg-gray-800 ${color}`}>
      <Icon size={24} />
    </div>
  </div>
);

export default Dashboard;