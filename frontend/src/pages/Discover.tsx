import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Sparkles, RefreshCw, Tag, Calendar, Users, Quote, FileText, BookOpen, Award } from 'lucide-react';
import toast from 'react-hot-toast';
import { papersApi } from '../api/papers';
import { libraryApi } from '../api/library';
import { useAuthStore } from '../stores/authStore';
import PaperCard from '../components/PaperCard';
import { PaperDetailSkeleton } from '../components/Skeleton';
import type { Paper } from '../types';

function HeroPaperCard({ paper }: { paper: Paper }) {
  const navigate = useNavigate();

  const authors = Array.isArray(paper.authors)
    ? paper.authors
    : typeof paper.authors === 'string'
    ? (() => { try { return JSON.parse(paper.authors); } catch { return []; } })()
    : [];

  const authorText = authors.length > 5
    ? `${authors.slice(0, 5).map((a: { name: string }) => a.name).join(', ')} +${authors.length - 5} more`
    : authors.map((a: { name: string }) => a.name).join(', ');

  const publishDate = paper.published_date
    ? new Date(paper.published_date).toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric' })
    : paper.year
    ? String(paper.year)
    : null;

  const citationCount = paper.citation_count ?? 0;

  return (
    <article
      className="relative bg-gradient-to-br from-primary-50 via-white to-amber-50 dark:from-primary-950/40 dark:via-surface-900 dark:to-amber-950/30 rounded-2xl border border-primary-200 dark:border-primary-800/50 p-6 sm:p-8 cursor-pointer hover:shadow-xl hover:shadow-primary-100/50 dark:hover:shadow-primary-900/30 transition-all duration-300"
      onClick={() => navigate(`/paper/${paper.id}`)}
    >
      {/* Decorative sparkle */}
      <div className="absolute top-4 right-4">
        <Sparkles className="h-6 w-6 text-amber-400/60" />
      </div>

      <div className="space-y-4">
        {/* Meta badges */}
        <div className="flex items-center gap-2 flex-wrap">
          {paper.source && (
            <span className={`inline-flex items-center px-2.5 py-1 rounded-lg text-xs font-medium ${
              paper.source === 'arxiv'
                ? 'bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-300'
                : 'bg-surface-100 text-surface-700 dark:bg-surface-800 dark:text-surface-300'
            }`}>
              {paper.source === 'arxiv' ? 'arXiv' : paper.source === 's2' ? 'S2' : paper.source}
            </span>
          )}
          {paper.primary_category && (
            <span className="inline-flex items-center px-2.5 py-1 rounded-lg text-xs font-medium bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300">
              {paper.primary_category}
            </span>
          )}
          {paper.venue && (
            <span className="inline-flex items-center px-2.5 py-1 rounded-lg text-xs font-medium bg-purple-50 text-purple-700 dark:bg-purple-950 dark:text-purple-300">
              {paper.venue}
            </span>
          )}
          {publishDate && (
            <span className="flex items-center gap-1 text-xs text-surface-500">
              <Calendar className="h-3 w-3" />
              {publishDate}
            </span>
          )}
        </div>

        {/* Title */}
        <h2 className="text-xl sm:text-2xl font-bold text-surface-900 dark:text-surface-100 leading-snug">
          {paper.title}
        </h2>

        {/* Authors */}
        {authorText && (
          <p className="flex items-center gap-1.5 text-sm text-surface-500 dark:text-surface-400">
            <Users className="h-4 w-4 flex-shrink-0" />
            {authorText}
          </p>
        )}

        {/* Citation stats */}
        {citationCount > 0 && (
          <div className="flex items-center gap-4 flex-wrap">
            <div className="flex items-center gap-1.5 text-sm">
              <Quote className="h-4 w-4 text-amber-500" />
              <span className="font-semibold text-amber-600 dark:text-amber-400">{citationCount.toLocaleString()}</span>
              <span className="text-surface-500">citations</span>
            </div>
            {(paper.influential_citation_count ?? 0) > 0 && (
              <div className="flex items-center gap-1.5 text-sm">
                <Award className="h-4 w-4 text-orange-500" />
                <span className="font-semibold text-orange-600 dark:text-orange-400">{paper.influential_citation_count}</span>
                <span className="text-surface-500">influential</span>
              </div>
            )}
          </div>
        )}

        {/* TLDR or Abstract excerpt */}
        {(paper.tldr || paper.abstract) && (
          <p className="text-sm text-surface-600 dark:text-surface-400 leading-relaxed line-clamp-4">
            {paper.tldr || paper.abstract}
          </p>
        )}

        {/* Actions */}
        <div className="flex items-center gap-3 pt-2">
          <button
            onClick={(e) => {
              e.stopPropagation();
              navigate(`/paper/${paper.id}`);
            }}
            className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-primary-600 hover:bg-primary-700 text-white font-medium transition-colors"
          >
            <BookOpen className="h-4 w-4" />
            Read Paper
          </button>
          {paper.pdf_url && (
            <a
              href={paper.pdf_url}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              className="flex items-center gap-2 px-4 py-2.5 rounded-xl border border-surface-300 dark:border-surface-700 text-surface-700 dark:text-surface-300 font-medium hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors"
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

export default function Discover() {
  const [seed, setSeed] = useState<string | undefined>();
  const queryClient = useQueryClient();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['discover', seed],
    queryFn: () => papersApi.getDiscover(seed),
    enabled: isAuthenticated,
    staleTime: 5 * 60 * 1000,
  });

  const saveMutation = useMutation({
    mutationFn: libraryApi.savePaper,
    onSuccess: () => {
      toast.success('Saved to library');
      queryClient.invalidateQueries({ queryKey: ['library'] });
    },
    onError: () => toast.error('Failed to save'),
  });

  const bookmarkMutation = useMutation({
    mutationFn: libraryApi.bookmarkPaper,
    onSuccess: () => {
      toast.success('Bookmarked');
      queryClient.invalidateQueries({ queryKey: ['bookmarks'] });
    },
    onError: () => toast.error('Failed to bookmark'),
  });

  const handleShuffle = () => {
    setSeed(String(Date.now()));
  };

  const handleBookmark = (id: string) => {
    saveMutation.mutate(id);
    bookmarkMutation.mutate(id);
  };

  const paper = data?.paper_of_the_day;
  const suggestions = data?.suggestions ?? [];
  const categories = data?.based_on_categories ?? [];

  if (!isAuthenticated) {
    return (
      <div className="text-center py-20">
        <Sparkles className="h-16 w-16 mx-auto text-surface-300 dark:text-surface-700 mb-4" />
        <h2 className="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-2">
          Sign in to get personalized suggestions
        </h2>
        <p className="text-surface-500 dark:text-surface-400">
          We'll suggest papers based on your reading interests
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900 dark:text-surface-100 flex items-center gap-2">
            <Sparkles className="h-6 w-6 text-amber-500" />
            Discover
          </h1>
          <p className="text-surface-500 dark:text-surface-400 mt-1">
            Paper suggestions based on your reading interests
          </p>
        </div>
        <button
          onClick={handleShuffle}
          disabled={isFetching}
          className="flex items-center gap-2 px-4 py-2.5 rounded-xl border border-surface-300 dark:border-surface-700 text-surface-700 dark:text-surface-300 font-medium hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
          Shuffle
        </button>
      </div>

      {/* Interest categories */}
      {categories.length > 0 && (
        <div className="flex items-center gap-2 flex-wrap">
          <Tag className="h-4 w-4 text-surface-400 flex-shrink-0" />
          <span className="text-sm text-surface-500">Based on your interests:</span>
          {categories.slice(0, 6).map((cat) => (
            <span
              key={cat}
              className="px-2.5 py-0.5 rounded-md text-xs font-medium bg-primary-50 dark:bg-primary-950 text-primary-700 dark:text-primary-300"
            >
              {cat}
            </span>
          ))}
        </div>
      )}

      {/* Paper of the Day */}
      {isLoading ? (
        <PaperDetailSkeleton />
      ) : paper ? (
        <section>
          <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-4 flex items-center gap-2">
            <Sparkles className="h-5 w-5 text-amber-500" />
            Paper of the Day
          </h2>
          <HeroPaperCard paper={paper} />
        </section>
      ) : (
        <div className="text-center py-16 bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800">
          <Sparkles className="h-12 w-12 mx-auto text-surface-300 dark:text-surface-600 mb-3" />
          <p className="text-surface-500 dark:text-surface-400 mb-1">
            No suggestions yet
          </p>
          <p className="text-sm text-surface-400 dark:text-surface-500">
            Save some papers to your library first to get personalized suggestions
          </p>
        </div>
      )}

      {/* More Suggestions */}
      {suggestions.length > 0 && (
        <section>
          <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-4">
            More Suggestions
          </h2>
          <div className="space-y-3">
            {suggestions.map((p) => (
              <PaperCard
                key={p.id}
                paper={p}
                onBookmark={handleBookmark}
              />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
