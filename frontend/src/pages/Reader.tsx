import { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft, FileText, FileType, Bookmark, BookmarkCheck,
  StickyNote, X, ChevronLeft, ChevronRight, ZoomIn, ZoomOut,
  Maximize, Moon, Sun, Minus
} from 'lucide-react';
import toast from 'react-hot-toast';
import { papersApi } from '../api/papers';
import { libraryApi } from '../api/library';
import { useThemeStore } from '../stores/themeStore';
import type { Paper, UserPaper } from '../types';

type ViewMode = 'html' | 'pdf';

export default function Reader() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { isDark, toggle: toggleTheme } = useThemeStore();

  const [viewMode, setViewMode] = useState<ViewMode>('html');
  const [notesOpen, setNotesOpen] = useState(false);
  const [notes, setNotes] = useState('');
  const [htmlUrl, setHtmlUrl] = useState<string | null>(null);
  const [htmlError, setHtmlError] = useState(false);

  // PDF state
  const [pdfPage, setPdfPage] = useState(1);
  const [pdfTotalPages, setPdfTotalPages] = useState(0);
  const [pdfScale, setPdfScale] = useState(1.2);
  const progressTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [readingProgress, setReadingProgress] = useState(0);

  // Fetch paper data
  const { data: paper } = useQuery({
    queryKey: ['paper', id],
    queryFn: () => papersApi.getById(id!),
    enabled: !!id,
  });

  // Fetch library data (to get user paper with notes)
  const { data: libraryData } = useQuery({
    queryKey: ['library', ''],
    queryFn: () => libraryApi.getLibrary('', 100, 0),
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
        .then((res) => setHtmlUrl(res.html_url))
        .catch(() => {
          setHtmlError(true);
          setViewMode('pdf');
        });
    }
  }, [id]);

  // Auto-save reading progress
  const updateMutation = useMutation({
    mutationFn: (data: { status?: string; reading_progress?: number; notes?: string }) =>
      libraryApi.updatePaper(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['library'] });
    },
  });

  // Save paper to library and set as reading on mount
  const saveMutation = useMutation({
    mutationFn: libraryApi.savePaper,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['library'] });
      updateMutation.mutate({ status: 'reading' });
    },
  });

  useEffect(() => {
    if (id && libraryData && !userPaper) {
      saveMutation.mutate(id);
    } else if (userPaper && userPaper.status === 'saved') {
      updateMutation.mutate({ status: 'reading' });
    }
  }, [id, libraryData, userPaper]);

  // Track reading progress by scroll position
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

  // Auto-save progress every 30 seconds
  useEffect(() => {
    progressTimerRef.current = setInterval(() => {
      if (readingProgress > 0 && id) {
        updateMutation.mutate({ reading_progress: readingProgress });
      }
    }, 30000);

    return () => {
      if (progressTimerRef.current) clearInterval(progressTimerRef.current);
      // Save on unmount
      if (readingProgress > 0 && id) {
        libraryApi.updatePaper(id, { reading_progress: readingProgress }).catch(() => {});
      }
    };
  }, [readingProgress, id]);

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

  // Save notes
  const saveNotes = useCallback(() => {
    if (id && notes !== (userPaper?.notes || '')) {
      updateMutation.mutate({ notes });
      toast.success('Notes saved');
    }
  }, [id, notes, userPaper]);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Don't trigger when typing in notes textarea
      if (e.target instanceof HTMLTextAreaElement) return;

      switch (e.key) {
        case 'b':
          bookmarkMutation.mutate();
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
        case 'ArrowLeft':
          if (viewMode === 'pdf' && pdfPage > 1) setPdfPage(pdfPage - 1);
          break;
        case 'ArrowRight':
          if (viewMode === 'pdf' && pdfPage < pdfTotalPages) setPdfPage(pdfPage + 1);
          break;
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [viewMode, pdfPage, pdfTotalPages, notesOpen, htmlError, isBookmarked]);

  const pdfUrl = paper ? papersApi.getPdfUrl(paper.id) : '';

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
              {paper?.source === 'arxiv' ? 'arXiv' : 'PubMed'} &middot; {paper?.external_id}
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

          {/* PDF controls */}
          {viewMode === 'pdf' && (
            <>
              <button
                onClick={() => setPdfScale(Math.max(0.5, pdfScale - 0.2))}
                className="p-2 rounded-lg text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800 transition-colors"
                title="Zoom out"
              >
                <ZoomOut className="h-4 w-4" />
              </button>
              <span className="text-xs text-surface-500 w-12 text-center">{Math.round(pdfScale * 100)}%</span>
              <button
                onClick={() => setPdfScale(Math.min(3, pdfScale + 0.2))}
                className="p-2 rounded-lg text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800 transition-colors"
                title="Zoom in"
              >
                <ZoomIn className="h-4 w-4" />
              </button>
              <button
                onClick={() => setPdfScale(1.0)}
                className="p-2 rounded-lg text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800 transition-colors"
                title="Fit to width"
              >
                <Maximize className="h-4 w-4" />
              </button>
              <div className="w-px h-6 bg-surface-200 dark:bg-surface-700 mx-1" />
            </>
          )}

          {/* Theme toggle */}
          <button
            onClick={toggleTheme}
            className="p-2 rounded-lg text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800 transition-colors"
            title="Toggle dark mode (D)"
          >
            {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </button>

          {/* Bookmark */}
          <button
            onClick={() => bookmarkMutation.mutate()}
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
        {/* Reader content */}
        <div className={`flex-1 overflow-auto transition-all duration-300 ${notesOpen ? 'mr-80' : ''}`}>
          {viewMode === 'html' ? (
            <HTMLReader htmlUrl={htmlUrl} paper={paper || null} />
          ) : (
            <PDFReader
              pdfUrl={pdfUrl}
              page={pdfPage}
              scale={pdfScale}
              onPageChange={setPdfPage}
              onTotalPagesChange={setPdfTotalPages}
              totalPages={pdfTotalPages}
            />
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
              placeholder="Write your notes here..."
              className="w-full h-full resize-none bg-transparent text-surface-800 dark:text-surface-200 placeholder:text-surface-400 focus:outline-none text-sm leading-relaxed"
            />
          </div>
          <div className="px-4 py-3 border-t border-surface-200 dark:border-surface-800">
            <button
              onClick={saveNotes}
              className="w-full py-2 px-4 rounded-lg bg-primary-600 hover:bg-primary-700 text-white text-sm font-medium transition-colors"
            >
              Save Notes
            </button>
          </div>
        </div>
      </div>

      {/* PDF page navigation (bottom bar) */}
      {viewMode === 'pdf' && pdfTotalPages > 0 && (
        <footer className="flex items-center justify-center gap-4 px-4 py-2 border-t border-surface-200 dark:border-surface-800 bg-white/90 dark:bg-surface-950/90 backdrop-blur-lg">
          <button
            onClick={() => setPdfPage(Math.max(1, pdfPage - 1))}
            disabled={pdfPage <= 1}
            className="p-1.5 rounded-lg text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800 disabled:opacity-30 transition-colors"
          >
            <ChevronLeft className="h-5 w-5" />
          </button>
          <span className="text-sm text-surface-600 dark:text-surface-400 min-w-[100px] text-center">
            Page {pdfPage} of {pdfTotalPages}
          </span>
          <button
            onClick={() => setPdfPage(Math.min(pdfTotalPages, pdfPage + 1))}
            disabled={pdfPage >= pdfTotalPages}
            className="p-1.5 rounded-lg text-surface-500 hover:bg-surface-100 dark:hover:bg-surface-800 disabled:opacity-30 transition-colors"
          >
            <ChevronRight className="h-5 w-5" />
          </button>
        </footer>
      )}

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

// HTML Reader component
function HTMLReader({ htmlUrl, paper }: { htmlUrl: string | null; paper: Paper | null }) {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const iframeRef = useRef<HTMLIFrameElement>(null);

  // Timeout fallback: if iframe hasn't loaded after 15s, show error
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

// PDF Reader component using iframe (reliable, no complex library setup)
function PDFReader({
  pdfUrl,
  page,
  scale,
  onPageChange,
  onTotalPagesChange,
  totalPages,
}: {
  pdfUrl: string;
  page: number;
  scale: number;
  onPageChange: (page: number) => void;
  onTotalPagesChange: (total: number) => void;
  totalPages: number;
}) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const pdfDocRef = useRef<any>(null);
  const renderTaskRef = useRef<any>(null);

  // Load PDF using pdfjs-dist
  useEffect(() => {
    let cancelled = false;

    async function loadPdf() {
      try {
        setLoading(true);
        setError(false);

        const pdfjsLib = await import('pdfjs-dist');
        pdfjsLib.GlobalWorkerOptions.workerSrc = `https://cdnjs.cloudflare.com/ajax/libs/pdf.js/${pdfjsLib.version}/pdf.worker.min.mjs`;

        const loadingTask = pdfjsLib.getDocument(pdfUrl);
        const pdf = await loadingTask.promise;

        if (cancelled) return;
        pdfDocRef.current = pdf;
        onTotalPagesChange(pdf.numPages);
        setLoading(false);
      } catch (err) {
        console.error('Failed to load PDF:', err);
        if (!cancelled) {
          setError(true);
          setLoading(false);
        }
      }
    }

    loadPdf();
    return () => { cancelled = true; };
  }, [pdfUrl]);

  // Render current page
  useEffect(() => {
    const pdf = pdfDocRef.current;
    const canvas = canvasRef.current;
    if (!pdf || !canvas) return;

    let cancelled = false;

    async function renderPage() {
      try {
        // Cancel any pending render
        if (renderTaskRef.current) {
          renderTaskRef.current.cancel();
        }

        const pdfPage = await pdf.getPage(page);
        if (cancelled) return;

        const viewport = pdfPage.getViewport({ scale });
        canvas!.height = viewport.height;
        canvas!.width = viewport.width;

        const context = canvas!.getContext('2d');
        if (!context) return;

        const renderTask = pdfPage.render({
          canvasContext: context,
          viewport: viewport,
        });
        renderTaskRef.current = renderTask;

        await renderTask.promise;
      } catch (err: any) {
        if (err?.name !== 'RenderingCancelledException') {
          console.error('Error rendering page:', err);
        }
      }
    }

    renderPage();
    return () => { cancelled = true; };
  }, [page, scale, pdfDocRef.current]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <div className="w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
          <p className="text-sm text-surface-500">Loading PDF...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <Minus className="h-16 w-16 mx-auto text-surface-300 dark:text-surface-700 mb-4 rotate-45" />
          <p className="text-surface-500">Failed to load PDF</p>
          <p className="text-sm text-surface-400 mt-1">The PDF might not be publicly accessible</p>
          {pdfUrl && (
            <a
              href={pdfUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-block mt-3 text-sm text-primary-600 dark:text-primary-400 hover:underline"
            >
              Open PDF in new tab
            </a>
          )}
        </div>
      </div>
    );
  }

  return (
    <div ref={containerRef} className="h-full overflow-auto flex justify-center p-4 bg-surface-100 dark:bg-surface-900">
      <canvas
        ref={canvasRef}
        className="shadow-lg rounded-sm bg-white"
      />
    </div>
  );
}
