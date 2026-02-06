import { api } from './client';
import type { Paper, SearchResult, HTMLURLResponse } from '../types';

export const papersApi = {
  search(query: string, source?: string, limit = 20, offset = 0, sort = 'relevance'): Promise<SearchResult> {
    return api.get('/api/v1/papers/search', { q: query, source: source || '', limit, offset, sort });
  },

  getById(id: string): Promise<Paper> {
    return api.get(`/api/v1/papers/${id}`);
  },

  // Returns the direct source PDF URL (arXiv, etc.) — we link to the source
  // per arXiv Terms of Use rather than proxying through our backend.
  getPdfUrl(paper: { source: string; external_id: string; pdf_url: string }): string {
    if (paper.source === 'arxiv') {
      return `https://arxiv.org/pdf/${paper.external_id}`;
    }
    return paper.pdf_url;
  },

  getHtmlUrl(id: string): Promise<HTMLURLResponse> {
    return api.get(`/api/v1/papers/${id}/html-url`);
  },
};
