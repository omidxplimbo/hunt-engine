export interface User {
  id: number;
  username: string;
  role: string;
  is_active: boolean;
  created_at: string;
}

export interface UserResponse {
  status: string;
  data: User[];
}