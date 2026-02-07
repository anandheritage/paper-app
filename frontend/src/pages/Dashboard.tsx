import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Search, ArrowRight, BookOpen, Bookmark, Clock } from 'lucide-react';
import { libraryApi } from '../api/library';
import { useAuthStore } from '../stores/authStore';
import PaperCard from '../components/PaperCard';
import { ListSkeleton } from '../components/Skeleton';

export default function Dashboard() {
  const [searchQuery, setSearchQuery] = useState('');
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  const { data: recentPapers, isLoading: loadingRecent } = useQuery({
    queryKey: ['library', 'reading'],
    queryFn: () => libraryApi.getLibrary('reading', 5, 0),
    enabled: isAuthenticated,
  });

  const { data: bookmarks, isLoading: loadingBookmarks } = useQuery({
    queryKey: ['bookmarks', 'recent'],
    queryFn: () => libraryApi.getBookmarks(5, 0),
    enabled: isAuthenticated,
  });

  const { data: savedPapers, isLoading: loadingSaved } = useQuery({
    queryKey: ['library', 'saved'],
    queryFn: () => libraryApi.getLibrary('saved', 5, 0),
    enabled: isAuthenticated,
  });

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (searchQuery.trim()) {
      navigate(`/search?q=${encodeURIComponent(searchQuery.trim())}`);
    }
  };

  const greeting = () => {
    const hour = new Date().getHours();
    if (hour < 12) return 'Good morning';
    if (hour < 18) return 'Good afternoon';
    return 'Good evening';
  };

  return (
    <div className="space-y-8">
      {/* Welcome + Search */}
      <div className="space-y-4">
        <div>
          <h1 className="text-2xl font-bold text-surface-900 dark:text-surface-100">
            {greeting()}, {user?.name?.split(' ')[0] || 'Reader'}
          </h1>
          <p className="text-surface-500 dark:text-surface-400 mt-1">
            What would you like to read today?
          </p>
        </div>

        <form onSubmit={handleSearch} className="flex gap-3 max-w-xl">
          <div className="relative flex-1">
            <Search className="absolute left-4 top-1/2 -translate-y-1/2 h-5 w-5 text-surface-400" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search arXiv papers..."
              className="w-full pl-12 pr-4 py-3.5 rounded-xl border border-surface-300 dark:border-surface-700 bg-white dark:bg-surface-900 text-surface-900 dark:text-surface-100 placeholder:text-surface-400 focus:outline-none focus:ring-2 focus:ring-primary-500/40 focus:border-primary-500 shadow-sm transition-all"
            />
          </div>
          <button
            type="submit"
            disabled={!searchQuery.trim()}
            className="px-6 py-3.5 rounded-xl bg-primary-600 hover:bg-primary-700 text-white font-medium transition-colors disabled:opacity-50"
          >
            Search
          </button>
        </form>
      </div>

      {/* Stats - only show when authenticated */}
      {isAuthenticated && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-4 flex items-center gap-4">
            <div className="p-3 rounded-xl bg-blue-50 dark:bg-blue-950 text-blue-600 dark:text-blue-400">
              <BookOpen className="h-6 w-6" />
            </div>
            <div>
              <p className="text-2xl font-bold text-surface-900 dark:text-surface-100">{recentPapers?.total ?? 0}</p>
              <p className="text-sm text-surface-500">Reading</p>
            </div>
          </div>
          <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-4 flex items-center gap-4">
            <div className="p-3 rounded-xl bg-primary-50 dark:bg-primary-950 text-primary-600 dark:text-primary-400">
              <Bookmark className="h-6 w-6" />
            </div>
            <div>
              <p className="text-2xl font-bold text-surface-900 dark:text-surface-100">{bookmarks?.total ?? 0}</p>
              <p className="text-sm text-surface-500">Bookmarked</p>
            </div>
          </div>
          <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-4 flex items-center gap-4">
            <div className="p-3 rounded-xl bg-green-50 dark:bg-green-950 text-green-600 dark:text-green-400">
              <Clock className="h-6 w-6" />
            </div>
            <div>
              <p className="text-2xl font-bold text-surface-900 dark:text-surface-100">{savedPapers?.total ?? 0}</p>
              <p className="text-sm text-surface-500">Saved</p>
            </div>
          </div>
        </div>
      )}

      {/* Continue Reading - only show when authenticated */}
      {isAuthenticated && (
        <section>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100">Continue Reading</h2>
            {(recentPapers?.total ?? 0) > 0 && (
              <button
                onClick={() => navigate('/library?status=reading')}
                className="flex items-center gap-1 text-sm text-primary-600 dark:text-primary-400 hover:underline"
              >
                View all <ArrowRight className="h-4 w-4" />
              </button>
            )}
          </div>
          {loadingRecent ? (
            <ListSkeleton count={2} />
          ) : recentPapers?.papers?.length ? (
            <div className="space-y-3">
              {recentPapers.papers.map((up) => (
                <div key={up.id} className="relative">
                  <PaperCard paper={up.paper} compact />
                  {up.reading_progress > 0 && (
                    <div className="absolute bottom-0 left-0 right-0 h-1 bg-surface-100 dark:bg-surface-800 rounded-b-xl overflow-hidden">
                      <div
                        className="h-full bg-primary-500 rounded-b-xl transition-all"
                        style={{ width: `${Math.min(up.reading_progress, 100)}%` }}
                      />
                    </div>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center py-12 bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800">
              <BookOpen className="h-12 w-12 mx-auto text-surface-300 dark:text-surface-600 mb-3" />
              <p className="text-surface-500 dark:text-surface-400">No papers in progress</p>
              <button
                onClick={() => navigate('/search')}
                className="mt-3 text-sm text-primary-600 dark:text-primary-400 hover:underline"
              >
                Search for papers to read
              </button>
            </div>
          )}
        </section>
      )}

      {/* Quick start for guests */}
      {!isAuthenticated && (
        <div className="text-center py-12 bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800">
          <Search className="h-12 w-12 mx-auto text-surface-300 dark:text-surface-600 mb-3" />
          <p className="text-surface-500 dark:text-surface-400 mb-2">Search and read 940K+ arXiv research papers</p>
          <button
            onClick={() => navigate('/search')}
            className="mt-2 px-6 py-2.5 rounded-xl bg-primary-600 hover:bg-primary-700 text-white text-sm font-medium transition-colors"
          >
            Start searching
          </button>
        </div>
      )}

      {/* Recent Bookmarks - only show when authenticated */}
      {isAuthenticated && (
        <section>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100">Recent Bookmarks</h2>
            {(bookmarks?.total ?? 0) > 0 && (
              <button
                onClick={() => navigate('/library?tab=bookmarks')}
                className="flex items-center gap-1 text-sm text-primary-600 dark:text-primary-400 hover:underline"
              >
                View all <ArrowRight className="h-4 w-4" />
              </button>
            )}
          </div>
          {loadingBookmarks ? (
            <ListSkeleton count={2} />
          ) : bookmarks?.papers?.length ? (
            <div className="space-y-3">
              {bookmarks.papers.map((up) => (
                <PaperCard key={up.id} paper={up.paper} compact isBookmarked />
              ))}
            </div>
          ) : (
            <div className="text-center py-8 bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800">
              <Bookmark className="h-10 w-10 mx-auto text-surface-300 dark:text-surface-600 mb-2" />
              <p className="text-sm text-surface-500 dark:text-surface-400">No bookmarks yet</p>
            </div>
          )}
        </section>
      )}

      {/* Recently Saved - only show when authenticated */}
      {isAuthenticated && !loadingSaved && savedPapers?.papers?.length ? (
        <section>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100">Recently Saved</h2>
            <button
              onClick={() => navigate('/library')}
              className="flex items-center gap-1 text-sm text-primary-600 dark:text-primary-400 hover:underline"
            >
              View all <ArrowRight className="h-4 w-4" />
            </button>
          </div>
          <div className="space-y-3">
            {savedPapers.papers.slice(0, 3).map((up) => (
              <PaperCard key={up.id} paper={up.paper} compact />
            ))}
          </div>
        </section>
      ) : null}
    </div>
  );
}
