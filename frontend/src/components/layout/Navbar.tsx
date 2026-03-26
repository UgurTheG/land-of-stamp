import { Link, useNavigate, useLocation } from 'react-router';
import { useAuth } from '../../hooks/useAuth';
import { useLocale } from '../../hooks/useLocale';
import { useTheme } from '../../hooks/useTheme';
import { Stamp, LogOut, LayoutDashboard, Home, Menu, X, Sun, Moon, UserCircle2 } from 'lucide-react';
import { useState } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import LanguageSwitcher from './LanguageSwitcher';

export default function Navbar() {
  const { user, logout, isAuthenticated } = useAuth();
  const { m } = useLocale();
  const { theme, toggleTheme } = useTheme();
  const navigate = useNavigate();
  const location = useLocation();
  const [mobileOpen, setMobileOpen] = useState(false);

  const handleLogout = () => {
    logout();
    navigate('/');
    setMobileOpen(false);
  };

  const isActive = (path: string) => location.pathname === path;

  const navLinkClass = (path: string) =>
    `transition-colors duration-200 font-medium ${
      isActive(path)
        ? 'text-accent'
        : 'text-indigo-200 hover:text-white'
    }`;

  return (
    <nav className="fixed top-0 left-0 right-0 z-50 bg-surface/60 backdrop-blur-2xl border-b border-white/6 shadow-[0_4px_30px_rgba(0,0,0,0.3)]">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Logo */}
          <Link to="/" className="flex items-center gap-2 group" aria-label="Länd of Stamp">
            <div className="w-9 h-9 bg-linear-to-br from-accent to-amber-400 rounded-xl flex items-center justify-center group-hover:scale-110 transition-transform">
              <Stamp className="w-5 h-5 text-surface" />
            </div>
            <span className="text-xl font-bold text-white tracking-tight">
              Länd of <span className="text-accent">Stamp</span>
            </span>
          </Link>

          {/* Desktop nav */}
          <div className="hidden md:flex items-center gap-6">
            <Link to="/" className={navLinkClass('/')}>
              {m.common.home}
            </Link>
            {isAuthenticated && user?.role === 'user' && (
              <Link to="/dashboard" className={navLinkClass('/dashboard')}>
                {m.common.myCards}
              </Link>
            )}
            {isAuthenticated && user?.role === 'admin' && (
              <Link to="/admin" className={navLinkClass('/admin')}>
                {m.common.dashboard}
              </Link>
            )}
            {isAuthenticated && (
              <Link to="/profile" className={navLinkClass('/profile')}>
                {m.common.profile}
              </Link>
            )}

            <LanguageSwitcher />

            {/* Theme toggle */}
            <button
              onClick={toggleTheme}
              className="theme-toggle text-indigo-200"
              aria-label={theme === 'dark' ? m.navbar.switchToLight : m.navbar.switchToDark}
              title={theme === 'dark' ? m.navbar.lightMode : m.navbar.darkMode}
            >
              {theme === 'dark' ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
            </button>

            {isAuthenticated ? (
              <div className="flex items-center gap-4">
                <span className="text-sm text-indigo-300 bg-white/5 px-3 py-1 rounded-full">
                  {user?.username}
                </span>
                <button
                  onClick={handleLogout}
                  className="flex items-center gap-1.5 text-indigo-300 hover:text-white transition-colors text-sm cursor-pointer"
                >
                  <LogOut className="w-4 h-4" />
                  {m.common.logout}
                </button>
              </div>
            ) : (
              <Link
                to="/login"
                className="bg-accent hover:bg-accent-dark text-surface font-semibold px-5 py-2 rounded-xl transition-all hover:scale-105"
              >
                {m.common.signIn}
              </Link>
            )}
          </div>

          {/* Mobile hamburger */}
          <button
            className="md:hidden text-white p-2 cursor-pointer"
            onClick={() => setMobileOpen(!mobileOpen)}
          >
            {mobileOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
          </button>
        </div>
      </div>

      {/* Mobile menu */}
      <AnimatePresence>
        {mobileOpen && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="md:hidden bg-surface/90 backdrop-blur-2xl border-b border-white/6 overflow-hidden"
          >
            <div className="px-4 py-4 space-y-3">
              {/* Theme toggle (mobile) */}
              <button
                onClick={toggleTheme}
                className="flex items-center gap-2 text-indigo-200 hover:text-white py-2 w-full cursor-pointer"
              >
                {theme === 'dark' ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
                {theme === 'dark' ? m.navbar.lightMode : m.navbar.darkMode}
              </button>
              <LanguageSwitcher fullWidth />
              <Link
                to="/"
                onClick={() => setMobileOpen(false)}
                className="flex items-center gap-2 text-indigo-200 hover:text-white py-2"
              >
                <Home className="w-4 h-4" />
                {m.common.home}
              </Link>
              {isAuthenticated && user?.role === 'user' && (
                <Link
                  to="/dashboard"
                  onClick={() => setMobileOpen(false)}
                  className="flex items-center gap-2 text-indigo-200 hover:text-white py-2"
                >
                  <LayoutDashboard className="w-4 h-4" />
                  {m.common.myCards}
                </Link>
              )}
              {isAuthenticated && user?.role === 'admin' && (
                <Link
                  to="/admin"
                  onClick={() => setMobileOpen(false)}
                  className="flex items-center gap-2 text-indigo-200 hover:text-white py-2"
                >
                  <LayoutDashboard className="w-4 h-4" />
                  {m.common.dashboard}
                </Link>
              )}
              {isAuthenticated && (
                <Link
                  to="/profile"
                  onClick={() => setMobileOpen(false)}
                  className="flex items-center gap-2 text-indigo-200 hover:text-white py-2"
                >
                  <UserCircle2 className="w-4 h-4" />
                  {m.common.profile}
                </Link>
              )}
              {isAuthenticated ? (
                <button
                  onClick={handleLogout}
                  className="flex items-center gap-2 text-red-400 hover:text-red-300 py-2 w-full cursor-pointer"
                >
                  <LogOut className="w-4 h-4" />
                  {m.common.logout} ({user?.username})
                </button>
              ) : (
                <Link
                  to="/login"
                  onClick={() => setMobileOpen(false)}
                  className="block text-center bg-accent hover:bg-accent-dark text-surface font-semibold px-5 py-2.5 rounded-xl"
                >
                  {m.common.signIn}
                </Link>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </nav>
  );
}

