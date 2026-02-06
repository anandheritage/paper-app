import { useGoogleLogin } from '@react-oauth/google';
import { Component, type ReactNode } from 'react';
import toast from 'react-hot-toast';

const hasGoogleOAuth = !!import.meta.env.VITE_GOOGLE_CLIENT_ID;
// #region agent log
const _dbg = (loc: string, msg: string, data: Record<string, unknown>, hId: string) => fetch('http://127.0.0.1:7243/ingest/8d1a3c41-fcb2-4ddc-ae90-28fa3d3a7afb',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:loc,message:msg,data,timestamp:Date.now(),sessionId:'debug-session',hypothesisId:hId})}).catch(()=>{});
_dbg('GoogleLoginButton.tsx:TOP','hasGoogleOAuth check',{hasGoogleOAuth,rawEnvValue:import.meta.env.VITE_GOOGLE_CLIENT_ID ?? 'UNDEFINED'},'A');
// #endregion

class GoogleErrorBoundary extends Component<{children: ReactNode}, {error: Error | null}> {
  state = { error: null as Error | null };
  static getDerivedStateFromError(error: Error) { return { error }; }
  render() { return this.state.error ? null : this.props.children; }
}

function GoogleLoginButtonInner({ onSuccess, disabled }: { onSuccess: (token: string) => void; disabled?: boolean }) {
  const googleLogin = useGoogleLogin({
    onSuccess: (tokenResponse) => {
      // #region agent log
      _dbg('GoogleLoginButton.tsx:onSuccess','implicit flow succeeded',{hasAccessToken:!!tokenResponse.access_token,tokenType:tokenResponse.token_type??'NONE'},'B');
      // #endregion
      onSuccess(tokenResponse.access_token);
    },
    onError: (errorResponse) => {
      // #region agent log
      _dbg('GoogleLoginButton.tsx:onError','implicit flow failed',{error:JSON.stringify(errorResponse)},'B');
      // #endregion
      console.error('Google login error:', errorResponse);
      toast.error('Google sign-in failed. Please try again.');
    },
  });

  return (
    <button
      onClick={() => googleLogin()}
      disabled={disabled}
      className="w-full flex items-center justify-center gap-3 px-4 py-3 rounded-xl border border-surface-300 dark:border-surface-700 text-surface-700 dark:text-surface-300 font-medium hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors disabled:opacity-50"
    >
      <svg className="h-5 w-5" viewBox="0 0 24 24">
        <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"/>
        <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
        <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
        <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
      </svg>
      Continue with Google
    </button>
  );
}

export default function GoogleLoginButton({ onSuccess, disabled }: { onSuccess: (token: string) => void; disabled?: boolean }) {
  if (!hasGoogleOAuth) {
    return null;
  }

  return (
    <GoogleErrorBoundary>
      <GoogleLoginButtonInner onSuccess={onSuccess} disabled={disabled} />
    </GoogleErrorBoundary>
  );
}
