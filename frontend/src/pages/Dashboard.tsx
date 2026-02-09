import { useQuery } from '@tanstack/react-query';
import { getDashboardStats } from '../api/stats';
import { Target, Activity, Zap, Database, Cpu, Network } from 'lucide-react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';
import MonitoringServer from '../components/MonitoringServer';
import { useAuth } from '../context/AuthContext';

const Dashboard = () => {
  const { role } = useAuth();
  const { data, isLoading, isError } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: getDashboardStats,
    refetchInterval: 10000,
  });

  if (isLoading) return <div className="p-4 md:p-8 text-hack-dim font-mono animate-pulse"> Loading system metrics...</div>;
  
  if (isError || !data) {
      return (
          <div className="p-4 md:p-8 border border-hack-danger text-hack-danger bg-hack-danger/5 font-mono text-sm md:text-base">
             SYSTEM ERROR: Failed to fetch telemetry data.
          </div>
      );
  }

  const stats = data;
  const COLORS = ['#00ff41', '#008F11', '#fcee0a', '#ff003c', '#e0e0e0'];

  const pieData = stats.assets_by_status || [];
  const barData = stats.top_technologies || [];
  const portsData = stats.top_open_ports || [];

  return (
    <div className="space-y-6 md:space-y-8">
      {/* Header */}
      <div className="flex items-center gap-2 border-b border-hack-border/50 pb-4">
        <Cpu className="text-hack-primary" size={20} />
        <h1 className="hack-title text-xl md:text-2xl">COMMAND CENTER</h1>
      </div>

      {/* Stat Cards - Grid is responsive by default (cols-1 -> cols-2 -> cols-4) */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard title="TOTAL TARGETS" value={stats.total_targets || 0} icon={Target} color="text-hack-primary" />
        <StatCard title="TOTAL ASSETS" value={stats.total_assets || 0} icon={Database} color="text-hack-warning" />
        <StatCard title="LIVE NODES" value={stats.live_assets || 0} icon={Activity} color="text-hack-text" />
        <StatCard title="FRESH INTEL (24H)" value={stats.fresh_assets || 0} icon={Zap} color="text-hack-danger" />
      </div>

      {/* Charts Section */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mt-8">
        {/* Pie Chart */}
        <div className="hack-box p-4 md:p-6 relative">
          <div className="absolute top-0 right-0 p-2 text-[8px] md:text-[10px] text-hack-dim">SYS.MON.01</div>
          <h3 className="text-xs md:text-sm font-bold text-hack-primary tracking-widest uppercase mb-6 flex items-center gap-2">
            <span className="w-1 h-4 bg-hack-primary"></span>
            Asset Status Distribution
          </h3>
          <div className="h-[250px] md:h-[300px] w-full">
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
                  stroke="none"
                >
                  {pieData.map((_, index) => (
                    <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip 
                    contentStyle={{ backgroundColor: '#050505', borderColor: '#00ff41', color: '#e0e0e0', fontFamily: 'monospace', fontSize: '12px' }}
                    itemStyle={{ color: '#00ff41' }}
                />
              </PieChart>
            </ResponsiveContainer>
          </div>
           <div className="flex flex-wrap gap-2 md:gap-3 justify-center mt-2">
              {pieData.map((entry, index) => (
                  <div key={entry.name} className="flex items-center gap-2 text-[10px] md:text-xs font-mono text-hack-dim border border-hack-border px-2 py-1 bg-black/40">
                      <div className="w-2 h-2 rounded-none" style={{ backgroundColor: COLORS[index % COLORS.length] }}></div>
                      <span>{entry.name}: <span className="text-white">{entry.count}</span></span>
                  </div>
              ))}
           </div>
        </div>

        {/* Bar Chart */}
        <div className="hack-box p-4 md:p-6 relative">
          <div className="absolute top-0 right-0 p-2 text-[8px] md:text-[10px] text-hack-dim">SYS.MON.02</div>
          <h3 className="text-xs md:text-sm font-bold text-hack-primary tracking-widest uppercase mb-6 flex items-center gap-2">
            <span className="w-1 h-4 bg-hack-primary"></span>
            Top Technologies
          </h3>
          <div className="h-[250px] md:h-[300px] w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart 
                data={barData as any[]} 
                layout="vertical" 
                margin={{ left: 0, right: 10 }}
              >
                <CartesianGrid strokeDasharray="3 3" stroke="#1f1f1f" horizontal={false} />
                <XAxis type="number" stroke="#666" tick={{fill: '#666', fontSize: 10, fontFamily: 'monospace'}} />
                <YAxis dataKey="name" type="category" stroke="#666" width={80} tick={{fill: '#e0e0e0', fontSize: 9, fontFamily: 'monospace'}} />
                <Tooltip 
                    cursor={{fill: 'rgba(0, 255, 65, 0.05)'}}
                    contentStyle={{ backgroundColor: '#050505', borderColor: '#00ff41', color: '#e0e0e0', fontFamily: 'monospace', fontSize: '12px' }}
                />
                <Bar dataKey="count" fill="#00ff41" radius={[0, 2, 2, 0]} barSize={15} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Top Open Ports */}
        <div className="hack-box p-4 md:p-6 relative">
          <div className="absolute top-0 right-0 p-2 text-[8px] md:text-[10px] text-hack-dim">SYS.MON.03</div>
          <h3 className="text-xs md:text-sm font-bold text-hack-primary tracking-widest uppercase mb-6 flex items-center gap-2">
            <span className="w-1 h-4 bg-hack-primary"></span>
            <span className="flex items-center gap-2">
              <Network size={14} className="text-hack-primary" />
              Top Open Ports
            </span>
          </h3>

          <div className="h-[250px] md:h-[300px] w-full">
            {portsData.length === 0 ? (
              <div className="h-full flex items-center justify-center text-hack-dim font-mono text-xs border border-hack-border bg-black/30">
                No portscan telemetry yet.
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart
                  data={portsData as any[]}
                  layout="vertical"
                  margin={{ left: 0, right: 10 }}
                >
                  <CartesianGrid strokeDasharray="3 3" stroke="#1f1f1f" horizontal={false} />
                  <XAxis type="number" stroke="#666" tick={{ fill: '#666', fontSize: 10, fontFamily: 'monospace' }} />
                  <YAxis
                    dataKey="name"
                    type="category"
                    stroke="#666"
                    width={50}
                    tick={{ fill: '#e0e0e0', fontSize: 10, fontFamily: 'monospace' }}
                  />
                  <Tooltip
                    cursor={{ fill: 'rgba(0, 255, 65, 0.05)' }}
                    contentStyle={{ backgroundColor: '#050505', borderColor: '#00ff41', color: '#e0e0e0', fontFamily: 'monospace', fontSize: '12px' }}
                    formatter={(value: any, _name: any, props: any) => [value, `port ${props?.payload?.name}`]}
                  />
                  <Bar dataKey="count" fill="#fcee0a" radius={[0, 2, 2, 0]} barSize={15} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </div>
      </div>

      {/* Monitoring Server Section (Admin Only) */}
      {role === 'admin' && <MonitoringServer />}
    </div>
  );
};

const StatCard = ({ title, value, icon: Icon, color }: any) => (
  <div className="hack-box p-4 md:p-5 flex items-center justify-between group hover:border-hack-primary/50 transition-colors">
    <div>
      <p className="text-hack-dim text-[9px] md:text-[10px] uppercase tracking-[0.2em] mb-1 group-hover:text-hack-primary transition-colors">{title}</p>
      <h2 className="text-3xl md:text-4xl font-display text-white mt-1 drop-shadow-neon">{value.toLocaleString()}</h2>
    </div>
    <div className={`p-2 md:p-3 border border-hack-border bg-black/50 ${color} group-hover:shadow-neon transition-all`}>
      <Icon size={20} className="md:w-6 md:h-6" />
    </div>
  </div>
);

export default Dashboard;