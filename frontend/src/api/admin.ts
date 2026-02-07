import { api } from './client';

export interface AdminUser {
  id: string;
  email: string;
  name: string;
  auth_provider: string;
  is_admin: boolean;
  created_at: string;
  updated_at: string;
}

export interface AdminUsersResponse {
  users: AdminUser[];
  total: number;
  limit: number;
  offset: number;
}

export interface AdminStats {
  total_users: number;
}

export const adminApi = {
  getUsers(limit = 50, offset = 0): Promise<AdminUsersResponse> {
    return api.get('/api/v1/admin/users', { limit, offset });
  },

  getStats(): Promise<AdminStats> {
    return api.get('/api/v1/admin/stats');
  },
};
