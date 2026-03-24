import { createContext, useState, type ReactNode } from 'react';
import { type User, apiLogin, apiRegister, apiLogout, clearSession } from '../lib/api';
import { toast } from 'sonner';

interface AuthContextType {
  user: User | null;
  login: (username: string, password: string) => Promise<User>;
  register: (username: string, password: string, role: 'user' | 'admin') => Promise<User>;
  logout: () => void;
  isAuthenticated: boolean;
}

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

  const login = async (username: string, password: string): Promise<User> => {
    const u = await apiLogin(username, password);
    setUser(u);
    return u;
  };

  const register = async (username: string, password: string, role: 'user' | 'admin'): Promise<User> => {
    const u = await apiRegister(username, password, role);
    setUser(u);
    return u;
  };

  const logout = () => {
    apiLogout().catch((e) => {
      const msg = e instanceof Error ? e.message : 'Logout request failed';
      toast.error(msg);
    });
    setUser(null);
    clearSession();
  };

  return (
    <AuthContext.Provider value={{ user, login, register, logout, isAuthenticated: !!user }}>
      {children}
    </AuthContext.Provider>
  );
}

export { AuthContext };
export type { AuthContextType };
