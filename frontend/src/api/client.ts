import { useAuthStore } from '../stores/authStore';

const API_BASE = import.meta.env.VITE_API_URL || '';

interface RequestOptions extends RequestInit {
  params?: Record<string, string | number>;
}

class ApiClient {
  private baseURL: string;
  // Mutex for token refresh - prevents race condition when multiple
  // requests get 401 simultaneously and all try to refresh
  private refreshPromise: Promise<boolean> | null = null;

  constructor(baseURL: string) {
    this.baseURL = baseURL;
  }

  private getHeaders(): HeadersInit {
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
    };
    const tokens = useAuthStore.getState().tokens;
    if (tokens?.access_token) {
      headers['Authorization'] = `Bearer ${tokens.access_token}`;
    }
    return headers;
  }

  private buildURL(path: string, params?: Record<string, string | number>): string {
    const url = new URL(`${this.baseURL}${path}`, window.location.origin);
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined && value !== null && value !== '') {
          url.searchParams.set(key, String(value));
        }
      });
    }
    return url.toString();
  }

  async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const { params, ...fetchOptions } = options;
    const url = this.buildURL(path, params);

    const response = await fetch(url, {
      ...fetchOptions,
      headers: {
        ...this.getHeaders(),
        ...fetchOptions.headers,
      },
    });

    if (response.status === 401) {
      // Skip refresh for auth endpoints to avoid loops
      if (path.includes('/auth/login') || path.includes('/auth/register') || path.includes('/auth/google')) {
        const errorBody = await response.json().catch(() => ({ error: 'Unauthorized' }));
        throw new ApiError(response.status, errorBody.error || 'Unauthorized');
      }

      // Try to refresh the token (with mutex to prevent race condition)
      const refreshed = await this.tryRefresh();
      if (refreshed) {
        // Retry the request with new token
        const retryResponse = await fetch(url, {
          ...fetchOptions,
          headers: {
            ...this.getHeaders(),
            ...fetchOptions.headers,
          },
        });
        if (!retryResponse.ok) {
          throw new ApiError(retryResponse.status, await retryResponse.text());
        }
        if (retryResponse.status === 204) return undefined as T;
        return retryResponse.json();
      }
      useAuthStore.getState().logout();
      window.location.href = '/login';
      throw new ApiError(401, 'Unauthorized');
    }

    if (!response.ok) {
      const errorBody = await response.json().catch(() => ({ error: 'Unknown error' }));
      throw new ApiError(response.status, errorBody.error || 'Unknown error');
    }

    if (response.status === 204) return undefined as T;
    return response.json();
  }

  private async tryRefresh(): Promise<boolean> {
    // If a refresh is already in progress, wait for it instead of making a new one
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    const tokens = useAuthStore.getState().tokens;
    if (!tokens?.refresh_token) return false;

    // Create the refresh promise so other concurrent 401 handlers can wait on it
    this.refreshPromise = this.doRefresh(tokens.refresh_token);

    try {
      return await this.refreshPromise;
    } finally {
      this.refreshPromise = null;
    }
  }

  private async doRefresh(refreshToken: string): Promise<boolean> {
    try {
      const response = await fetch(this.buildURL('/api/v1/auth/refresh'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refreshToken }),
      });

      if (!response.ok) return false;

      const newTokens = await response.json();
      useAuthStore.getState().setTokens(newTokens);
      return true;
    } catch {
      return false;
    }
  }

  get<T>(path: string, params?: Record<string, string | number>): Promise<T> {
    return this.request<T>(path, { method: 'GET', params });
  }

  post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  patch<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'PATCH',
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  delete<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'DELETE' });
  }
}

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = 'ApiError';
  }
}

export const api = new ApiClient(API_BASE);
