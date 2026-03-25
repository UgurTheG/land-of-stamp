import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router';
import { apiGetMe, persistSession } from '../lib/api';
import { useAuth } from '../hooks/useAuth';
import { useLocale } from '../hooks/useLocale';

/**
 * After the backend OAuth callback sets the JWT cookie and redirects here,
 * this page calls GetMe to hydrate the auth context, then navigates to
 * the appropriate dashboard.
 */
export default function OAuthCallbackPage() {
  const navigate = useNavigate();
  const { refreshUser } = useAuth();
  const { m } = useLocale();
  const [error, setError] = useState('');

  useEffect(() => {
    apiGetMe()
      .then((user) => {
        persistSession(user);
        refreshUser(user);
        navigate(user.role === 'admin' ? '/admin' : '/dashboard', { replace: true });
      })
      .catch(() => {
        setError(m.login.validation.genericError);
        setTimeout(() => navigate('/login', { replace: true }), 2000);
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-red-400">{error}</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center">
      <p className="text-indigo-300 animate-pulse">{m.common.loading}</p>
    </div>
  );
}

