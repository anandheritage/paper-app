import { useState } from 'react';
import { useNavigate, Navigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Search, ArrowRight, BookOpen, Bookmark, Sparkles, Trophy, Quote, Calendar, Users, RefreshCw, Tag, Award, FileText } from 'lucide-react';
import toast from 'react-hot-toast';
import { libraryApi } from '../api/library';
import { papersApi } from '../api/papers';
import { useAuthStore } from '../stores/authStore';
import PaperCard from '../components/PaperCard';
import { ListSkeleton, PaperDetailSkeleton } from '../components/Skeleton';
import type { Paper } from '../types';

function formatCitations(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(n >= 10000 ? 0 : 1)}k`;
  return n.toLocaleString();
}

const medalColors = [
  'from-amber-400 to-amber-600',
  'from-slate-300 to-slate-500',
  'from-orange-400 to-orange-600',
  'from-primary-400 to-primary-600',
  'from-primary-400 to-primary-600',
];

function TrendingPaperCard({ paper }: { paper: Paper }) {
  const navigate = useNavigate();
  const authors = Array.isArray(paper.authors)
    ? paper.authors
    : typeof paper.authors === 'string'
    ? (() => { try { return JSON.parse(paper.authors); } catch { return []; } })()
    : [];
  const authorText = authors.length > 4
    ? `${authors.slice(0, 4).map((a: { name: string }) => a.name).join(', ')} +${authors.length - 4} more`
    : authors.map((a: { name: string }) => a.name).join(', ');
  const publishDate = paper.published_date
    ? new Date(paper.published_date).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
    : paper.year ? String(paper.year) : null;
  const citationCount = paper.citation_count ?? 0;

  return (
    <article
      className="relative bg-gradient-to-br from-primary-50 via-white to-amber-50 dark:from-primary-950/40 dark:via-surface-900 dark:to-amber-950/30 rounded-2xl border border-primary-200 dark:border-primary-800/50 p-5 sm:p-6 cursor-pointer hover:shadow-xl hover:shadow-primary-100/50 dark:hover:shadow-primary-900/30 transition-all duration-300"
      onClick={() => navigate(`/paper/${paper.id}`)}
    >
      <div className="absolute top-3 right-3">
        <Sparkles className="h-5 w-5 text-amber-400/60" />
      </div>
      <div className="space-y-3">
        <div className="flex items-center gap-2 flex-wrap">
          {paper.source && (
            <span className={`inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium ${
              paper.source === 'arxiv'
                ? 'bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-300'
                : 'bg-surface-100 text-surface-700 dark:bg-surface-800 dark:text-surface-300'
            }`}>
              {paper.source === 'arxiv' ? 'arXiv' : paper.source}
            </span>
          )}
          {paper.primary_category && (
            <span className="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300">
              {paper.primary_category}
            </span>
          )}
          {publishDate && (
            <span className="flex items-center gap-1 text-xs text-surface-500">
              <Calendar className="h-3 w-3" />
              {publishDate}
            </span>
          )}
        </div>
        <h3 className="text-lg sm:text-xl font-bold text-surface-900 dark:text-surface-100 leading-snug line-clamp-2">
          {paper.title}
        </h3>
        {authorText && (
          <p className="flex items-center gap-1.5 text-sm text-surface-500 dark:text-surface-400 truncate">
            <Users className="h-3.5 w-3.5 flex-shrink-0" />
            {authorText}
          </p>
        )}
        <div className="flex items-center gap-4 flex-wrap">
          {citationCount > 0 && (
            <div className="flex items-center gap-1.5 text-sm">
              <Quote className="h-3.5 w-3.5 text-amber-500" />
              <span className="font-semibold text-amber-600 dark:text-amber-400">{citationCount.toLocaleString()}</span>
              <span className="text-surface-500">citations</span>
            </div>
          )}
          {(paper.influential_citation_count ?? 0) > 0 && (
            <div className="flex items-center gap-1.5 text-sm">
              <Award className="h-3.5 w-3.5 text-orange-500" />
              <span className="font-semibold text-orange-600 dark:text-orange-400">{paper.influential_citation_count}</span>
              <span className="text-surface-500">influential</span>
            </div>
          )}
        </div>
        {(paper.tldr || paper.abstract) && (
          <p className="text-sm text-surface-600 dark:text-surface-400 leading-relaxed line-clamp-2">
            {paper.tldr || paper.abstract}
          </p>
        )}
        <div className="flex items-center gap-3 pt-1">
          <button
            onClick={(e) => { e.stopPropagation(); navigate(`/paper/${paper.id}`); }}
            className="flex items-center gap-2 px-4 py-2 rounded-xl bg-primary-600 hover:bg-primary-700 text-white text-sm font-medium transition-colors"
          >
            <BookOpen className="h-4 w-4" />
            Read Paper
          </button>
          {paper.pdf_url && (
            <a href={paper.pdf_url} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()}
              className="flex items-center gap-2 px-3 py-2 rounded-xl border border-surface-300 dark:border-surface-700 text-surface-700 dark:text-surface-300 text-sm font-medium hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors"
            >
              <FileText className="h-4 w-4" />
              PDF
            </a>
          )}
        </div>
      </div>
    </article>
  );
}

function TopCitedMiniCard({ paper, rank }: { paper: Paper; rank: number }) {
  const navigate = useNavigate();
  const citations = paper.citation_count ?? 0;
  const authors = Array.isArray(paper.authors)
    ? paper.authors
    : typeof paper.authors === 'string'
    ? (() => { try { return JSON.parse(paper.authors); } catch { return []; } })()
    : [];
  const firstAuthor = authors.length > 0 ? authors[0].name : '';

  return (
    <article
      onClick={() => navigate(`/paper/${paper.id}`)}
      className="group relative bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-3.5 cursor-pointer hover:shadow-lg hover:border-primary-300 dark:hover:border-primary-700 transition-all duration-200 flex flex-col min-w-0"
    >
      <div className={`absolute -top-2 -left-2 w-6 h-6 rounded-full bg-gradient-to-br ${medalColors[rank - 1] || medalColors[4]} text-white text-[10px] font-bold flex items-center justify-center shadow-md`}>
        {rank}
      </div>
      {paper.primary_category && (
        <span className="self-start inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300 mb-1.5">
          {paper.primary_category}
        </span>
      )}
      <h4 className="text-xs font-semibold text-surface-900 dark:text-surface-100 leading-snug line-clamp-2 group-hover:text-primary-600 dark:group-hover:text-primary-400 transition-colors flex-1">
        {paper.title}
      </h4>
      {firstAuthor && (
        <p className="text-[10px] text-surface-500 dark:text-surface-400 mt-1.5 truncate">{firstAuthor}</p>
      )}
      <div className="flex items-center gap-1 mt-2 pt-2 border-t border-surface-100 dark:border-surface-800">
        <Quote className="h-3 w-3 text-amber-500" />
        <span className="text-xs font-bold text-amber-600 dark:text-amber-400">{formatCitations(citations)}</span>
      </div>
    </article>
  );
}

export default function Dashboard() {
  const [searchQuery, setSearchQuery] = useState('');
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  // Library data
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

  // Discover data
  const [discoverSeed, setDiscoverSeed] = useState<string | undefined>();
  const { data: discoverData, isLoading: loadingDiscover, isFetching: fetchingDiscover } = useQuery({
    queryKey: ['discover', discoverSeed],
    queryFn: () => papersApi.getDiscover(discoverSeed),
    enabled: isAuthenticated,
    staleTime: 5 * 60 * 1000,
  });

  const saveMutation = useMutation({
    mutationFn: libraryApi.savePaper,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['library'] }),
    onError: () => toast.error('Failed to save'),
  });

  const bookmarkMutation = useMutation({
    mutationFn: libraryApi.bookmarkPaper,
    onSuccess: () => {
      toast.success('Bookmarked!');
      queryClient.invalidateQueries({ queryKey: ['bookmarks'] });
      queryClient.invalidateQueries({ queryKey: ['library'] });
    },
    onError: () => toast.error('Failed to bookmark'),
  });

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (searchQuery.trim()) {
      navigate(`/search?q=${encodeURIComponent(searchQuery.trim())}`);
    }
  };

  const handleBookmark = (id: string) => {
    saveMutation.mutate(id, { onSuccess: () => bookmarkMutation.mutate(id) });
  };

  const handleShuffle = () => setDiscoverSeed(String(Date.now()));

  const greeting = () => {
    const hour = new Date().getHours();
    if (hour < 12) return 'Good morning';
    if (hour < 18) return 'Good afternoon';
    return 'Good evening';
  };

  const paper = discoverData?.paper_of_the_day;
  const suggestions = discoverData?.suggestions ?? [];
  const topCited = discoverData?.top_cited ?? [];
  const categories = discoverData?.based_on_categories ?? [];

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

      {/* Stats */}
      {isAuthenticated && (
        <div className="grid grid-cols-2 sm:grid-cols-2 gap-4 max-w-lg">
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
        </div>
      )}

      {/* Continue Reading */}
      {isAuthenticated && (recentPapers?.papers?.length ?? 0) > 0 && (
        <section>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 flex items-center gap-2">
              <BookOpen className="h-5 w-5 text-blue-500" />
              Continue Reading
            </h2>
            <button
              onClick={() => navigate('/library?status=reading')}
              className="flex items-center gap-1 text-sm text-primary-600 dark:text-primary-400 hover:underline"
            >
              View all <ArrowRight className="h-4 w-4" />
            </button>
          </div>
          {loadingRecent ? (
            <ListSkeleton count={2} />
          ) : (
            <div className="space-y-3">
              {recentPapers!.papers.slice(0, 3).map((up) => (
                <div key={up.id} className="relative">
                  <PaperCard paper={up.paper} compact />
                  {up.reading_progress > 0 && (
                    <div className="absolute bottom-0 left-0 right-0 h-1 bg-surface-100 dark:bg-surface-800 rounded-b-xl overflow-hidden">
                      <div className="h-full bg-primary-500 rounded-b-xl transition-all" style={{ width: `${Math.min(up.reading_progress, 100)}%` }} />
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </section>
      )}

      {/* ── Discover Section ── */}

      {/* Trending Paper of the Day */}
      {isAuthenticated && (
        <section>
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 flex items-center gap-2">
                <Sparkles className="h-5 w-5 text-amber-500" />
                Trending Today
              </h2>
              {categories.length > 0 && (
                <div className="flex items-center gap-1.5 mt-1.5 flex-wrap">
                  <Tag className="h-3 w-3 text-surface-400" />
                  {categories.slice(0, 4).map((cat) => (
                    <span key={cat} className="px-2 py-0.5 rounded text-[10px] font-medium bg-primary-50 dark:bg-primary-950 text-primary-700 dark:text-primary-300">
                      {cat}
                    </span>
                  ))}
                </div>
              )}
            </div>
            <button
              onClick={handleShuffle}
              disabled={fetchingDiscover}
              className="flex items-center gap-1.5 px-3 py-2 rounded-xl border border-surface-300 dark:border-surface-700 text-surface-600 dark:text-surface-400 text-sm font-medium hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors disabled:opacity-50"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${fetchingDiscover ? 'animate-spin' : ''}`} />
              Shuffle
            </button>
          </div>
          {loadingDiscover ? (
            <PaperDetailSkeleton />
          ) : paper ? (
            <TrendingPaperCard paper={paper} />
          ) : (
            <div className="text-center py-10 bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800">
              <Sparkles className="h-10 w-10 mx-auto text-surface-300 dark:text-surface-600 mb-2" />
              <p className="text-sm text-surface-500 dark:text-surface-400">
                Read a few papers to get personalized recommendations
              </p>
            </div>
          )}
        </section>
      )}

      {/* Top Cited of All Time */}
      {topCited.length > 0 && (
        <section>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 flex items-center gap-2">
              <Trophy className="h-5 w-5 text-amber-500" />
              Most Cited Papers
            </h2>
            <button
              onClick={() => navigate('/discover')}
              className="flex items-center gap-1 text-sm text-primary-600 dark:text-primary-400 hover:underline"
            >
              See more <ArrowRight className="h-4 w-4" />
            </button>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
            {topCited.map((p, idx) => (
              <TopCitedMiniCard key={p.id} paper={p} rank={idx + 1} />
            ))}
          </div>
        </section>
      )}

      {/* More Suggestions */}
      {suggestions.length > 0 && (
        <section>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100">
              Recommended for You
            </h2>
            <button
              onClick={() => navigate('/discover')}
              className="flex items-center gap-1 text-sm text-primary-600 dark:text-primary-400 hover:underline"
            >
              More <ArrowRight className="h-4 w-4" />
            </button>
          </div>
          <div className="space-y-3">
            {suggestions.slice(0, 3).map((p) => (
              <PaperCard key={p.id} paper={p} onBookmark={handleBookmark} />
            ))}
          </div>
        </section>
      )}

      {/* Recent Bookmarks */}
      {isAuthenticated && (bookmarks?.papers?.length ?? 0) > 0 && (
        <section>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 flex items-center gap-2">
              <Bookmark className="h-5 w-5 text-primary-500" />
              Recent Bookmarks
            </h2>
            <button
              onClick={() => navigate('/library?tab=bookmarks')}
              className="flex items-center gap-1 text-sm text-primary-600 dark:text-primary-400 hover:underline"
            >
              View all <ArrowRight className="h-4 w-4" />
            </button>
          </div>
          {loadingBookmarks ? (
            <ListSkeleton count={2} />
          ) : (
            <div className="space-y-3">
              {bookmarks!.papers.slice(0, 3).map((up) => (
                <PaperCard key={up.id} paper={up.paper} compact isBookmarked />
              ))}
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
    </div>
  );
}
