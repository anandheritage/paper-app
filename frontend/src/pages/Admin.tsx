import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Shield, Users, Mail, Calendar, ChevronLeft, ChevronRight, Search } from 'lucide-react';
import { adminApi, type AdminUser } from '../api/admin';

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  return `${months}mo ago`;
}

function AuthProviderBadge({ provider }: { provider: string }) {
  const colors: Record<string, string> = {
    google: 'bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300',
    email: 'bg-green-50 text-green-700 dark:bg-green-950 dark:text-green-300',
  };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium ${colors[provider] || 'bg-surface-100 text-surface-700 dark:bg-surface-800 dark:text-surface-300'}`}>
      {provider === 'google' ? 'Google' : provider === 'email' ? 'Email' : provider}
    </span>
  );
}

export default function Admin() {
  const [page, setPage] = useState(0);
  const [searchFilter, setSearchFilter] = useState('');
  const limit = 25;

  const { data: stats } = useQuery({
    queryKey: ['admin', 'stats'],
    queryFn: () => adminApi.getStats(),
  });

  const { data, isLoading, isError } = useQuery({
    queryKey: ['admin', 'users', page],
    queryFn: () => adminApi.getUsers(limit, page * limit),
  });

  if (isError) {
    return (
      <div className="text-center py-16">
        <Shield className="h-16 w-16 mx-auto text-red-300 dark:text-red-700 mb-4" />
        <h3 className="text-lg font-medium text-surface-900 dark:text-surface-100 mb-1">Access Denied</h3>
        <p className="text-surface-500 dark:text-surface-400">You don't have admin permissions.</p>
      </div>
    );
  }

  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  // Client-side filter
  const filteredUsers = data?.users?.filter((u: AdminUser) => {
    if (!searchFilter) return true;
    const q = searchFilter.toLowerCase();
    return u.email.toLowerCase().includes(q) || u.name.toLowerCase().includes(q);
  }) || [];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2">
            <Shield className="h-6 w-6 text-amber-500" />
            <h1 className="text-2xl font-bold text-surface-900 dark:text-surface-100">Admin Panel</h1>
          </div>
          <p className="text-surface-500 dark:text-surface-400 mt-1">
            Manage users and view platform analytics
          </p>
        </div>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-5">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-primary-50 dark:bg-primary-950">
              <Users className="h-5 w-5 text-primary-600 dark:text-primary-400" />
            </div>
            <div>
              <p className="text-sm text-surface-500 dark:text-surface-400">Total Users</p>
              <p className="text-2xl font-bold text-surface-900 dark:text-surface-100">
                {stats?.total_users ?? '—'}
              </p>
            </div>
          </div>
        </div>
        <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-5">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-blue-50 dark:bg-blue-950">
              <Mail className="h-5 w-5 text-blue-600 dark:text-blue-400" />
            </div>
            <div>
              <p className="text-sm text-surface-500 dark:text-surface-400">Google Sign-ups</p>
              <p className="text-2xl font-bold text-surface-900 dark:text-surface-100">
                {data?.users?.filter((u: AdminUser) => u.auth_provider === 'google').length ?? '—'}
              </p>
            </div>
          </div>
        </div>
        <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-5">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-green-50 dark:bg-green-950">
              <Calendar className="h-5 w-5 text-green-600 dark:text-green-400" />
            </div>
            <div>
              <p className="text-sm text-surface-500 dark:text-surface-400">Newest User</p>
              <p className="text-lg font-semibold text-surface-900 dark:text-surface-100 truncate max-w-[180px]">
                {data?.users?.[0] ? timeAgo(data.users[0].created_at) : '—'}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Search filter */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-surface-400" />
        <input
          type="text"
          value={searchFilter}
          onChange={(e) => setSearchFilter(e.target.value)}
          placeholder="Filter users by name or email..."
          className="w-full pl-10 pr-4 py-2.5 rounded-xl border border-surface-300 dark:border-surface-700 bg-white dark:bg-surface-900 text-surface-900 dark:text-surface-100 placeholder:text-surface-400 focus:outline-none focus:ring-2 focus:ring-primary-500/40 focus:border-primary-500 transition-all text-sm"
        />
      </div>

      {/* Users table */}
      <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center">
            <div className="h-6 w-6 border-2 border-primary-600 border-t-transparent rounded-full animate-spin mx-auto mb-2" />
            <p className="text-sm text-surface-400">Loading users...</p>
          </div>
        ) : (
          <>
            {/* Desktop table */}
            <div className="hidden md:block overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-surface-200 dark:border-surface-800 bg-surface-50 dark:bg-surface-950">
                    <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">User</th>
                    <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">Email</th>
                    <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">Provider</th>
                    <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">Role</th>
                    <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">Signed Up</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-surface-100 dark:divide-surface-800">
                  {filteredUsers.map((user: AdminUser) => (
                    <tr key={user.id} className="hover:bg-surface-50 dark:hover:bg-surface-950/50 transition-colors">
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-3">
                          <div className="w-8 h-8 rounded-full bg-primary-100 dark:bg-primary-900 flex items-center justify-center text-primary-700 dark:text-primary-300 font-medium text-sm flex-shrink-0">
                            {user.name?.charAt(0)?.toUpperCase() || user.email?.charAt(0)?.toUpperCase() || '?'}
                          </div>
                          <span className="text-sm font-medium text-surface-900 dark:text-surface-100 truncate max-w-[150px]">
                            {user.name || '—'}
                          </span>
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <span className="text-sm text-surface-600 dark:text-surface-400">{user.email}</span>
                      </td>
                      <td className="px-4 py-3">
                        <AuthProviderBadge provider={user.auth_provider} />
                      </td>
                      <td className="px-4 py-3">
                        {user.is_admin ? (
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs font-medium bg-amber-50 text-amber-700 dark:bg-amber-950 dark:text-amber-300">
                            <Shield className="h-3 w-3" />
                            Admin
                          </span>
                        ) : (
                          <span className="text-xs text-surface-400">User</span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <div className="text-sm text-surface-600 dark:text-surface-400">{formatDate(user.created_at)}</div>
                        <div className="text-xs text-surface-400">{timeAgo(user.created_at)}</div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Mobile cards */}
            <div className="md:hidden divide-y divide-surface-100 dark:divide-surface-800">
              {filteredUsers.map((user: AdminUser) => (
                <div key={user.id} className="p-4 space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <div className="w-8 h-8 rounded-full bg-primary-100 dark:bg-primary-900 flex items-center justify-center text-primary-700 dark:text-primary-300 font-medium text-sm">
                        {user.name?.charAt(0)?.toUpperCase() || user.email?.charAt(0)?.toUpperCase() || '?'}
                      </div>
                      <div>
                        <p className="text-sm font-medium text-surface-900 dark:text-surface-100">{user.name || '—'}</p>
                        <p className="text-xs text-surface-500">{user.email}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <AuthProviderBadge provider={user.auth_provider} />
                      {user.is_admin && (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs font-medium bg-amber-50 text-amber-700 dark:bg-amber-950 dark:text-amber-300">
                          <Shield className="h-3 w-3" />
                        </span>
                      )}
                    </div>
                  </div>
                  <p className="text-xs text-surface-400">Joined {formatDate(user.created_at)} ({timeAgo(user.created_at)})</p>
                </div>
              ))}
            </div>

            {/* Empty state */}
            {filteredUsers.length === 0 && !isLoading && (
              <div className="p-8 text-center">
                <Users className="h-12 w-12 mx-auto text-surface-300 dark:text-surface-700 mb-2" />
                <p className="text-sm text-surface-500">{searchFilter ? 'No users match your filter' : 'No users yet'}</p>
              </div>
            )}
          </>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-surface-500 dark:text-surface-400">
            Showing {page * limit + 1}–{Math.min((page + 1) * limit, data?.total || 0)} of {data?.total || 0}
          </p>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setPage(Math.max(0, page - 1))}
              disabled={page === 0}
              className="p-2 rounded-lg border border-surface-300 dark:border-surface-700 text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800 disabled:opacity-50 transition-colors"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
            <span className="text-sm text-surface-500 px-2">
              Page {page + 1} of {totalPages}
            </span>
            <button
              onClick={() => setPage(Math.min(totalPages - 1, page + 1))}
              disabled={page >= totalPages - 1}
              className="p-2 rounded-lg border border-surface-300 dark:border-surface-700 text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800 disabled:opacity-50 transition-colors"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
