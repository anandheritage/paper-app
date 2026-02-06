import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Search as SearchIcon, Filter, X, ArrowUpDown } from 'lucide-react';
import toast from 'react-hot-toast';
import { papersApi } from '../api/papers';
import { libraryApi } from '../api/library';
import { useAuthStore } from '../stores/authStore';
import PaperCard from '../components/PaperCard';
import { ListSkeleton } from '../components/Skeleton';

const SOURCES = [
  { value: '', label: 'All Sources' },
  { value: 'arxiv', label: 'arXiv' },
  { value: 'pubmed', label: 'PubMed' },
];

const SORT_OPTIONS = [
  { value: 'relevance', label: 'Relevance' },
  { value: 'citations', label: 'Most Cited' },
  { value: 'date', label: 'Newest First' },
];

export default function Search() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [query, setQuery] = useState(searchParams.get('q') || '');
  const [source, setSource] = useState(searchParams.get('source') || '');
  const [sort, setSort] = useState(searchParams.get('sort') || 'relevance');
  const [page, setPage] = useState(0);
  const queryClient = useQueryClient();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  const searchQuery = searchParams.get('q') || '';
  const searchSource = searchParams.get('source') || '';
  const searchSort = searchParams.get('sort') || 'relevance';

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['search', searchQuery, searchSource, searchSort, page],
    queryFn: () => papersApi.search(searchQuery, searchSource, 20, page * 20, searchSort),
    enabled: !!searchQuery,
    placeholderData: (prev) => prev,
  });

  const saveMutation = useMutation({
    mutationFn: libraryApi.savePaper,
    onSuccess: () => {
      toast.success('Paper saved to library');
      queryClient.invalidateQueries({ queryKey: ['library'] });
    },
    onError: () => toast.error('Failed to save paper'),
  });

  const bookmarkMutation = useMutation({
    mutationFn: libraryApi.bookmarkPaper,
    onSuccess: () => {
      toast.success('Paper bookmarked');
      queryClient.invalidateQueries({ queryKey: ['bookmarks'] });
    },
    onError: () => toast.error('Failed to bookmark paper'),
  });

  useEffect(() => {
    setQuery(searchParams.get('q') || '');
    setSource(searchParams.get('source') || '');
    setSort(searchParams.get('sort') || 'relevance');
  }, [searchParams]);

  const updateSearch = (params: Record<string, string>) => {
    const newParams: Record<string, string> = {
      q: params.q ?? searchQuery,
      ...(params.source !== undefined ? (params.source ? { source: params.source } : {}) : (searchSource ? { source: searchSource } : {})),
      ...(params.sort !== undefined ? (params.sort && params.sort !== 'relevance' ? { sort: params.sort } : {}) : (searchSort && searchSort !== 'relevance' ? { sort: searchSort } : {})),
    };
    setPage(0);
    setSearchParams(newParams);
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;
    updateSearch({ q: query.trim() });
  };

  const handleBookmark = (id: string) => {
    if (!isAuthenticated) {
      toast.error('Sign in to bookmark papers');
      return;
    }
    saveMutation.mutate(id);
    bookmarkMutation.mutate(id);
  };

  const totalPages = data ? Math.ceil(data.total / 20) : 0;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-surface-900 dark:text-surface-100">Search Papers</h1>
        <p className="text-surface-500 dark:text-surface-400 mt-1">
          Discover research papers from arXiv, PubMed and Semantic Scholar
        </p>
      </div>

      {/* Search bar */}
      <form onSubmit={handleSearch} className="space-y-3">
        <div className="flex gap-3">
          <div className="relative flex-1">
            <SearchIcon className="absolute left-4 top-1/2 -translate-y-1/2 h-5 w-5 text-surface-400" />
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search by title, author, topic..."
              className="w-full pl-12 pr-4 py-3 rounded-xl border border-surface-300 dark:border-surface-700 bg-white dark:bg-surface-900 text-surface-900 dark:text-surface-100 placeholder:text-surface-400 focus:outline-none focus:ring-2 focus:ring-primary-500/40 focus:border-primary-500 transition-all"
            />
            {query && (
              <button
                type="button"
                onClick={() => { setQuery(''); setSearchParams({}); }}
                className="absolute right-3 top-1/2 -translate-y-1/2 p-1 text-surface-400 hover:text-surface-600"
              >
                <X className="h-4 w-4" />
              </button>
            )}
          </div>
          <button
            type="submit"
            disabled={!query.trim()}
            className="px-6 py-3 rounded-xl bg-primary-600 hover:bg-primary-700 text-white font-medium transition-colors disabled:opacity-50"
          >
            Search
          </button>
        </div>

        {/* Filters row */}
        <div className="flex items-center justify-between gap-4 flex-wrap">
          <div className="flex items-center gap-2">
            <Filter className="h-4 w-4 text-surface-400" />
            <div className="flex gap-1">
              {SOURCES.map((s) => (
                <button
                  key={s.value}
                  type="button"
                  onClick={() => {
                    setSource(s.value);
                    if (searchQuery) {
                      updateSearch({ source: s.value });
                    }
                  }}
                  className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                    source === s.value
                      ? 'bg-primary-100 dark:bg-primary-900 text-primary-700 dark:text-primary-300'
                      : 'text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800'
                  }`}
                >
                  {s.label}
                </button>
              ))}
            </div>
          </div>

          {/* Sort control */}
          {searchQuery && (
            <div className="flex items-center gap-2">
              <ArrowUpDown className="h-4 w-4 text-surface-400" />
              <div className="flex gap-1">
                {SORT_OPTIONS.map((s) => (
                  <button
                    key={s.value}
                    type="button"
                    onClick={() => {
                      setSort(s.value);
                      if (searchQuery) {
                        updateSearch({ sort: s.value });
                      }
                    }}
                    className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                      sort === s.value
                        ? 'bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300'
                        : 'text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800'
                    }`}
                  >
                    {s.label}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      </form>

      {/* Results */}
      {isLoading ? (
        <ListSkeleton count={5} />
      ) : data?.papers?.length ? (
        <>
          <div className="flex items-center justify-between">
            <p className="text-sm text-surface-500 dark:text-surface-400">
              {data.total.toLocaleString()} results found
              {isFetching && ' (updating...)'}
            </p>
          </div>

          <div className="space-y-3">
            {data.papers.map((paper) => (
              <PaperCard
                key={paper.id}
                paper={paper}
                onBookmark={handleBookmark}
              />
            ))}
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2 pt-4">
              <button
                onClick={() => setPage(Math.max(0, page - 1))}
                disabled={page === 0}
                className="px-4 py-2 rounded-lg text-sm font-medium border border-surface-300 dark:border-surface-700 text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800 disabled:opacity-50 transition-colors"
              >
                Previous
              </button>
              <span className="text-sm text-surface-500">
                Page {page + 1} of {Math.min(totalPages, 50)}
              </span>
              <button
                onClick={() => setPage(Math.min(totalPages - 1, page + 1))}
                disabled={page >= totalPages - 1}
                className="px-4 py-2 rounded-lg text-sm font-medium border border-surface-300 dark:border-surface-700 text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800 disabled:opacity-50 transition-colors"
              >
                Next
              </button>
            </div>
          )}
        </>
      ) : searchQuery ? (
        <div className="text-center py-16">
          <SearchIcon className="h-16 w-16 mx-auto text-surface-300 dark:text-surface-700 mb-4" />
          <h3 className="text-lg font-medium text-surface-900 dark:text-surface-100 mb-1">No results found</h3>
          <p className="text-surface-500 dark:text-surface-400">
            Try different keywords or change the source filter
          </p>
        </div>
      ) : (
        <div className="text-center py-16">
          <SearchIcon className="h-16 w-16 mx-auto text-surface-300 dark:text-surface-700 mb-4" />
          <h3 className="text-lg font-medium text-surface-900 dark:text-surface-100 mb-1">Search academic papers</h3>
          <p className="text-surface-500 dark:text-surface-400">
            Enter keywords to search across arXiv, PubMed and Semantic Scholar
          </p>
        </div>
      )}
    </div>
  );
}
