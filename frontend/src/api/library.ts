import { api } from './client';
import type { UserPaper, LibraryResult } from '../types';

export const libraryApi = {
  getLibrary(status?: string, limit = 20, offset = 0): Promise<LibraryResult> {
    return api.get('/api/v1/library', { status: status || '', limit, offset });
  },

  savePaper(paperId: string): Promise<UserPaper> {
    return api.post(`/api/v1/library/${paperId}`);
  },

  removePaper(paperId: string): Promise<void> {
    return api.delete(`/api/v1/library/${paperId}`);
  },

  updatePaper(paperId: string, data: { status?: string; reading_progress?: number; notes?: string }): Promise<UserPaper> {
    return api.patch(`/api/v1/library/${paperId}`, data);
  },

  getBookmarks(limit = 20, offset = 0): Promise<LibraryResult> {
    return api.get('/api/v1/bookmarks', { limit, offset });
  },

  bookmarkPaper(paperId: string): Promise<UserPaper> {
    return api.post(`/api/v1/bookmarks/${paperId}`);
  },

  unbookmarkPaper(paperId: string): Promise<void> {
    return api.delete(`/api/v1/bookmarks/${paperId}`);
  },
};
