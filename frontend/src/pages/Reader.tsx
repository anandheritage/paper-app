import { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft, FileText, FileType, Bookmark, BookmarkCheck,
  StickyNote, X, Moon, Sun, ExternalLink
} from 'lucide-react';
import toast from 'react-hot-toast';
import { papersApi } from '../api/papers';
import { libraryApi } from '../api/library';
import { useAuthStore } from '../stores/authStore';
import { useThemeStore } from '../stores/themeStore';
import type { Paper, UserPaper } from '../types';

type ViewMode = 'html' | 'pdf';

export default function Reader() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { isDark, toggle: toggleTheme } = useThemeStore();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);

  const [viewMode, setViewMode] = useState<ViewMode>('html');
  const [notesOpen, setNotesOpen] = useState(false);
  const [notes, setNotes] = useState('');
  const [htmlUrl, setHtmlUrl] = useState<string | null>(null);
  const [htmlError, setHtmlError] = useState(false);

  const progressTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [readingProgress, setReadingProgress] = useState(0);

  // Fetch paper data
  const { data: paper } = useQuery({
    queryKey: ['paper', id],
    queryFn: () => papersApi.getById(id!),
    enabled: !!id,
  });

  // Fetch library data only if authenticated
  const { data: libraryData } = useQuery({
    queryKey: ['library', ''],
    queryFn: () => libraryApi.getLibrary('', 100, 0),
    enabled: isAuthenticated,
  });

  const userPaper: UserPaper | undefined = libraryData?.papers?.find((up) => up.paper_id === id);
  const isBookmarked = userPaper?.is_bookmarked ?? false;

  // Initialize notes from user paper
  useEffect(() => {
    if (userPaper?.notes) {
      setNotes(userPaper.notes);
    }
    if (userPaper?.reading_progress) {
      setReadingProgress(userPaper.reading_progress);
    }
  }, [userPaper]);

  // Fetch HTML URL
  useEffect(() => {
    if (id) {
      papersApi.getHtmlUrl(id)
        .then((res) => {
          if (res.html_url) {
            setHtmlUrl(res.html_url);
          } else {
            setHtmlError(true);
            setViewMode('pdf');
          }
        })
        .catch(() => {
          setHtmlError(true);
          setViewMode('pdf');
        });
    }
  }, [id]);

  // Auto-save reading progress (only when authenticated)
  const updateMutation = useMutation({
    mutationFn: (data: { status?: string; reading_progress?: number; notes?: string }) =>
      libraryApi.updatePaper(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['library'] });
    },
  });

  // Save paper to library on mount (only when authenticated)
  const saveMutation = useMutation({
    mutationFn: libraryApi.savePaper,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['library'] });
      updateMutation.mutate({ status: 'reading' });
    },
  });

  useEffect(() => {
    if (!isAuthenticated || !id || !libraryData) return;
    if (!userPaper) {
      saveMutation.mutate(id);
    } else if (userPaper.status === 'saved') {
      updateMutation.mutate({ status: 'reading' });
    }
  }, [id, libraryData, userPaper, isAuthenticated]);

  // Track reading progress by scroll
  useEffect(() => {
    const handleScroll = () => {
      const scrollTop = window.scrollY;
      const docHeight = document.documentElement.scrollHeight - window.innerHeight;
      if (docHeight > 0) {
        const progress = Math.round((scrollTop / docHeight) * 100);
        setReadingProgress(Math.max(readingProgress, progress));
      }
    };

    window.addEventListener('scroll', handleScroll, { passive: true });
    return () => window.removeEventListener('scroll', handleScroll);
  }, [readingProgress]);

  // Auto-save progress every 30 seconds (only when authenticated)
  useEffect(() => {
    if (!isAuthenticated) return;

    progressTimerRef.current = setInterval(() => {
      if (readingProgress > 0 && id) {
        updateMutation.mutate({ reading_progress: readingProgress });
      }
    }, 30000);

    return () => {
      if (progressTimerRef.current) clearInterval(progressTimerRef.current);
      if (readingProgress > 0 && id) {
        libraryApi.updatePaper(id, { reading_progress: readingProgress }).catch(() => {});
      }
    };
  }, [readingProgress, id, isAuthenticated]);

  // Bookmark toggle
  const bookmarkMutation = useMutation({
    mutationFn: async () => {
      if (isBookmarked) {
        await libraryApi.unbookmarkPaper(id!);
      } else {
        await libraryApi.bookmarkPaper(id!);
      }
    },
    onSuccess: () => {
      toast.success(isBookmarked ? 'Bookmark removed' : 'Bookmarked');
      queryClient.invalidateQueries({ queryKey: ['library'] });
      queryClient.invalidateQueries({ queryKey: ['bookmarks'] });
    },
  });

  const handleBookmark = () => {
    if (!isAuthenticated) {
      toast.error('Sign in to bookmark papers');
      return;
    }
    bookmarkMutation.mutate();
  };

  // Save notes
  const saveNotes = useCallback(() => {
    if (!isAuthenticated) {
      toast.error('Sign in to save notes');
      return;
    }
    if (id && notes !== (userPaper?.notes || '')) {
      updateMutation.mutate({ notes });
      toast.success('Notes saved');
    }
  }, [id, notes, userPaper, isAuthenticated]);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLTextAreaElement) return;

      switch (e.key) {
        case 'b':
          handleBookmark();
          break;
        case 'n':
          setNotesOpen((prev) => !prev);
          break;
        case 'h':
          if (!htmlError) setViewMode('html');
          break;
        case 'p':
          setViewMode('pdf');
          break;
        case 'Escape':
          if (notesOpen) setNotesOpen(false);
          else navigate(-1);
          break;
        case 'd':
          toggleTheme();
          break;
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [viewMode, notesOpen, htmlError, isBookmarked, isAuthenticated]);

  // Build the direct source PDF URL (loads from arXiv, not our server)
  const pdfUrl = paper ? papersApi.getPdfUrl(paper) : '';

  const getSourceLabel = (source?: string) => {
    switch (source) {
      case 'arxiv': return 'arXiv';
      case 'pubmed': return 'PubMed';
      case 'openalex': return 'OpenAlex';
      default: return 'Paper';
    }
  };

  return (
    <div className="fixed inset-0 bg-white dark:bg-surface-950 flex flex-col z-50">
      {/* Reading progress bar */}
      <div className="absolute top-0 left-0 right-0 h-1 bg-surface-100 dark:bg-surface-900 z-50">
        <div
          className="h-full bg-primary-500 transition-all duration-300"
          style={{ width: `${readingProgress}%` }}
        />
      </div>

      {/* Top toolbar */}
      <header className="flex items-center justify-between px-4 py-2 border-b border-surface-200 dark:border-surface-800 bg-white/90 dark:bg-surface-950/90 backdrop-blur-lg z-40">
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate(-1)}
            className="p-2 rounded-lg text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800 transition-colors"
            title="Back (Esc)"
          >
            <ArrowLeft className="h-5 w-5" />
          </button>
          <div className="hidden sm:block max-w-md">
            <h1 className="text-sm font-medium text-surface-900 dark:text-surface-100 truncate">
              {paper?.title || 'Loading...'}
            </h1>
            <p className="text-xs text-surface-500">
              {getSourceLabel(paper?.source)} &middot; {paper?.external_id}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-1">
          {/* View mode toggle */}
          <div className="flex gap-0.5 p-0.5 bg-surface-100 dark:bg-surface-800 rounded-lg">
            <button
              onClick={() => !htmlError && setViewMode('html')}
              disabled={htmlError}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                viewMode === 'html'
                  ? 'bg-white dark:bg-surface-700 text-surface-900 dark:text-surface-100 shadow-sm'
                  : 'text-surface-500 hover:text-surface-700 dark:hover:text-surface-300'
              } ${htmlError ? 'opacity-40 cursor-not-allowed' : ''}`}
              title="HTML mode (H)"
            >
              <FileText className="h-4 w-4" />
              <span className="hidden sm:inline">Read</span>
            </button>
            <button
              onClick={() => setViewMode('pdf')}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                viewMode === 'pdf'
                  ? 'bg-white dark:bg-surface-700 text-surface-900 dark:text-surface-100 shadow-sm'
                  : 'text-surface-500 hover:text-surface-700 dark:hover:text-surface-300'
              }`}
              title="PDF mode (P)"
            >
              <FileType className="h-4 w-4" />
              <span className="hidden sm:inline">PDF</span>
            </button>
          </div>

          <div className="w-px h-6 bg-surface-200 dark:bg-surface-700 mx-1" />

          {/* Open on source site */}
          {pdfUrl && (
            <a
              href={pdfUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="p-2 rounded-lg text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800 transition-colors"
              title="Open on source site"
            >
              <ExternalLink className="h-4 w-4" />
            </a>
          )}

          {/* Theme toggle */}
          <button
            onClick={toggleTheme}
            className="p-2 rounded-lg text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800 transition-colors"
            title="Toggle dark mode (D)"
          >
            {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </button>

          {/* Bookmark (auth-gated) */}
          <button
            onClick={handleBookmark}
            className={`p-2 rounded-lg transition-colors ${
              isBookmarked
                ? 'text-primary-600 dark:text-primary-400 bg-primary-50 dark:bg-primary-950'
                : 'text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800'
            }`}
            title="Bookmark (B)"
          >
            {isBookmarked ? <BookmarkCheck className="h-5 w-5" /> : <Bookmark className="h-5 w-5" />}
          </button>

          {/* Notes */}
          <button
            onClick={() => setNotesOpen(!notesOpen)}
            className={`p-2 rounded-lg transition-colors ${
              notesOpen
                ? 'text-primary-600 dark:text-primary-400 bg-primary-50 dark:bg-primary-950'
                : 'text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800'
            }`}
            title="Notes (N)"
          >
            <StickyNote className="h-5 w-5" />
          </button>
        </div>
      </header>

      {/* Main content area */}
      <div className="flex-1 flex overflow-hidden relative">
        <div className={`flex-1 overflow-auto transition-all duration-300 ${notesOpen ? 'mr-80' : ''}`}>
          {viewMode === 'html' ? (
            <HTMLReader htmlUrl={htmlUrl} paper={paper || null} />
          ) : (
            <PDFReader pdfUrl={pdfUrl} paper={paper || null} />
          )}
        </div>

        {/* Notes panel */}
        <div
          className={`absolute top-0 right-0 bottom-0 w-80 bg-white dark:bg-surface-900 border-l border-surface-200 dark:border-surface-800 flex flex-col transition-transform duration-300 ${
            notesOpen ? 'translate-x-0' : 'translate-x-full'
          }`}
        >
          <div className="flex items-center justify-between px-4 py-3 border-b border-surface-200 dark:border-surface-800">
            <h3 className="font-semibold text-surface-900 dark:text-surface-100">Notes</h3>
            <button
              onClick={() => setNotesOpen(false)}
              className="p-1 rounded-lg text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
          <div className="flex-1 p-4">
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder={isAuthenticated ? 'Write your notes here...' : 'Sign in to save notes'}
              disabled={!isAuthenticated}
              className="w-full h-full resize-none bg-transparent text-surface-800 dark:text-surface-200 placeholder:text-surface-400 focus:outline-none text-sm leading-relaxed disabled:opacity-60"
            />
          </div>
          <div className="px-4 py-3 border-t border-surface-200 dark:border-surface-800">
            <button
              onClick={saveNotes}
              disabled={!isAuthenticated}
              className="w-full py-2 px-4 rounded-lg bg-primary-600 hover:bg-primary-700 text-white text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isAuthenticated ? 'Save Notes' : 'Sign in to save'}
            </button>
          </div>
        </div>
      </div>

      {/* Keyboard shortcuts hint */}
      <div className="hidden lg:block fixed bottom-4 left-4 text-xs text-surface-400 dark:text-surface-600 space-y-0.5">
        <p><kbd className="px-1 py-0.5 rounded border border-surface-300 dark:border-surface-700 text-[10px]">H</kbd> HTML mode</p>
        <p><kbd className="px-1 py-0.5 rounded border border-surface-300 dark:border-surface-700 text-[10px]">P</kbd> PDF mode</p>
        <p><kbd className="px-1 py-0.5 rounded border border-surface-300 dark:border-surface-700 text-[10px]">B</kbd> Bookmark</p>
        <p><kbd className="px-1 py-0.5 rounded border border-surface-300 dark:border-surface-700 text-[10px]">N</kbd> Notes</p>
        <p><kbd className="px-1 py-0.5 rounded border border-surface-300 dark:border-surface-700 text-[10px]">D</kbd> Dark mode</p>
      </div>
    </div>
  );
}

