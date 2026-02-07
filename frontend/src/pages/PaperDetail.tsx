import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Calendar, Users, ExternalLink, BookOpen, Bookmark, BookmarkCheck, Plus, Check, ArrowLeft, FileText, Tag } from 'lucide-react';
import toast from 'react-hot-toast';
import { papersApi } from '../api/papers';
import { libraryApi } from '../api/library';
import { useAuthStore } from '../stores/authStore';
import { PaperDetailSkeleton } from '../components/Skeleton';



function getArxivAbsUrl(externalId: string): string {
  return `https://arxiv.org/abs/${externalId}`;
}

function getArxivPdfUrl(externalId: string): string {
  return `https://arxiv.org/pdf/${externalId}`;
}

function getArxivHtmlUrl(externalId: string): string {
  return `https://arxiv.org/html/${externalId}`;
}

export default function PaperDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  const { data: paper, isLoading } = useQuery({
    queryKey: ['paper', id],
    queryFn: () => papersApi.getById(id!),
    enabled: !!id,
  });

  const { data: libraryData } = useQuery({
    queryKey: ['library', ''],
    queryFn: () => libraryApi.getLibrary('', 100, 0),
    enabled: isAuthenticated,
  });

  const userPaper = libraryData?.papers?.find((up) => up.paper_id === id);
  const isSaved = !!userPaper;
  const isBookmarked = userPaper?.is_bookmarked ?? false;

  const saveMutation = useMutation({
    mutationFn: libraryApi.savePaper,
    onSuccess: () => {
      toast.success('Saved to library');
      queryClient.invalidateQueries({ queryKey: ['library'] });
    },
    onError: () => toast.error('Failed to save'),
  });

  const removeMutation = useMutation({
    mutationFn: libraryApi.removePaper,
    onSuccess: () => {
      toast.success('Removed from library');
      queryClient.invalidateQueries({ queryKey: ['library'] });
    },
    onError: () => toast.error('Failed to remove'),
  });

  const bookmarkMutation = useMutation({
    mutationFn: libraryApi.bookmarkPaper,
    onSuccess: () => {
      toast.success('Bookmarked');
      queryClient.invalidateQueries({ queryKey: ['library'] });
      queryClient.invalidateQueries({ queryKey: ['bookmarks'] });
    },
  });

  const unbookmarkMutation = useMutation({
    mutationFn: libraryApi.unbookmarkPaper,
    onSuccess: () => {
      toast.success('Bookmark removed');
      queryClient.invalidateQueries({ queryKey: ['library'] });
      queryClient.invalidateQueries({ queryKey: ['bookmarks'] });
    },
  });

  const handleSave = () => {
    if (!isAuthenticated) { toast.error('Sign in to save papers'); return; }
    saveMutation.mutate(paper!.id);
  };

  const handleRemove = () => {
    removeMutation.mutate(paper!.id);
  };

  const handleBookmarkToggle = () => {
    if (!isAuthenticated) { toast.error('Sign in to bookmark papers'); return; }
    if (isBookmarked) unbookmarkMutation.mutate(paper!.id);
    else bookmarkMutation.mutate(paper!.id);
  };

  if (isLoading) {
    return (
      <div className="max-w-3xl mx-auto">
        <PaperDetailSkeleton />
      </div>
    );
  }

  if (!paper) {
    return (
      <div className="text-center py-16">
        <p className="text-surface-500">Paper not found</p>
        <button onClick={() => navigate(-1)} className="mt-3 text-primary-600 hover:underline text-sm">
          Go back
        </button>
      </div>
    );
  }

  const authors = Array.isArray(paper.authors)
    ? paper.authors
    : typeof paper.authors === 'string'
    ? (() => { try { return JSON.parse(paper.authors); } catch { return []; } })()
    : [];

  const publishDate = paper.published_date
    ? new Date(paper.published_date).toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric' })
    : null;

  // Extract categories — prefer the new top-level field, fallback to metadata
  let categories: string[] = paper.categories ?? [];
  if (categories.length === 0 && paper.metadata) {
    try {
      const meta = typeof paper.metadata === 'string' ? JSON.parse(paper.metadata) : paper.metadata;
      if (Array.isArray(meta.categories)) {
        categories = meta.categories;
      }
    } catch { /* ignore */ }
  }

  const isArxiv = paper.source === 'arxiv';

  return (
    <div className="max-w-3xl mx-auto">
      {/* Back button */}
      <button
        onClick={() => navigate(-1)}
        className="flex items-center gap-1 text-sm text-surface-500 hover:text-surface-700 dark:hover:text-surface-300 mb-6 transition-colors"
      >
        <ArrowLeft className="h-4 w-4" />
        Back
      </button>

      {/* Header */}
      <div className="space-y-4">
        <div className="flex items-center gap-2 flex-wrap">
          <span className={`inline-flex items-center px-2.5 py-1 rounded-lg text-xs font-medium ${
            isArxiv
              ? 'bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-300'
              : 'bg-surface-100 text-surface-700 dark:bg-surface-800 dark:text-surface-300'
          }`}>
            {isArxiv ? 'arXiv' : paper.source}
          </span>
          <span className="text-sm text-surface-400">{paper.external_id}</span>
        </div>

        <h1 className="text-2xl sm:text-3xl font-bold text-surface-900 dark:text-surface-100 leading-tight">
          {paper.title}
        </h1>

        {/* Authors */}
        {authors.length > 0 && (
          <div className="flex items-start gap-2">
            <Users className="h-5 w-5 text-surface-400 mt-0.5 flex-shrink-0" />
            <div className="flex flex-wrap gap-x-2 gap-y-1">
              {authors.map((author: { name: string; affiliation?: string }, i: number) => (
                <span key={i} className="text-sm text-surface-600 dark:text-surface-400" title={author.affiliation}>
                  {author.name}{i < authors.length - 1 ? ',' : ''}
                </span>
              ))}
            </div>
          </div>
        )}

        {publishDate && (
          <div className="flex items-center gap-2 text-sm text-surface-500">
            <Calendar className="h-4 w-4" />
            {publishDate}
          </div>
        )}

        {/* Categories */}
        {categories.length > 0 && (
          <div className="flex items-center gap-2 flex-wrap">
            <Tag className="h-4 w-4 text-surface-400 flex-shrink-0" />
            {categories.map((cat) => (
              <span key={cat} className="inline-flex items-center px-2 py-0.5 rounded-md text-xs bg-surface-100 dark:bg-surface-800 text-surface-600 dark:text-surface-400">
                {cat}
              </span>
            ))}
          </div>
        )}

        {/* DOI / Journal Ref */}
        {(paper.doi || paper.journal_ref) && (
          <div className="flex flex-wrap items-center gap-3 text-sm text-surface-500">
            {paper.doi && (
              <a
                href={`https://doi.org/${paper.doi}`}
                target="_blank"
                rel="noopener noreferrer"
                className="hover:text-primary-600 dark:hover:text-primary-400 transition-colors underline decoration-dotted"
              >
                DOI: {paper.doi}
              </a>
            )}
            {paper.journal_ref && (
              <span className="italic">{paper.journal_ref}</span>
            )}
          </div>
        )}

        {/* Action buttons — links go directly to arXiv */}
        <div className="flex flex-wrap items-center gap-3 pt-2">
          {isArxiv && (
            <>
              <a
                href={getArxivPdfUrl(paper.external_id)}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-primary-600 hover:bg-primary-700 text-white font-medium transition-colors"
              >
                <FileText className="h-5 w-5" />
                Read PDF
              </a>

              <a
                href={getArxivHtmlUrl(paper.external_id)}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 px-4 py-2.5 rounded-xl border border-surface-300 dark:border-surface-700 text-surface-700 dark:text-surface-300 font-medium hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors"
              >
                <BookOpen className="h-4 w-4" />
                Read HTML
              </a>

              <a
                href={getArxivAbsUrl(paper.external_id)}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 px-4 py-2.5 rounded-xl border border-surface-300 dark:border-surface-700 text-surface-700 dark:text-surface-300 font-medium hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors"
              >
                <ExternalLink className="h-4 w-4" />
                View on arXiv
              </a>
            </>
          )}

          {!isArxiv && paper.pdf_url && (
            <a
              href={paper.pdf_url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-primary-600 hover:bg-primary-700 text-white font-medium transition-colors"
            >
              <FileText className="h-5 w-5" />
              Read PDF
            </a>
          )}

          {isAuthenticated && (
            <>
              {isSaved ? (
                <button
                  onClick={handleRemove}
                  className="flex items-center gap-2 px-4 py-2.5 rounded-xl border border-green-300 dark:border-green-700 text-green-700 dark:text-green-300 font-medium hover:bg-green-50 dark:hover:bg-green-950 transition-colors"
                >
                  <Check className="h-4 w-4" />
                  In Library
                </button>
              ) : (
                <button
                  onClick={handleSave}
                  className="flex items-center gap-2 px-4 py-2.5 rounded-xl border border-surface-300 dark:border-surface-700 text-surface-700 dark:text-surface-300 font-medium hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors"
                >
                  <Plus className="h-4 w-4" />
                  Save to Library
                </button>
              )}

              <button
                onClick={handleBookmarkToggle}
                className={`flex items-center gap-2 px-4 py-2.5 rounded-xl border font-medium transition-colors ${
                  isBookmarked
                    ? 'border-primary-300 dark:border-primary-700 text-primary-700 dark:text-primary-300 hover:bg-primary-50 dark:hover:bg-primary-950'
                    : 'border-surface-300 dark:border-surface-700 text-surface-700 dark:text-surface-300 hover:bg-surface-50 dark:hover:bg-surface-800'
                }`}
              >
                {isBookmarked ? <BookmarkCheck className="h-4 w-4" /> : <Bookmark className="h-4 w-4" />}
                {isBookmarked ? 'Bookmarked' : 'Bookmark'}
              </button>
            </>
          )}
        </div>
      </div>

      {/* Abstract */}
      {paper.abstract && (
        <div className="mt-8 pt-8 border-t border-surface-200 dark:border-surface-800">
          <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-4">Abstract</h2>
          <p className="text-surface-700 dark:text-surface-300 leading-relaxed whitespace-pre-wrap">
            {paper.abstract}
          </p>
        </div>
      )}

      {/* Notes */}
      {userPaper?.notes && (
        <div className="mt-8 pt-8 border-t border-surface-200 dark:border-surface-800">
          <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-4">Your Notes</h2>
          <p className="text-surface-700 dark:text-surface-300 leading-relaxed whitespace-pre-wrap bg-primary-50/50 dark:bg-primary-950/30 rounded-xl p-4 border border-primary-100 dark:border-primary-900">
            {userPaper.notes}
          </p>
        </div>
      )}
    </div>
  );
}
