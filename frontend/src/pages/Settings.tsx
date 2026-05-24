import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Navigate } from "react-router-dom";
import { getUsers, deleteUser, updateUser } from "../api/users";
import { UserModal } from "../components/UserModal";
import { User, Plus, Edit2, Trash2, Power, ListOrdered } from "lucide-react";
import { useAuth } from "../context/AuthContext";
import { ConcurrencyConfig } from "../components/ConcurrencyConfig";

const Settings = () => {
  const { role } = useAuth();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<any>(null);
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ["users"],
    queryFn: getUsers,
  });

  const deleteMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
  });

  const toggleActiveMutation = useMutation({
    mutationFn: ({ id, is_active }: { id: number; is_active: boolean }) =>
      updateUser(id, { is_active }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
  });

  if (role !== "admin") return <Navigate to="/" replace />;

  return (
    <div className="space-y-6">
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <div>
          <h1 className="font-mono text-2xl uppercase tracking-wider text-hack-primary">
            SYSTEM CONFIGURATION
          </h1>
          <p className="text-hack-dim font-mono text-sm">
            Access Control & Parameters
          </p>
        </div>
        <button
          onClick={() => {
            setEditingUser(null);
            setIsModalOpen(true);
          }}
          className="hack-btn w-full md:w-auto flex items-center justify-center gap-2"
        >
          <Plus size={16} /> New User
        </button>
      </div>

      <ConcurrencyConfig />

      <div className="border border-hack-border bg-black/30 p-5">
        <h2 className="mb-4 font-mono text-lg uppercase tracking-wider text-hack-primary">
          User_Privileges_DB
        </h2>

        <div className="overflow-x-auto">
          <table className="w-full text-left font-mono text-sm">
            <thead>
              <tr className="border-b border-hack-border text-hack-dim uppercase text-xs tracking-wider">
                <th className="py-3 pr-4">Identity</th>
                <th className="py-3 pr-4">Clearance Level</th>
                <th className="py-3 pr-4">Scan Slots</th>
                <th className="py-3 pr-4">Status</th>
                <th className="py-3 pr-4">Registration Date</th>
                <th className="py-3 text-right">Override</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <tr>
                  <td colSpan={6} className="py-4 text-hack-dim">
                    DECRYPTING USER DATA...
                  </td>
                </tr>
              ) : (
                data?.data.map((user: any) => (
                  <tr key={user.id} className="border-b border-hack-border/50">
                    <td className="py-3 pr-4 text-white flex items-center gap-2">
                      <User size={16} className="text-hack-primary" />{" "}
                      {user.username}
                    </td>
                    <td className="py-3 pr-4 uppercase text-hack-primary">
                      {user.role}
                    </td>
                    <td className="py-3 pr-4">
                      <span className="inline-flex items-center gap-2 text-hack-dim">
                        <ListOrdered size={14} />{" "}
                        {user.role === "admin"
                          ? "UNLIMITED"
                          : user.max_concurrent_scans || 1}
                      </span>
                    </td>
                    <td className="py-3 pr-4">
                      {user.is_active ? "active" : "deactive"}
                    </td>
                    <td className="py-3 pr-4 text-hack-dim">
                      {new Date(user.created_at).toLocaleDateString()}
                    </td>
                    <td className="py-3 text-right">
                      <div className="inline-flex items-center gap-2">
                        <button
                          onClick={() =>
                            toggleActiveMutation.mutate({
                              id: user.id,
                              is_active: !user.is_active,
                            })
                          }
                          className={`p-2 transition-colors ${user.is_active ? "hover:text-hack-warning" : "hover:text-hack-primary"}`}
                          title={user.is_active ? "DEACTIVATE" : "ACTIVATE"}
                        >
                          <Power size={16} />
                        </button>
                        <button
                          onClick={() => {
                            setEditingUser(user);
                            setIsModalOpen(true);
                          }}
                          className="p-2 hover:text-hack-primary transition-colors"
                          title="MODIFY"
                        >
                          <Edit2 size={16} />
                        </button>
                        <button
                          onClick={() => {
                            if (confirm(">> EXECUTE DELETION PROTOCOL?"))
                              deleteMutation.mutate(user.id);
                          }}
                          className="p-2 hover:text-hack-danger transition-colors"
                          title="TERMINATE"
                        >
                          <Trash2 size={16} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      <UserModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        user={editingUser}
      />
    </div>
  );
};

export default Settings;