// HTML Reader component — iframe loads directly from arXiv/ar5iv
function HTMLReader({ htmlUrl, paper }: { htmlUrl: string | null; paper: Paper | null }) {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const iframeRef = useRef<HTMLIFrameElement>(null);

  useEffect(() => {
    if (!htmlUrl) return;
    setLoading(true);
    setLoadError(false);
    const timer = setTimeout(() => {
      if (loading) {
        setLoadError(true);
        setLoading(false);
      }
    }, 15000);
    return () => clearTimeout(timer);
  }, [htmlUrl]);

  if (!htmlUrl) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <FileText className="h-16 w-16 mx-auto text-surface-300 dark:text-surface-700 mb-4" />
          <p className="text-surface-500">HTML version not available</p>
          <p className="text-sm text-surface-400 mt-1">Switch to PDF mode to read this paper</p>
        </div>
      </div>
    );
  }

  if (loadError) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <FileText className="h-16 w-16 mx-auto text-surface-300 dark:text-surface-700 mb-4" />
          <p className="text-surface-500">HTML version couldn't be loaded</p>
          <p className="text-sm text-surface-400 mt-1 mb-3">The paper may not have an HTML version, or the service is temporarily down</p>
          <a
            href={htmlUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-block px-4 py-2 rounded-lg bg-primary-600 hover:bg-primary-700 text-white text-sm font-medium transition-colors"
          >
            Open HTML in new tab
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full relative">
      {loading && (
        <div className="absolute inset-0 flex items-center justify-center bg-white dark:bg-surface-950 z-10">
          <div className="text-center">
            <div className="w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
            <p className="text-sm text-surface-500">Loading paper...</p>
          </div>
        </div>
      )}
      <iframe
        ref={iframeRef}
        src={htmlUrl}
        className="w-full h-full border-0"
        title={paper?.title || 'Paper'}
        onLoad={() => { setLoading(false); setLoadError(false); }}
        onError={() => { setLoadError(true); setLoading(false); }}
        sandbox="allow-scripts allow-same-origin allow-popups"
      />
    </div>
  );
}

