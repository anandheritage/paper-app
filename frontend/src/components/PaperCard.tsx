import { useNavigate } from 'react-router-dom';
import { Calendar, Users, ExternalLink, Bookmark, BookmarkCheck } from 'lucide-react';
import type { Paper } from '../types';

interface PaperCardProps {
  paper: Paper;
  isBookmarked?: boolean;
  onBookmark?: (paperId: string) => void;
  onUnbookmark?: (paperId: string) => void;
  compact?: boolean;
}

function getSourceLabel(source: string): string {
  switch (source) {
    case 'arxiv': return 'arXiv';
    default: return source;
  }
}

export default function PaperCard({ paper, isBookmarked, onBookmark, onUnbookmark, compact }: PaperCardProps) {
  const navigate = useNavigate();

  const authors = Array.isArray(paper.authors)
    ? paper.authors
    : typeof paper.authors === 'string'
    ? (() => { try { return JSON.parse(paper.authors); } catch { return []; } })()
    : [];

  const authorText = authors.length > 3
    ? `${authors.slice(0, 3).map((a: { name: string }) => a.name).join(', ')} +${authors.length - 3} more`
    : authors.map((a: { name: string }) => a.name).join(', ');

  const publishDate = paper.published_date
    ? new Date(paper.published_date).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
    : null;

  return (
    <article
      className="group relative bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-5 hover:border-primary-300 dark:hover:border-primary-700 hover:shadow-lg hover:shadow-primary-100/50 dark:hover:shadow-primary-900/20 transition-all duration-200 cursor-pointer"
      onClick={() => navigate(`/paper/${paper.id}`)}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          {/* Source badge + category + date */}
          <div className="flex items-center gap-2 mb-2 flex-wrap">
            <span className={`inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium ${
              paper.source === 'arxiv'
                ? 'bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-300'
                : 'bg-surface-100 text-surface-700 dark:bg-surface-800 dark:text-surface-300'
            }`}>
              {getSourceLabel(paper.source)}
            </span>
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

          {/* Title */}
          <h3 className="text-base font-semibold text-surface-900 dark:text-surface-100 leading-snug group-hover:text-primary-700 dark:group-hover:text-primary-400 transition-colors line-clamp-2">
            {paper.title}
          </h3>

          {/* Authors */}
          {authorText && (
            <p className="flex items-center gap-1 mt-1.5 text-sm text-surface-500 dark:text-surface-400 truncate">
              <Users className="h-3.5 w-3.5 flex-shrink-0" />
              {authorText}
            </p>
          )}

          {/* Abstract */}
          {!compact && paper.abstract && (
            <p className="mt-2 text-sm text-surface-600 dark:text-surface-400 line-clamp-3 leading-relaxed">
              {paper.abstract}
            </p>
          )}
        </div>

        {/* Actions */}
        <div className="flex flex-col items-center gap-2 flex-shrink-0">
          {(onBookmark || onUnbookmark) && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                if (isBookmarked) onUnbookmark?.(paper.id);
                else onBookmark?.(paper.id);
              }}
              className={`p-2 rounded-lg transition-colors ${
                isBookmarked
                  ? 'text-primary-600 dark:text-primary-400 bg-primary-50 dark:bg-primary-950'
                  : 'text-surface-400 hover:text-primary-600 hover:bg-primary-50 dark:hover:text-primary-400 dark:hover:bg-primary-950'
              }`}
              title={isBookmarked ? 'Remove bookmark' : 'Bookmark'}
            >
              {isBookmarked ? <BookmarkCheck className="h-5 w-5" /> : <Bookmark className="h-5 w-5" />}
            </button>
          )}
          <a
            href={paper.pdf_url}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            className="p-2 rounded-lg text-surface-400 hover:text-surface-600 hover:bg-surface-100 dark:hover:text-surface-300 dark:hover:bg-surface-800 transition-colors"
            title="Open original"
          >
            <ExternalLink className="h-4 w-4" />
          </a>
        </div>
      </div>
    </article>
  );
}
