export interface User {
  id: string;
  email: string;
  name: string;
  auth_provider: string;
  created_at: string;
  updated_at: string;
}

export interface Paper {
  id: string;
  external_id: string;
  source: string;
  title: string;
  abstract: string;
  authors: Author[];
  published_date: string | null;
  updated_date?: string | null;
  pdf_url: string;
  metadata: Record<string, unknown>;
  created_at: string;
  citation_count?: number;
  primary_category?: string;
  categories?: string[];
  doi?: string;
  journal_ref?: string;
  comments?: string;
  license?: string;
}

export interface Author {
  name: string;
  affiliation?: string;
}

export interface UserPaper {
  id: string;
  user_id: string;
  paper_id: string;
  status: 'saved' | 'reading' | 'finished';
  is_bookmarked: boolean;
  reading_progress: number;
  notes: string;
  tags: string[];
  saved_at: string;
  last_read_at: string | null;
  paper: Paper;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  expires_at: number;
}

export interface AuthResponse {
  user: User;
  tokens: TokenPair;
}

export interface SearchResult {
  papers: Paper[];
  total: number;
  offset: number;
  limit: number;
}

export interface LibraryResult {
  papers: UserPaper[];
  total: number;
  offset: number;
  limit: number;
}

export interface CategoryInfo {
  id: string;
  name: string;
  group: string;
  count: number;
}
