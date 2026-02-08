import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Shield, Users, Mail, Calendar, ChevronLeft, ChevronRight, Search,
  Activity, Globe, Monitor, TrendingUp, UserCheck,
} from 'lucide-react';
import { adminApi, type AdminUser, type LoginEvent } from '../api/admin';

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
    email_register: 'bg-purple-50 text-purple-700 dark:bg-purple-950 dark:text-purple-300',
    token_refresh: 'bg-surface-100 text-surface-600 dark:bg-surface-800 dark:text-surface-400',
  };
  const labels: Record<string, string> = {
    google: 'Google',
    email: 'Email',
    email_register: 'Register',
    token_refresh: 'Refresh',
  };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium ${colors[provider] || 'bg-surface-100 text-surface-700 dark:bg-surface-800 dark:text-surface-300'}`}>
      {labels[provider] || provider}
    </span>
  );
}

function truncateUA(ua: string, max = 60): string {
  if (!ua) return '—';
  // Extract browser/device info
  const match = ua.match(/(Chrome|Firefox|Safari|Edge|Mobile|iPhone|Android|iPad)[/\s]?[\d.]*/i);
  if (match) return match[0];
  return ua.length > max ? ua.slice(0, max) + '…' : ua;
}

type Tab = 'users' | 'activity';

export default function Admin() {
  const [tab, setTab] = useState<Tab>('users');
  const [userPage, setUserPage] = useState(0);
  const [activityPage, setActivityPage] = useState(0);
  const [searchFilter, setSearchFilter] = useState('');
  const limit = 25;

  const { data: stats } = useQuery({
    queryKey: ['admin', 'stats'],
    queryFn: () => adminApi.getStats(),
    refetchInterval: 30_000,
  });

  const { data: usersData, isLoading: loadingUsers } = useQuery({
    queryKey: ['admin', 'users', userPage],
    queryFn: () => adminApi.getUsers(limit, userPage * limit),
  });

  const { data: activityData, isLoading: loadingActivity } = useQuery({
    queryKey: ['admin', 'activity', activityPage],
    queryFn: () => adminApi.getActivity(limit, activityPage * limit),
    enabled: tab === 'activity',
    refetchInterval: 15_000,
  });

  const totalUserPages = usersData ? Math.ceil(usersData.total / limit) : 0;
  const totalActivityPages = activityData ? Math.ceil(activityData.total / limit) : 0;

  const filteredUsers = usersData?.users?.filter((u: AdminUser) => {
    if (!searchFilter) return true;
    const q = searchFilter.toLowerCase();
    return u.email.toLowerCase().includes(q) || u.name?.toLowerCase().includes(q);
  }) || [];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="flex items-center gap-2">
          <Shield className="h-6 w-6 text-amber-500" />
          <h1 className="text-2xl font-bold text-surface-900 dark:text-surface-100">Admin Panel</h1>
        </div>
        <p className="text-surface-500 dark:text-surface-400 mt-1">Monitor usage and manage users</p>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <StatCard
          icon={<Users className="h-5 w-5 text-primary-600 dark:text-primary-400" />}
          bgColor="bg-primary-50 dark:bg-primary-950"
          label="Total Users"
          value={stats?.total_users ?? '—'}
        />
        <StatCard
          icon={<UserCheck className="h-5 w-5 text-green-600 dark:text-green-400" />}
          bgColor="bg-green-50 dark:bg-green-950"
          label="Active Today"
          value={stats?.active_today ?? '—'}
        />
        <StatCard
          icon={<TrendingUp className="h-5 w-5 text-blue-600 dark:text-blue-400" />}
          bgColor="bg-blue-50 dark:bg-blue-950"
          label="Active This Week"
          value={stats?.active_this_week ?? '—'}
        />
        <StatCard
          icon={<Activity className="h-5 w-5 text-amber-600 dark:text-amber-400" />}
          bgColor="bg-amber-50 dark:bg-amber-950"
          label="Logins (30d)"
          value={stats?.total_logins ?? '—'}
        />
      </div>

      {/* Login method breakdown */}
      {stats?.logins_by_method && Object.keys(stats.logins_by_method).length > 0 && (
        <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-5">
          <h3 className="text-sm font-semibold text-surface-700 dark:text-surface-300 mb-3">Login Methods (30 days)</h3>
          <div className="flex flex-wrap gap-4">
            {Object.entries(stats.logins_by_method).map(([method, count]) => (
              <div key={method} className="flex items-center gap-2">
                <AuthProviderBadge provider={method} />
                <span className="text-sm font-medium text-surface-900 dark:text-surface-100">{count}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Daily login chart (simple bar) */}
      {stats?.daily_logins && stats.daily_logins.length > 0 && (
        <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-5">
          <h3 className="text-sm font-semibold text-surface-700 dark:text-surface-300 mb-3">Daily Logins (Last 30 Days)</h3>
          <div className="flex items-end gap-1 h-24">
            {stats.daily_logins.map((d) => {
              const max = Math.max(...stats.daily_logins.map((x) => x.count), 1);
              const height = Math.max((d.count / max) * 100, 4);
              return (
                <div key={d.date} className="flex-1 flex flex-col items-center group relative">
                  <div
                    className="w-full bg-primary-500 dark:bg-primary-400 rounded-t transition-all hover:bg-primary-600"
                    style={{ height: `${height}%` }}
                    title={`${d.date}: ${d.count} logins`}
                  />
                  <div className="absolute -top-8 left-1/2 -translate-x-1/2 hidden group-hover:block bg-surface-900 dark:bg-surface-100 text-white dark:text-surface-900 text-xs px-2 py-1 rounded whitespace-nowrap z-10">
                    {d.date}: {d.count}
                  </div>
                </div>
              );
            })}
          </div>
          <div className="flex justify-between mt-1">
            <span className="text-[10px] text-surface-400">{stats.daily_logins[0]?.date}</span>
            <span className="text-[10px] text-surface-400">{stats.daily_logins[stats.daily_logins.length - 1]?.date}</span>
          </div>
        </div>
      )}

      {/* Tab navigation */}
      <div className="flex gap-1 bg-surface-100 dark:bg-surface-800 p-1 rounded-lg w-fit">
        <button
          onClick={() => setTab('users')}
          className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
            tab === 'users'
              ? 'bg-white dark:bg-surface-900 text-surface-900 dark:text-surface-100 shadow-sm'
              : 'text-surface-500 hover:text-surface-700 dark:hover:text-surface-300'
          }`}
        >
          <Users className="h-4 w-4" />
          Users
        </button>
        <button
          onClick={() => setTab('activity')}
          className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
            tab === 'activity'
              ? 'bg-white dark:bg-surface-900 text-surface-900 dark:text-surface-100 shadow-sm'
              : 'text-surface-500 hover:text-surface-700 dark:hover:text-surface-300'
          }`}
        >
          <Activity className="h-4 w-4" />
          Login Activity
        </button>
      </div>

      {/* ───── Users Tab ───── */}
      {tab === 'users' && (
        <>
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

          <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 overflow-hidden">
            {loadingUsers ? (
              <LoadingSpinner text="Loading users..." />
            ) : (
              <>
                <div className="hidden md:block overflow-x-auto">
                  <table className="w-full">
                    <thead>
                      <tr className="border-b border-surface-200 dark:border-surface-800 bg-surface-50 dark:bg-surface-950">
                        <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">User</th>
                        <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">Email</th>
                        <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">Provider</th>
                        <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">Last Login</th>
                        <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">Signed Up</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-surface-100 dark:divide-surface-800">
                      {filteredUsers.map((user: AdminUser) => (
                        <tr key={user.id} className="hover:bg-surface-50 dark:hover:bg-surface-950/50 transition-colors">
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-3">
                              <Avatar name={user.name} email={user.email} />
                              <div>
                                <span className="text-sm font-medium text-surface-900 dark:text-surface-100 truncate max-w-[150px] block">
                                  {user.name || '—'}
                                </span>
                                {user.is_admin && (
                                  <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-amber-50 text-amber-700 dark:bg-amber-950 dark:text-amber-300">
                                    <Shield className="h-2.5 w-2.5" />Admin
                                  </span>
                                )}
                              </div>
                            </div>
                          </td>
                          <td className="px-4 py-3">
                            <span className="text-sm text-surface-600 dark:text-surface-400">{user.email}</span>
                          </td>
                          <td className="px-4 py-3">
                            <AuthProviderBadge provider={user.auth_provider} />
                          </td>
                          <td className="px-4 py-3">
                            {user.last_login_at ? (
                              <div>
                                <div className="text-sm text-surface-600 dark:text-surface-400">{timeAgo(user.last_login_at)}</div>
                              </div>
                            ) : (
                              <span className="text-xs text-surface-400">Never</span>
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
                          <Avatar name={user.name} email={user.email} />
                          <div>
                            <p className="text-sm font-medium text-surface-900 dark:text-surface-100">{user.name || '—'}</p>
                            <p className="text-xs text-surface-500">{user.email}</p>
                          </div>
                        </div>
                        <AuthProviderBadge provider={user.auth_provider} />
                      </div>
                      <div className="flex justify-between text-xs text-surface-400">
                        <span>Joined {timeAgo(user.created_at)}</span>
                        <span>Last login: {user.last_login_at ? timeAgo(user.last_login_at) : 'Never'}</span>
                      </div>
                    </div>
                  ))}
                </div>

                {filteredUsers.length === 0 && !loadingUsers && (
                  <EmptyState icon={<Users className="h-12 w-12" />} text={searchFilter ? 'No users match your filter' : 'No users yet'} />
                )}
              </>
            )}
          </div>

          <Pagination page={userPage} totalPages={totalUserPages} total={usersData?.total ?? 0} limit={limit} onPageChange={setUserPage} />
        </>
      )}

      {/* ───── Activity Tab ───── */}
      {tab === 'activity' && (
        <>
          <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 overflow-hidden">
            {loadingActivity ? (
              <LoadingSpinner text="Loading activity..." />
            ) : (
              <>
                <div className="hidden md:block overflow-x-auto">
                  <table className="w-full">
                    <thead>
                      <tr className="border-b border-surface-200 dark:border-surface-800 bg-surface-50 dark:bg-surface-950">
                        <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">User</th>
                        <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">Method</th>
                        <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">IP Address</th>
                        <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">Device</th>
                        <th className="text-left px-4 py-3 text-xs font-semibold text-surface-500 dark:text-surface-400 uppercase tracking-wider">Time</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-surface-100 dark:divide-surface-800">
                      {(activityData?.events ?? []).map((event: LoginEvent) => (
                        <tr key={event.id} className="hover:bg-surface-50 dark:hover:bg-surface-950/50 transition-colors">
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-2">
                              <Avatar name={event.user_name} email={event.user_email} />
                              <div>
                                <p className="text-sm font-medium text-surface-900 dark:text-surface-100">{event.user_name || '—'}</p>
                                <p className="text-xs text-surface-500">{event.user_email}</p>
                              </div>
                            </div>
                          </td>
                          <td className="px-4 py-3">
                            <AuthProviderBadge provider={event.auth_method} />
                          </td>
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-1.5 text-sm text-surface-600 dark:text-surface-400">
                              <Globe className="h-3.5 w-3.5 text-surface-400" />
                              {event.ip_address || '—'}
                            </div>
                          </td>
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-1.5 text-sm text-surface-600 dark:text-surface-400">
                              <Monitor className="h-3.5 w-3.5 text-surface-400" />
                              {truncateUA(event.user_agent)}
                            </div>
                          </td>
                          <td className="px-4 py-3">
                            <div className="text-sm text-surface-600 dark:text-surface-400">{timeAgo(event.created_at)}</div>
                            <div className="text-xs text-surface-400">{formatDate(event.created_at)}</div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {/* Mobile cards */}
                <div className="md:hidden divide-y divide-surface-100 dark:divide-surface-800">
                  {(activityData?.events ?? []).map((event: LoginEvent) => (
                    <div key={event.id} className="p-4 space-y-2">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <Avatar name={event.user_name} email={event.user_email} />
                          <div>
                            <p className="text-sm font-medium text-surface-900 dark:text-surface-100">{event.user_name || event.user_email || '—'}</p>
                            <p className="text-xs text-surface-500">{timeAgo(event.created_at)}</p>
                          </div>
                        </div>
                        <AuthProviderBadge provider={event.auth_method} />
                      </div>
                      <div className="flex gap-4 text-xs text-surface-400">
                        <span className="flex items-center gap-1"><Globe className="h-3 w-3" />{event.ip_address || '—'}</span>
                        <span className="flex items-center gap-1"><Monitor className="h-3 w-3" />{truncateUA(event.user_agent, 30)}</span>
                      </div>
                    </div>
                  ))}
                </div>

                {(activityData?.events ?? []).length === 0 && !loadingActivity && (
                  <EmptyState icon={<Activity className="h-12 w-12" />} text="No login activity recorded yet" />
                )}
              </>
            )}
          </div>

          <Pagination page={activityPage} totalPages={totalActivityPages} total={activityData?.total ?? 0} limit={limit} onPageChange={setActivityPage} />
        </>
      )}
    </div>
  );
}

// ─── Reusable Components ───

function Avatar({ name, email }: { name?: string; email?: string }) {
  const char = name?.charAt(0)?.toUpperCase() || email?.charAt(0)?.toUpperCase() || '?';
  return (
    <div className="w-8 h-8 rounded-full bg-primary-100 dark:bg-primary-900 flex items-center justify-center text-primary-700 dark:text-primary-300 font-medium text-sm flex-shrink-0">
      {char}
    </div>
  );
}

function StatCard({ icon, bgColor, label, value }: { icon: React.ReactNode; bgColor: string; label: string; value: string | number }) {
  return (
    <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-4">
      <div className="flex items-center gap-3">
        <div className={`p-2 rounded-lg ${bgColor}`}>{icon}</div>
        <div>
          <p className="text-xs text-surface-500 dark:text-surface-400">{label}</p>
          <p className="text-xl font-bold text-surface-900 dark:text-surface-100">{value}</p>
        </div>
      </div>
    </div>
  );
}

function LoadingSpinner({ text }: { text: string }) {
  return (
    <div className="p-8 text-center">
      <div className="h-6 w-6 border-2 border-primary-600 border-t-transparent rounded-full animate-spin mx-auto mb-2" />
      <p className="text-sm text-surface-400">{text}</p>
    </div>
  );
}

function EmptyState({ icon, text }: { icon: React.ReactNode; text: string }) {
  return (
    <div className="p-8 text-center">
      <div className="text-surface-300 dark:text-surface-700 mb-2 flex justify-center">{icon}</div>
      <p className="text-sm text-surface-500">{text}</p>
    </div>
  );
}

function Pagination({ page, totalPages, total, limit, onPageChange }: { page: number; totalPages: number; total: number; limit: number; onPageChange: (p: number) => void }) {
  if (totalPages <= 1) return null;
  return (
    <div className="flex items-center justify-between">
      <p className="text-sm text-surface-500 dark:text-surface-400">
        Showing {page * limit + 1}–{Math.min((page + 1) * limit, total)} of {total}
      </p>
      <div className="flex items-center gap-2">
        <button
          onClick={() => onPageChange(Math.max(0, page - 1))}
          disabled={page === 0}
          className="p-2 rounded-lg border border-surface-300 dark:border-surface-700 text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800 disabled:opacity-50 transition-colors"
        >
          <ChevronLeft className="h-4 w-4" />
        </button>
        <span className="text-sm text-surface-500 px-2">Page {page + 1} of {totalPages}</span>
        <button
          onClick={() => onPageChange(Math.min(totalPages - 1, page + 1))}
          disabled={page >= totalPages - 1}
          className="p-2 rounded-lg border border-surface-300 dark:border-surface-700 text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800 disabled:opacity-50 transition-colors"
        >
          <ChevronRight className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}
