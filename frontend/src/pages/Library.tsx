import { useState } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Library as LibraryIcon, BookOpen, Clock, CheckCircle, Bookmark, Trash2, LogIn } from 'lucide-react';
import toast from 'react-hot-toast';
import { libraryApi } from '../api/library';
import { useAuthStore } from '../stores/authStore';
import PaperCard from '../components/PaperCard';
import { ListSkeleton } from '../components/Skeleton';

const STATUS_TABS = [
  { value: '', label: 'All', icon: LibraryIcon },
  { value: 'reading', label: 'Reading', icon: BookOpen },
  { value: 'saved', label: 'Saved', icon: Clock },
  { value: 'finished', label: 'Finished', icon: CheckCircle },
];

export default function Library() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [page, setPage] = useState(0);
  const queryClient = useQueryClient();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  const activeTab = searchParams.get('tab') || 'library';
  const statusFilter = searchParams.get('status') || '';

  const { data: libraryData, isLoading: loadingLibrary } = useQuery({
    queryKey: ['library', statusFilter, page],
    queryFn: () => libraryApi.getLibrary(statusFilter, 20, page * 20),
    enabled: activeTab === 'library' && isAuthenticated,
  });

  const { data: bookmarksData, isLoading: loadingBookmarks } = useQuery({
    queryKey: ['bookmarks', page],
    queryFn: () => libraryApi.getBookmarks(20, page * 20),
    enabled: activeTab === 'bookmarks' && isAuthenticated,
  });

  const removeMutation = useMutation({
    mutationFn: libraryApi.removePaper,
    onSuccess: () => {
      toast.success('Paper removed from library');
      queryClient.invalidateQueries({ queryKey: ['library'] });
    },
    onError: () => toast.error('Failed to remove paper'),
  });

  const bookmarkMutation = useMutation({
    mutationFn: libraryApi.bookmarkPaper,
    onSuccess: () => {
      toast.success('Paper bookmarked');
      queryClient.invalidateQueries({ queryKey: ['bookmarks'] });
      queryClient.invalidateQueries({ queryKey: ['library'] });
    },
    onError: () => toast.error('Failed to bookmark'),
  });

  const unbookmarkMutation = useMutation({
    mutationFn: libraryApi.unbookmarkPaper,
    onSuccess: () => {
      toast.success('Bookmark removed');
      queryClient.invalidateQueries({ queryKey: ['bookmarks'] });
      queryClient.invalidateQueries({ queryKey: ['library'] });
    },
    onError: () => toast.error('Failed to unbookmark'),
  });

  // Not signed in
  if (!isAuthenticated) {
    return (
      <div className="text-center py-20">
        <LibraryIcon className="h-16 w-16 mx-auto text-surface-300 dark:text-surface-700 mb-4" />
        <h2 className="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-2">
          Sign in to access your library
        </h2>
        <p className="text-surface-500 dark:text-surface-400 mb-6">
          Save papers, track your reading progress, and organize your research
        </p>
        <Link
          to="/login"
          className="inline-flex items-center gap-2 px-6 py-3 rounded-xl bg-primary-600 hover:bg-primary-700 text-white font-medium transition-colors"
        >
          <LogIn className="h-5 w-5" />
          Sign in
        </Link>
      </div>
    );
  }

  const isLoading = activeTab === 'bookmarks' ? loadingBookmarks : loadingLibrary;
  const data = activeTab === 'bookmarks' ? bookmarksData : libraryData;
  const totalPages = data ? Math.ceil(data.total / 20) : 0;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-surface-900 dark:text-surface-100">Library</h1>
        <p className="text-surface-500 dark:text-surface-400 mt-1">
          Your saved and bookmarked papers
        </p>
      </div>

      {/* Main tabs */}
      <div className="flex gap-1 p-1 bg-surface-100 dark:bg-surface-800 rounded-xl w-fit">
        <button
          onClick={() => { setSearchParams({ tab: 'library' }); setPage(0); }}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            activeTab === 'library'
              ? 'bg-white dark:bg-surface-900 text-surface-900 dark:text-surface-100 shadow-sm'
              : 'text-surface-600 dark:text-surface-400 hover:text-surface-900 dark:hover:text-surface-100'
          }`}
        >
          <LibraryIcon className="h-4 w-4" />
          Library
        </button>
        <button
          onClick={() => { setSearchParams({ tab: 'bookmarks' }); setPage(0); }}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            activeTab === 'bookmarks'
              ? 'bg-white dark:bg-surface-900 text-surface-900 dark:text-surface-100 shadow-sm'
              : 'text-surface-600 dark:text-surface-400 hover:text-surface-900 dark:hover:text-surface-100'
          }`}
        >
          <Bookmark className="h-4 w-4" />
          Bookmarks
        </button>
      </div>

      {/* Status filters (library tab only) */}
      {activeTab === 'library' && (
        <div className="flex items-center gap-1 flex-wrap">
          {STATUS_TABS.map(({ value, label, icon: Icon }) => (
            <button
              key={value}
              onClick={() => {
                setPage(0);
                setSearchParams({ tab: 'library', ...(value ? { status: value } : {}) });
              }}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                statusFilter === value
                  ? 'bg-primary-100 dark:bg-primary-900 text-primary-700 dark:text-primary-300'
                  : 'text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800'
              }`}
            >
              <Icon className="h-4 w-4" />
              {label}
            </button>
          ))}
        </div>
      )}

      {/* Results */}
      {isLoading ? (
        <ListSkeleton count={5} />
      ) : data?.papers?.length ? (
        <>
          <p className="text-sm text-surface-500 dark:text-surface-400">
            {data.total} paper{data.total !== 1 ? 's' : ''}
          </p>

          <div className="space-y-3">
            {data.papers.map((up) => (
              <div key={up.id} className="relative group/item">
                <PaperCard
                  paper={up.paper}
                  isBookmarked={up.is_bookmarked}
                  onBookmark={(id) => bookmarkMutation.mutate(id)}
                  onUnbookmark={(id) => unbookmarkMutation.mutate(id)}
                />
                {up.reading_progress > 0 && (
                  <div className="absolute bottom-0 left-0 right-0 h-1 bg-surface-100 dark:bg-surface-800 rounded-b-xl overflow-hidden">
                    <div
                      className="h-full bg-primary-500 rounded-b-xl transition-all"
                      style={{ width: `${Math.min(up.reading_progress, 100)}%` }}
                    />
                  </div>
                )}
                <div className="absolute top-3 right-16">
                  <span className={`text-xs font-medium px-2 py-0.5 rounded-md ${
                    up.status === 'reading' ? 'bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300' :
                    up.status === 'finished' ? 'bg-green-50 text-green-700 dark:bg-green-950 dark:text-green-300' :
                    'bg-surface-100 text-surface-600 dark:bg-surface-800 dark:text-surface-400'
                  }`}>
                    {up.status}
                  </span>
                </div>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    if (confirm('Remove this paper from your library?')) {
                      removeMutation.mutate(up.paper_id);
                    }
                  }}
                  className="absolute bottom-3 right-3 p-1.5 rounded-lg text-surface-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-950 opacity-0 group-hover/item:opacity-100 transition-all"
                  title="Remove from library"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            ))}
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2 pt-4">
              <button
                onClick={() => setPage(Math.max(0, page - 1))}
                disabled={page === 0}
                className="px-4 py-2 rounded-lg text-sm font-medium border border-surface-300 dark:border-surface-700 disabled:opacity-50 transition-colors"
              >
                Previous
              </button>
              <span className="text-sm text-surface-500">
                Page {page + 1} of {totalPages}
              </span>
              <button
                onClick={() => setPage(Math.min(totalPages - 1, page + 1))}
                disabled={page >= totalPages - 1}
                className="px-4 py-2 rounded-lg text-sm font-medium border border-surface-300 dark:border-surface-700 disabled:opacity-50 transition-colors"
              >
                Next
              </button>
            </div>
          )}
        </>
      ) : (
        <div className="text-center py-16">
          <LibraryIcon className="h-16 w-16 mx-auto text-surface-300 dark:text-surface-700 mb-4" />
          <h3 className="text-lg font-medium text-surface-900 dark:text-surface-100 mb-1">
            {activeTab === 'bookmarks' ? 'No bookmarks yet' : 'Your library is empty'}
          </h3>
          <p className="text-surface-500 dark:text-surface-400">
            {activeTab === 'bookmarks'
              ? 'Bookmark papers to find them quickly later'
              : 'Search for papers and save them to your library'}
          </p>
        </div>
      )}
    </div>
  );
}
