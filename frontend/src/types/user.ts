export interface User {
  id: number;
  username: string;
  role: string;
  is_active: boolean;
  created_at: string;
  max_concurrent_scans: number;
}

export interface UserResponse {
  status: string;
  data: User[];
}
