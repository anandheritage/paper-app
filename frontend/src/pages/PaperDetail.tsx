import { useParams, useNavigate } from 'react-router-dom';
import { useRef, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Calendar, Users, ExternalLink, BookOpen, Bookmark, BookmarkCheck, Plus, Check, ArrowLeft, FileText, Tag, Quote, Award, Globe, Share2 } from 'lucide-react';
import toast from 'react-hot-toast';
import { papersApi } from '../api/papers';
import { libraryApi } from '../api/library';
import { useAuthStore } from '../stores/authStore';
import { PaperDetailSkeleton } from '../components/Skeleton';

function formatCitations(count: number): string {
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}k`;
  return String(count);
}

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
    queryKey: ['library', '', 'all'],
    queryFn: () => libraryApi.getLibrary('', 1000, 0),
    enabled: isAuthenticated,
  });

  // Match by paper_id (PG UUID) OR paper.external_id (for papers opened from search with corpusid)
  const userPaper = libraryData?.papers?.find(
    (up) => up.paper_id === id || up.paper?.external_id === id || up.paper?.id === id
  );
  const isSaved = !!userPaper;
  const isBookmarked = userPaper?.is_bookmarked ?? false;

  // Auto-set status to "reading" when a saved paper is viewed,
  // and update last_read_at for papers already in "reading" status.
  const hasAutoMarkedRef = useRef(false);
  const lastAutoMarkedIdRef = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (lastAutoMarkedIdRef.current !== id) {
      hasAutoMarkedRef.current = false;
      lastAutoMarkedIdRef.current = id;
    }
  }, [id]);
  useEffect(() => {
    if (!userPaper || hasAutoMarkedRef.current) return;
    hasAutoMarkedRef.current = true;

    if (userPaper.status === 'saved') {
      // Transition saved → reading (also updates last_read_at on backend)
      libraryApi.updatePaper(userPaper.paper_id, { status: 'reading' })
        .then(() => queryClient.invalidateQueries({ queryKey: ['library'] }))
        .catch(() => {});
    } else if (userPaper.status === 'reading') {
      // Paper already in reading — touch last_read_at by re-sending current status
      libraryApi.updatePaper(userPaper.paper_id, { status: 'reading' })
        .then(() => queryClient.invalidateQueries({ queryKey: ['library'] }))
        .catch(() => {});
    }
  }, [userPaper?.paper_id, userPaper?.status, queryClient]);

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
    // Use the user_papers paper_id when available (correct PG UUID)
    removeMutation.mutate(userPaper?.paper_id ?? paper!.id);
  };

  const handleBookmarkToggle = () => {
    if (!isAuthenticated) { toast.error('Sign in to bookmark papers'); return; }
    const paperId = userPaper?.paper_id ?? paper!.id;
    if (isBookmarked) unbookmarkMutation.mutate(paperId);
    else bookmarkMutation.mutate(paperId);
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
    : paper.year
    ? String(paper.year)
    : null;

  const categories: string[] = paper.categories ?? [];
  const citationCount = paper.citation_count ?? 0;
  const referenceCount = paper.reference_count ?? 0;
  const influentialCount = paper.influential_citation_count ?? 0;
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
          {paper.external_id && (
            <span className="text-sm text-surface-400">{paper.external_id}</span>
          )}
          {paper.venue && (
            <span className="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-purple-50 text-purple-700 dark:bg-purple-950 dark:text-purple-300">
              {paper.venue}
            </span>
          )}
          {paper.is_open_access && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs font-medium bg-green-50 text-green-700 dark:bg-green-950 dark:text-green-300">
              <Globe className="h-3 w-3" />
              Open Access
            </span>
          )}
        </div>

        <h1 className="text-2xl sm:text-3xl font-bold text-surface-900 dark:text-surface-100 leading-tight">
          {paper.title}
        </h1>

        {/* Authors */}
        {authors.length > 0 && (
          <div className="flex items-start gap-2">
            <Users className="h-5 w-5 text-surface-400 mt-0.5 flex-shrink-0" />
            <div className="flex flex-wrap gap-x-2 gap-y-1">
              {authors.map((author: { name: string; authorId?: string }, i: number) => (
                <span key={i} className="text-sm text-surface-600 dark:text-surface-400">
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

        {/* Citation stats */}
        {(citationCount > 0 || referenceCount > 0) && (
          <div className="flex items-center gap-4 flex-wrap">
            {citationCount > 0 && (
              <div className="flex items-center gap-1.5 text-sm">
                <Quote className="h-4 w-4 text-amber-500" />
                <span className="font-semibold text-amber-600 dark:text-amber-400">{formatCitations(citationCount)}</span>
                <span className="text-surface-500">citations</span>
              </div>
            )}
            {influentialCount > 0 && (
              <div className="flex items-center gap-1.5 text-sm">
                <Award className="h-4 w-4 text-orange-500" />
                <span className="font-semibold text-orange-600 dark:text-orange-400">{formatCitations(influentialCount)}</span>
                <span className="text-surface-500">influential</span>
              </div>
            )}
            {referenceCount > 0 && (
              <div className="flex items-center gap-1.5 text-sm text-surface-500">
                <span className="font-medium">{referenceCount}</span> references
              </div>
            )}
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

        {/* Publication types */}
        {paper.publication_types && paper.publication_types.length > 0 && (
          <div className="flex items-center gap-2 flex-wrap">
            <FileText className="h-4 w-4 text-surface-400 flex-shrink-0" />
            {paper.publication_types.map((pt) => (
              <span key={pt} className="inline-flex items-center px-2 py-0.5 rounded-md text-xs bg-indigo-50 dark:bg-indigo-950 text-indigo-600 dark:text-indigo-400">
                {pt}
              </span>
            ))}
          </div>
        )}

        {/* DOI / Journal */}
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

        {/* Action buttons */}
        <div className="flex flex-wrap items-center gap-3 pt-2">
          {isArxiv && paper.external_id && (
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

          {paper.s2_url && (
            <a
              href={paper.s2_url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 px-4 py-2.5 rounded-xl border border-surface-300 dark:border-surface-700 text-surface-700 dark:text-surface-300 font-medium hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors"
            >
              <ExternalLink className="h-4 w-4" />
              Semantic Scholar
            </a>
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

              {userPaper && (
                <select
                  value={userPaper.status}
                  onChange={(e) => {
                    libraryApi.updatePaper(userPaper.paper_id, { status: e.target.value })
                      .then(() => {
                        toast.success('Status updated');
                        queryClient.invalidateQueries({ queryKey: ['library'] });
                      })
                      .catch(() => toast.error('Failed to update status'));
                  }}
                  onClick={(e) => e.stopPropagation()}
                  className="px-3 py-2.5 rounded-xl border border-surface-300 dark:border-surface-700 bg-white dark:bg-surface-900 text-surface-700 dark:text-surface-300 text-sm font-medium transition-colors cursor-pointer"
                >
                  <option value="saved">Saved</option>
                  <option value="reading">Reading</option>
                  <option value="finished">Finished</option>
                </select>
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

          {/* WhatsApp Share */}
          <a
            href={`https://wa.me/?text=${encodeURIComponent(
              `${paper.title}\n\nhttps://dapapers.com/paper/${paper.id}`
            )}`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-4 py-2.5 rounded-xl border border-green-300 dark:border-green-800 text-green-600 dark:text-green-400 font-medium hover:bg-green-50 dark:hover:bg-green-950 transition-colors"
          >
            <Share2 className="h-4 w-4" />
            WhatsApp
          </a>
        </div>
      </div>

      {/* TLDR */}
      {paper.tldr && (
        <div className="mt-8 pt-8 border-t border-surface-200 dark:border-surface-800">
          <h2 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-3">TL;DR</h2>
          <p className="text-surface-700 dark:text-surface-300 leading-relaxed bg-blue-50/50 dark:bg-blue-950/30 rounded-xl p-4 border border-blue-100 dark:border-blue-900">
            {paper.tldr}
          </p>
        </div>
      )}

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
