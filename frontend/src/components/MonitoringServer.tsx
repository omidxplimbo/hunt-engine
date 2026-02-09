import { useQuery } from '@tanstack/react-query';
import { getMonitorStats } from '../api/stats';
import { Server, Activity, Cpu, HardDrive, Terminal } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { useState, useEffect } from 'react';

const MonitoringServer = () => {
    const [history, setHistory] = useState<{time: string, cpu: number, ram: number}[]>([]);
    
    const { data } = useQuery({
        queryKey: ['monitor-stats'],
        queryFn: getMonitorStats,
        refetchInterval: 2000, // 2 seconds
        retry: false, // Don't retry if it fails (e.g. 403)
    });

    useEffect(() => {
        if (data) {
            setHistory(prev => {
                const now = new Date().toLocaleTimeString([], { hour12: false });
                const newPoint = {
                    time: now,
                    cpu: data.stats.cpu_usage,
                    ram: data.stats.memory_usage
                };
                const newHistory = [...prev, newPoint];
                if (newHistory.length > 20) newHistory.shift(); // Keep last 20 points
                return newHistory;
            });
        }
    }, [data]);

    if (!data) return null; // Or return a placeholder if desired

    const { stats, processes } = data;

    return (
        <div className="space-y-6 animate-in fade-in duration-500">
             {/* Header */}
            <div className="flex items-center gap-2 border-b border-hack-border/50 pb-4 mt-8">
                <Server className="text-hack-primary" size={20} />
                <h1 className="hack-title text-xl md:text-2xl">MONITORING SERVER</h1>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                {/* CPU Chart */}
                <div className="hack-box p-4 md:p-6 relative col-span-2">
                    <div className="absolute top-0 right-0 p-2 text-[8px] md:text-[10px] text-hack-dim">SYS.CPU.REALTIME</div>
                    <h3 className="text-xs md:text-sm font-bold text-hack-primary tracking-widest uppercase mb-4 flex items-center gap-2">
                        <Cpu size={16} /> SYSTEM LOAD
                    </h3>
                    <div className="h-[200px] w-full">
                        <ResponsiveContainer width="100%" height="100%">
                            <AreaChart data={history}>
                                <CartesianGrid strokeDasharray="3 3" stroke="#333" />
                                <XAxis dataKey="time" hide />
                                <YAxis domain={[0, 100]} stroke="#666" fontSize={10} />
                                <Tooltip 
                                    contentStyle={{ backgroundColor: '#050505', borderColor: '#00ff41', color: '#e0e0e0', fontFamily: 'monospace' }}
                                />
                                <Area type="monotone" dataKey="cpu" stroke="#00ff41" fill="#00ff41" fillOpacity={0.1} name="CPU %" />
                                <Area type="monotone" dataKey="ram" stroke="#fcee0a" fill="#fcee0a" fillOpacity={0.1} name="RAM %" />
                            </AreaChart>
                        </ResponsiveContainer>
                    </div>
                </div>

                {/* Stats Grid */}
                <div className="space-y-4">
                     <div className="hack-box p-4 flex items-center justify-between">
                        <div>
                            <p className="text-hack-dim text-xs">CPU USAGE</p>
                            <p className="text-hack-primary text-xl font-mono">{stats.cpu_usage.toFixed(1)}%</p>
                        </div>
                        <Activity className="text-hack-primary/50" />
                     </div>
                     <div className="hack-box p-4 flex items-center justify-between">
                        <div>
                            <p className="text-hack-dim text-xs">MEMORY USAGE</p>
                            <p className="text-hack-warning text-xl font-mono">{stats.memory_usage.toFixed(1)}%</p>
                            <p className="text-hack-dim text-[10px]">{(stats.memory_used / 1024 / 1024).toFixed(0)}MB / {(stats.memory_total / 1024 / 1024).toFixed(0)}MB</p>
                        </div>
                        <HardDrive className="text-hack-warning/50" />
                     </div>
                     <div className="hack-box p-4 flex items-center justify-between">
                        <div>
                            <p className="text-hack-dim text-xs">GOROUTINES</p>
                            <p className="text-white text-xl font-mono">{stats.num_goroutine}</p>
                        </div>
                        <Terminal className="text-white/50" />
                     </div>
                </div>
            </div>

            {/* Process List */}
            <div className="hack-box p-4 md:p-6 relative">
                 <div className="absolute top-0 right-0 p-2 text-[8px] md:text-[10px] text-hack-dim">SYS.PROC.LIST</div>
                 <h3 className="text-xs md:text-sm font-bold text-hack-primary tracking-widest uppercase mb-4 flex items-center gap-2">
                    <Terminal size={16} /> ACTIVE PROCESSES ({processes.length})
                </h3>
                <div className="overflow-x-auto">
                    <table className="w-full text-left border-collapse">
                        <thead>
                            <tr className="text-hack-dim text-xs border-b border-hack-border/30">
                                <th className="p-2">TARGET</th>
                                <th className="p-2">COMMAND</th>
                                <th className="p-2">PID</th>
                                <th className="p-2">DURATION</th>
                            </tr>
                        </thead>
                        <tbody className="font-mono text-sm">
                            {processes.length === 0 ? (
                                <tr>
                                    <td colSpan={4} className="p-4 text-center text-hack-dim italic">No active background tasks.</td>
                                </tr>
                            ) : (
                                processes.map((proc) => (
                                    <tr key={proc.pid} className="border-b border-hack-border/10 hover:bg-hack-primary/5 transition-colors">
                                        <td className="p-2 text-white">{proc.target_name}</td>
                                        <td className="p-2 text-hack-primary break-all">{proc.command}</td>
                                        <td className="p-2 text-hack-dim">{proc.pid}</td>
                                        <td className="p-2 text-hack-warning">{proc.duration}</td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    );
};

export default MonitoringServer;
