import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User, TokenPair } from '../types';

interface AuthState {
  user: User | null;
  tokens: TokenPair | null;
  isAuthenticated: boolean;
  _hasHydrated: boolean;
  setAuth: (user: User, tokens: TokenPair) => void;
  setTokens: (tokens: TokenPair) => void;
  logout: () => void;
  setHasHydrated: (v: boolean) => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      tokens: null,
      isAuthenticated: false,
      _hasHydrated: false,
      setAuth: (user, tokens) =>
        set({ user, tokens, isAuthenticated: true }),
      setTokens: (tokens) =>
        set({ tokens }),
      logout: () =>
        set({ user: null, tokens: null, isAuthenticated: false }),
      setHasHydrated: (v) => set({ _hasHydrated: v }),
    }),
    {
      name: 'paper-auth',
      partialize: (state) => ({
        user: state.user,
        tokens: state.tokens,
        isAuthenticated: state.isAuthenticated,
      }),
      onRehydrateStorage: () => {
        return (_state, _error) => {
          // Called after rehydration completes
          useAuthStore.setState({ _hasHydrated: true });
        };
      },
    }
  )
);
