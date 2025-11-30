export interface User {
  id: number;
  username: string;
  role: string;
  created_at: string;
}

export interface UserResponse {
  status: string;
  data: User[];
}