// PDF Reader component — iframe loads PDF directly from arXiv (legal compliance)
// Per arXiv ToU: "Direct users to arXiv.org to retrieve e-print content"
function PDFReader({ pdfUrl, paper }: { pdfUrl: string; paper: Paper | null }) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const iframeRef = useRef<HTMLIFrameElement>(null);

  useEffect(() => {
    if (!pdfUrl) return;
    setLoading(true);
    setError(false);
    // Timeout fallback in case the iframe never fires onLoad
    const timer = setTimeout(() => {
      setLoading(false);
    }, 10000);
    return () => clearTimeout(timer);
  }, [pdfUrl]);

  if (!pdfUrl) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <FileText className="h-16 w-16 mx-auto text-surface-300 dark:text-surface-700 mb-4" />
          <p className="text-surface-500">PDF not available</p>
          <p className="text-sm text-surface-400 mt-1">This paper doesn't have a PDF URL</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <FileText className="h-16 w-16 mx-auto text-surface-300 dark:text-surface-700 mb-4" />
          <p className="text-surface-500">PDF couldn't be embedded</p>
          <p className="text-sm text-surface-400 mt-1 mb-3">You can still read it on the source site</p>
          <a
            href={pdfUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-block px-4 py-2 rounded-lg bg-primary-600 hover:bg-primary-700 text-white text-sm font-medium transition-colors"
          >
            Open PDF on {paper?.source === 'arxiv' ? 'arXiv' : 'source'}
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full relative">
      {loading && (
        <div className="absolute inset-0 flex items-center justify-center bg-white dark:bg-surface-950 z-10">
          <div className="text-center">
            <div className="w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
            <p className="text-sm text-surface-500">Loading PDF from {paper?.source === 'arxiv' ? 'arXiv' : 'source'}...</p>
          </div>
        </div>
      )}
      <iframe
        ref={iframeRef}
        src={pdfUrl}
        className="w-full h-full border-0"
        title={paper?.title || 'PDF'}
        onLoad={() => { setLoading(false); setError(false); }}
        onError={() => { setError(true); setLoading(false); }}
      />
    </div>
  );
}
