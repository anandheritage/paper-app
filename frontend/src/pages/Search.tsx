import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Search as SearchIcon, X, ArrowUpDown, Tag, ChevronDown, ChevronUp } from 'lucide-react';
import toast from 'react-hot-toast';
import { papersApi } from '../api/papers';
import { libraryApi } from '../api/library';
import { useAuthStore } from '../stores/authStore';
import PaperCard from '../components/PaperCard';
import { ListSkeleton } from '../components/Skeleton';

const SORT_OPTIONS = [
  { value: 'relevance', label: 'Relevance' },
  { value: 'citations', label: 'Most Cited' },
  { value: 'date', label: 'Newest First' },
];

export default function Search() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [query, setQuery] = useState(searchParams.get('q') || '');
  const [sort, setSort] = useState(searchParams.get('sort') || 'relevance');
  const [selectedCategories, setSelectedCategories] = useState<string[]>(() => {
    const cats = searchParams.get('categories');
    return cats ? cats.split(',') : [];
  });
  const [showCategories, setShowCategories] = useState(false);
  const [page, setPage] = useState(0);
  const queryClient = useQueryClient();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  const searchQuery = searchParams.get('q') || '';
  const searchSort = searchParams.get('sort') || 'relevance';
  const searchCategories = searchParams.get('categories')?.split(',').filter(Boolean) || [];

  // Fetch categories for filter panel
  const { data: groupedCategories } = useQuery({
    queryKey: ['categories', 'grouped'],
    queryFn: () => papersApi.getGroupedCategories(),
    staleTime: 10 * 60 * 1000, // Cache for 10 minutes
  });

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['search', searchQuery, searchSort, searchCategories.join(','), page],
    queryFn: () => papersApi.search(searchQuery, '', 20, page * 20, searchSort, searchCategories.length > 0 ? searchCategories : undefined),
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
    setSort(searchParams.get('sort') || 'relevance');
    const cats = searchParams.get('categories');
    setSelectedCategories(cats ? cats.split(',').filter(Boolean) : []);
  }, [searchParams]);

  const updateSearch = (params: Record<string, string | undefined>) => {
    const newParams: Record<string, string> = {};
    const q = params.q ?? searchQuery;
    if (q) newParams.q = q;
    const s = params.sort ?? searchSort;
    if (s && s !== 'relevance') newParams.sort = s;
    const c = params.categories !== undefined ? params.categories : searchCategories.join(',');
    if (c) newParams.categories = c;
    setPage(0);
    setSearchParams(newParams);
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;
    updateSearch({ q: query.trim() });
  };

  const toggleCategory = (catId: string) => {
    const newCats = selectedCategories.includes(catId)
      ? selectedCategories.filter((c) => c !== catId)
      : [...selectedCategories, catId];
    setSelectedCategories(newCats);
    if (searchQuery) {
      updateSearch({ categories: newCats.join(',') || undefined });
    }
  };

  const clearCategories = () => {
    setSelectedCategories([]);
    if (searchQuery) {
      updateSearch({ categories: undefined });
    }
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

  // Format number for display
  const formatCount = (n: number) => {
    if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
    return String(n);
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-surface-900 dark:text-surface-100">Search Papers</h1>
        <p className="text-surface-500 dark:text-surface-400 mt-1">
          Search across millions of research papers powered by Semantic Scholar
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

        {/* Sort + Category toggle */}
        <div className="flex items-center justify-between gap-4 flex-wrap">
          <div className="flex items-center gap-3">
            {/* Category filter toggle */}
            <button
              type="button"
              onClick={() => setShowCategories(!showCategories)}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                selectedCategories.length > 0
                  ? 'bg-primary-100 dark:bg-primary-900 text-primary-700 dark:text-primary-300'
                  : 'text-surface-600 dark:text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800'
              }`}
            >
              <Tag className="h-3.5 w-3.5" />
              Fields
              {selectedCategories.length > 0 && (
                <span className="ml-1 px-1.5 py-0.5 rounded-full bg-primary-200 dark:bg-primary-800 text-xs">
                  {selectedCategories.length}
                </span>
              )}
              {showCategories ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
            </button>

            {selectedCategories.length > 0 && (
              <button
                type="button"
                onClick={clearCategories}
                className="text-xs text-surface-400 hover:text-surface-600 dark:hover:text-surface-300"
              >
                Clear filters
              </button>
            )}
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
                      if (searchQuery) updateSearch({ sort: s.value });
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

      {/* Category filter panel */}
      {showCategories && groupedCategories && (
        <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-surface-900 dark:text-surface-100">Filter by Field</h3>
            {selectedCategories.length > 0 && (
              <button onClick={clearCategories} className="text-xs text-primary-600 dark:text-primary-400 hover:underline">
                Clear all
              </button>
            )}
          </div>
          {Object.entries(groupedCategories)
            .sort(([a], [b]) => a.localeCompare(b))
            .map(([group, cats]) => (
              <div key={group}>
                <div className="flex flex-wrap gap-1.5">
                  {cats
                    .sort((a, b) => b.count - a.count)
                    .map((cat) => (
                      <button
                        key={cat.id}
                        type="button"
                        onClick={() => toggleCategory(cat.id)}
                        className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-medium transition-colors ${
                          selectedCategories.includes(cat.id)
                            ? 'bg-primary-100 dark:bg-primary-900 text-primary-700 dark:text-primary-300 ring-1 ring-primary-300 dark:ring-primary-700'
                            : 'bg-surface-100 dark:bg-surface-800 text-surface-600 dark:text-surface-400 hover:bg-surface-200 dark:hover:bg-surface-700'
                        }`}
                      >
                        {cat.name}
                        <span className="text-[10px] opacity-60">{formatCount(cat.count)}</span>
                      </button>
                    ))}
                </div>
              </div>
            ))}
        </div>
      )}

      {/* Active category pills */}
      {selectedCategories.length > 0 && !showCategories && (
        <div className="flex flex-wrap gap-1.5">
          {selectedCategories.map((catId) => (
            <span
              key={catId}
              className="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-medium bg-primary-100 dark:bg-primary-900 text-primary-700 dark:text-primary-300"
            >
              {catId}
              <button onClick={() => toggleCategory(catId)} className="hover:text-primary-900 dark:hover:text-primary-100">
                <X className="h-3 w-3" />
              </button>
            </span>
          ))}
        </div>
      )}

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
            Try different keywords or adjust your field filters
          </p>
        </div>
      ) : (
        <div className="text-center py-16">
          <SearchIcon className="h-16 w-16 mx-auto text-surface-300 dark:text-surface-700 mb-4" />
          <h3 className="text-lg font-medium text-surface-900 dark:text-surface-100 mb-1">Search academic papers</h3>
          <p className="text-surface-500 dark:text-surface-400">
            Search across millions of papers from arXiv and more
          </p>
        </div>
      )}
    </div>
  );
}
