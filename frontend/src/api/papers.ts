import { api } from './client';
import type { Paper, SearchResult, CategoryInfo, DiscoverResult } from '../types';

export const papersApi = {
  search(
    query: string,
    source?: string,
    limit = 20,
    offset = 0,
    sort = 'relevance',
    categories?: string[],
  ): Promise<SearchResult> {
    const params: Record<string, string | number> = {
      q: query,
      source: source || '',
      limit,
      offset,
      sort,
    };
    if (categories && categories.length > 0) {
      params.categories = categories.join(',');
    }
    return api.get('/api/v1/papers/search', params);
  },

  getById(id: string): Promise<Paper> {
    return api.get(`/api/v1/papers/${id}`);
  },

  getCategories(): Promise<CategoryInfo[]> {
    return api.get('/api/v1/papers/categories');
  },

  getGroupedCategories(): Promise<Record<string, CategoryInfo[]>> {
    return api.get('/api/v1/papers/categories/grouped');
  },

  getDiscover(seed?: string): Promise<DiscoverResult> {
    const params: Record<string, string | number> = {};
    if (seed) params.seed = seed;
    return api.get('/api/v1/discover', params);
  },
};
