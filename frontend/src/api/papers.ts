import { api } from './client';
import type { Paper, SearchResult, HTMLURLResponse } from '../types';

export const papersApi = {
  search(query: string, source?: string, limit = 20, offset = 0): Promise<SearchResult> {
    return api.get('/api/v1/papers/search', { q: query, source: source || '', limit, offset });
  },

  getById(id: string): Promise<Paper> {
    return api.get(`/api/v1/papers/${id}`);
  },

  getPdfUrl(id: string): string {
    const base = import.meta.env.VITE_API_URL || '';
    return `${base}/api/v1/papers/${id}/pdf`;
  },

  getHtmlUrl(id: string): Promise<HTMLURLResponse> {
    return api.get(`/api/v1/papers/${id}/html-url`);
  },
};
