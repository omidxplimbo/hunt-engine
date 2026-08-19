import { useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Key, Plus, Trash2, Shield, Cookie, FileText, Lock } from "lucide-react";
import { toast } from "react-hot-toast";

// API Calls
const API_BASE = "/api";

interface AuthContext {
  id: number;
  target_id: number;
  user_id: number;
  name: string;
  context_type: "cookie" | "header" | "token" | "session";
  key_name: string;
  value: string;
  domain?: string;
  path?: string;
  description?: string;
  is_active: boolean;
  created_at: string;
}

const fetchAuthContexts = async (targetId: number): Promise<AuthContext[]> => {
  const res = await fetch(`${API_BASE}/targets/${targetId}/auth-contexts`, {
    headers: {
      "Authorization": `Bearer ${localStorage.getItem("token")}`,
    },
  });
  if (!res.ok) throw new Error("Failed to fetch auth contexts");
  const data = await res.json();
  return data.data || [];
};

const createAuthContext = async (data: Partial<AuthContext>): Promise<AuthContext> => {
  const res = await fetch(`${API_BASE}/auth-contexts`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": `Bearer ${localStorage.getItem("token")}`,
    },
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error("Failed to create auth context");
  const result = await res.json();
  return result.data;
};

const deleteAuthContext = async (id: number): Promise<void> => {
  const res = await fetch(`${API_BASE}/auth-contexts/${id}`, {
    method: "DELETE",
    headers: {
      "Authorization": `Bearer ${localStorage.getItem("token")}`,
    },
  });
  if (!res.ok) throw new Error("Failed to delete auth context");
};

// Main Component
export default function AuthContextManager() {
  const { id: targetId } = useParams<{ id: string }>();
  const queryClient = useQueryClient();
  const [showAddForm, setShowAddForm] = useState(false);
  const [newContext, setNewContext] = useState({
    name: "",
    context_type: "cookie" as const,
    key_name: "",
    value: "",
    description: "",
  });

  const { data: contexts, isLoading } = useQuery({
    queryKey: ["authContexts", targetId],
    queryFn: () => fetchAuthContexts(Number(targetId)),
    enabled: !!targetId,
  });

  const createMutation = useMutation({
    mutationFn: createAuthContext,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["authContexts", targetId] });
      setShowAddForm(false);
      setNewContext({ name: "", context_type: "cookie", key_name: "", value: "", description: "" });
      toast.success("Auth Context added successfully");
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const deleteMutation = useMutation({
    mutationFn: deleteAuthContext,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["authContexts", targetId] });
      toast.success("Auth Context deleted");
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!targetId) return;
    createMutation.mutate({
      ...newContext,
      target_id: Number(targetId),
      path: "/",
    });
  };

  const getContextIcon = (type: string) => {
    switch (type) {
      case "cookie": return <Cookie className="w-4 h-4 text-blue-400" />;
      case "header": return <FileText className="w-4 h-4 text-green-400" />;
      case "token": return <Lock className="w-4 h-4 text-purple-400" />;
      default: return <Key className="w-4 h-4 text-gray-400" />;
    }
  };

  return (
    <div className="bg-[#0a0a0a] border border-[#333] rounded-lg p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-white flex items-center gap-2">
          <Shield className="w-5 h-5 text-green-400" />
          Authenticated Contexts (IDOR/BOLA Testing)
        </h3>
        <button
          onClick={() => setShowAddForm(!showAddForm)}
          className="px-3 py-1 bg-green-600 hover:bg-green-700 text-white text-sm rounded flex items-center gap-1"
        >
          <Plus className="w-4 h-4" /> Add Context
        </button>
      </div>

      {showAddForm && (
        <form onSubmit={handleSubmit} className="mb-4 p-3 bg-[#111] border border-[#333] rounded space-y-2">
          <input
            type="text"
            placeholder="Name (e.g., Admin Session)"
            value={newContext.name}
            onChange={(e) => setNewContext({ ...newContext, name: e.target.value })}
            className="w-full bg-[#0a0a0a] border border-[#333] text-white px-2 py-1 rounded text-sm"
            required
          />
          <select
            value={newContext.context_type}
            onChange={(e) => setNewContext({ ...newContext, context_type: e.target.value as any })}
            className="w-full bg-[#0a0a0a] border border-[#333] text-white px-2 py-1 rounded text-sm"
          >
            <option value="cookie">Cookie</option>
            <option value="header">Header</option>
            <option value="token">Token</option>
            <option value="session">Session</option>
          </select>
          <input
            type="text"
            placeholder="Key Name (e.g., sessionid, Authorization)"
            value={newContext.key_name}
            onChange={(e) => setNewContext({ ...newContext, key_name: e.target.value })}
            className="w-full bg-[#0a0a0a] border border-[#333] text-white px-2 py-1 rounded text-sm"
            required
          />
          <textarea
            placeholder="Value (Cookie value, Token, etc.)"
            value={newContext.value}
            onChange={(e) => setNewContext({ ...newContext, value: e.target.value })}
            className="w-full bg-[#0a0a0a] border border-[#333] text-white px-2 py-1 rounded text-sm font-mono"
            rows={2}
            required
          />
          <textarea
            placeholder="Description (optional)"
            value={newContext.description}
            onChange={(e) => setNewContext({ ...newContext, description: e.target.value })}
            className="w-full bg-[#0a0a0a] border border-[#333] text-white px-2 py-1 rounded text-sm"
            rows={1}
          />
          <div className="flex gap-2">
            <button type="submit" className="px-3 py-1 bg-green-600 hover:bg-green-700 text-white text-sm rounded">
              Save
            </button>
            <button
              type="button"
              onClick={() => setShowAddForm(false)}
              className="px-3 py-1 bg-gray-600 hover:bg-gray-700 text-white text-sm rounded"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {isLoading ? (
        <p className="text-gray-400 text-sm">Loading contexts...</p>
      ) : contexts && contexts.length > 0 ? (
        <div className="space-y-2">
          {contexts.map((ctx) => (
            <div key={ctx.id} className="flex items-center justify-between p-2 bg-[#111] border border-[#333] rounded">
              <div className="flex items-center gap-2">
                {getContextIcon(ctx.context_type)}
                <div>
                  <p className="text-white text-sm font-medium">{ctx.name}</p>
                  <p className="text-gray-400 text-xs font-mono">
                    {ctx.key_name}: {ctx.value.substring(0, 20)}{ctx.value.length > 20 ? "..." : ""}
                  </p>
                </div>
              </div>
              <button
                onClick={() => deleteMutation.mutate(ctx.id)}
                className="p-1 hover:bg-red-900/30 rounded text-red-400"
                title="Delete"
              >
                <Trash2 className="w-4 h-4" />
              </button>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-gray-500 text-sm text-center py-4">
          No authentication contexts configured. Add one to enable IDOR/BOLA testing.
        </p>
      )}
    </div>
  );
}
