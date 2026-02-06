import { api } from './client';
import type { AuthResponse, TokenPair } from '../types';

export const authApi = {
  register(email: string, password: string, name: string): Promise<AuthResponse> {
    return api.post('/api/v1/auth/register', { email, password, name });
  },

  login(email: string, password: string): Promise<AuthResponse> {
    return api.post('/api/v1/auth/login', { email, password });
  },

  googleLogin(accessToken: string): Promise<AuthResponse> {
    return api.post('/api/v1/auth/google', { access_token: accessToken });
  },

  refresh(refreshToken: string): Promise<TokenPair> {
    return api.post('/api/v1/auth/refresh', { refresh_token: refreshToken });
  },

  logout(refreshToken: string): Promise<void> {
    return api.post('/api/v1/auth/logout', { refresh_token: refreshToken });
  },
};
