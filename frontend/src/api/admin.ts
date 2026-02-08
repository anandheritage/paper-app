import { api } from './client';

export interface AdminUser {
  id: string;
  email: string;
  name: string;
  auth_provider: string;
  is_admin: boolean;
  last_login_at?: string;
  created_at: string;
  updated_at: string;
}

export interface AdminUsersResponse {
  users: AdminUser[];
  total: number;
  limit: number;
  offset: number;
}

export interface DailyCount {
  date: string;
  count: number;
}

export interface AdminStats {
  total_users: number;
  active_today: number;
  active_this_week: number;
  active_this_month: number;
  total_logins: number;
  logins_by_method: Record<string, number>;
  total_papers_read: number;
  total_bookmarks: number;
  total_saved: number;
  daily_logins: DailyCount[];
  new_users_today: number;
  new_users_this_week: number;
}

export interface LoginEvent {
  id: string;
  user_id: string;
  auth_method: string;
  ip_address: string;
  user_agent: string;
  created_at: string;
  user_email?: string;
  user_name?: string;
}

export interface AdminActivityResponse {
  events: LoginEvent[];
  total: number;
  limit: number;
  offset: number;
}

export const adminApi = {
  getUsers(limit = 50, offset = 0): Promise<AdminUsersResponse> {
    return api.get('/api/v1/admin/users', { limit, offset });
  },

  getStats(): Promise<AdminStats> {
    return api.get('/api/v1/admin/stats');
  },

  getActivity(limit = 50, offset = 0): Promise<AdminActivityResponse> {
    return api.get('/api/v1/admin/activity', { limit, offset });
  },

  getUserActivity(userId: string, limit = 20): Promise<{ events: LoginEvent[] }> {
    return api.get(`/api/v1/admin/users/${userId}/activity`, { limit });
  },
};
