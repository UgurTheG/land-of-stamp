import { createContext, useState, useEffect, type ReactNode } from 'react';
import { apiLogout, apiGetMe, clearSession } from '../lib/api';
import { toast } from 'sonner';

interface AuthContextType {
  user: User | null;
  logout: () => void;
  refreshUser: (user: User) => void;
  isAuthenticated: boolean;
}

// Re-export User from api so consumers don't need a separate import.
import type { User } from '../lib/api';

const AuthContext = createContext<AuthContextType | null>(null);

function getInitialUser(): User | null {
  try {
    const stored = localStorage.getItem('land_of_stamp_current_user');
    if (stored) return JSON.parse(stored);
  } catch {
    localStorage.removeItem('land_of_stamp_current_user');
  }
  return null;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(getInitialUser);

  // Verify the session is still valid on mount. If the backend restarted
  // with a new JWT secret the cookie token is stale and GetMe will
  // return 401 — in that case clear the local state so the UI reflects it.
  useEffect(() => {
    if (!user) return;
    apiGetMe()
      .then((freshUser) => setUser(freshUser))
      .catch(() => {
        setUser(null);
        clearSession();
      });
    // Only run once on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const logout = () => {
    apiLogout().catch((e) => {
      const msg = e instanceof Error ? e.message : 'Logout request failed';
      toast.error(msg);
    });
    setUser(null);
    clearSession();
  };

  const refreshUser = (u: User) => setUser(u);

  return (
    <AuthContext.Provider value={{ user, logout, refreshUser, isAuthenticated: !!user }}>
      {children}
    </AuthContext.Provider>
  );
}

export { AuthContext };
export type { AuthContextType };
