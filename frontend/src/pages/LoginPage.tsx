import { useState } from 'react';
import { useNavigate, Link, useLocation } from 'react-router';
import { motion } from 'motion/react';
import { useAuth } from '../hooks/useAuth';
import { useLocale } from '../hooks/useLocale';
import { toast } from 'sonner';
import { Stamp, User, ShieldCheck, ArrowRight, Eye, EyeOff, UserPlus } from 'lucide-react';

export default function LoginPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState<'user' | 'admin'>('user');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState('');
  const [isRegister, setIsRegister] = useState(false);
  const [loading, setLoading] = useState(false);
  const { login, register } = useAuth();
  const { m } = useLocale();
  const navigate = useNavigate();
  const location = useLocation();

  // Redirect target after login (e.g. /claim/abc123)
  const redirectTo = (location.state as { from?: string })?.from;

  const handleSubmit = async () => {
    setError('');

    if (!username.trim()) {
      setError(m.login.validation.usernameRequired);
      return;
    }
    if (username.trim().length < 2) {
      setError(m.login.validation.usernameMin);
      return;
    }
    if (!password || password.length < 4) {
      setError(m.login.validation.passwordMin);
      return;
    }

    setLoading(true);
    try {
      let user;
      if (isRegister) {
        user = await register(username.trim(), password, role);
      } else {
        user = await login(username.trim(), password);
      }
      // If we have a pending redirect (e.g. from QR scan), go there
      if (redirectTo) {
        navigate(redirectTo);
      } else {
        navigate(user.role === 'admin' ? '/admin' : '/dashboard');
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : m.login.validation.genericError;
      setError(msg);
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center px-4 pt-20 pb-10">

      <motion.div
        initial={{ opacity: 0, y: 30 }}
        animate={{ opacity: 1, y: 0 }}
        className="relative w-full max-w-md"
      >
        {/* Logo */}
        <div className="text-center mb-8">
          <Link to="/" className="inline-flex items-center gap-2 group">
            <div className="w-12 h-12 bg-linear-to-br from-accent to-amber-400 rounded-2xl flex items-center justify-center group-hover:scale-110 transition-transform">
              <Stamp className="w-6 h-6 text-surface" />
            </div>
          </Link>
          <h1 className="text-2xl sm:text-3xl font-black text-white mt-4">
            {isRegister ? m.login.createAccount : m.login.welcomeBack}
          </h1>
          <p className="text-indigo-300 mt-1">
            {isRegister ? m.login.joinToday : m.login.signInAccount}
          </p>
        </div>

        {/* Card */}
        <div className="bg-linear-to-br from-white/8 to-white/3 backdrop-blur-xl border border-white/10 rounded-3xl p-6 sm:p-8">
          <form action={handleSubmit} className="space-y-5">
            {/* Register: Role toggle */}
            {isRegister && (
              <div className="grid grid-cols-2 gap-2 p-1 bg-white/5 rounded-xl">
                <button
                  type="button"
                  onClick={() => setRole('user')}
                  className={`flex items-center justify-center gap-2 py-2.5 rounded-lg text-sm font-semibold transition-all cursor-pointer ${
                    role === 'user'
                      ? 'bg-primary text-white shadow-lg shadow-primary/25'
                      : 'text-indigo-300 hover:text-white'
                  }`}
                >
                  <User className="w-4 h-4" />
                  {m.common.customer}
                </button>
                <button
                  type="button"
                  onClick={() => setRole('admin')}
                  className={`flex items-center justify-center gap-2 py-2.5 rounded-lg text-sm font-semibold transition-all cursor-pointer ${
                    role === 'admin'
                      ? 'bg-primary text-white shadow-lg shadow-primary/25'
                      : 'text-indigo-300 hover:text-white'
                  }`}
                >
                  <ShieldCheck className="w-4 h-4" />
                  {m.common.shopOwner}
                </button>
              </div>
            )}

            {/* Username */}
            <div>
              <label className="block text-sm font-medium text-indigo-200 mb-1.5">{m.common.username}</label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder={m.login.enterUsername}
                className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white placeholder:text-indigo-400/50 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all"
              />
            </div>

            {/* Password */}
            <div>
              <label className="block text-sm font-medium text-indigo-200 mb-1.5">{m.common.password}</label>
              <div className="relative">
                <input
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={m.login.enterPassword}
                  className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white placeholder:text-indigo-400/50 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all pr-12"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-indigo-400 hover:text-white transition-colors cursor-pointer"
                >
                  {showPassword ? <EyeOff className="w-5 h-5" /> : <Eye className="w-5 h-5" />}
                </button>
              </div>
            </div>

            {error && (
              <motion.p
                initial={{ opacity: 0, y: -5 }}
                animate={{ opacity: 1, y: 0 }}
                className="text-red-400 text-sm bg-red-500/10 border border-red-500/20 rounded-xl px-4 py-2"
              >
                {error}
              </motion.p>
            )}

            <button
              type="submit"
              disabled={loading}
              className="w-full flex items-center justify-center gap-2 bg-linear-to-r from-accent to-amber-400 text-surface font-bold py-3.5 rounded-xl hover:shadow-lg hover:shadow-accent/25 transition-all hover:scale-[1.02] text-lg cursor-pointer disabled:opacity-50 disabled:hover:scale-100"
            >
              {loading ? (
                m.login.pleaseWait
              ) : isRegister ? (
                <>
                  {m.login.createAccountButton}
                  <UserPlus className="w-5 h-5" />
                </>
              ) : (
                <>
                  {m.login.signInButton}
                  <ArrowRight className="w-5 h-5" />
                </>
              )}
            </button>
          </form>

          {/* OAuth providers */}
          <div className="mt-5">
            <div className="flex items-center gap-3 mb-4">
              <div className="flex-1 h-px bg-white/10" />
              <span className="text-xs text-indigo-400 uppercase tracking-wider">{m.login.orContinueWith}</span>
              <div className="flex-1 h-px bg-white/10" />
            </div>
            <div className="grid grid-cols-3 gap-3">
              <a
                href="/auth/google"
                className="flex items-center justify-center gap-2 bg-white/5 hover:bg-white/10 border border-white/10 rounded-xl py-2.5 text-sm font-medium text-white transition-all hover:scale-[1.02]"
              >
                <svg className="w-5 h-5" viewBox="0 0 24 24">
                  <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"/>
                  <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                  <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                  <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                </svg>
                {m.login.continueWithGoogle}
              </a>
              <a
                href="/auth/github"
                className="flex items-center justify-center gap-2 bg-white/5 hover:bg-white/10 border border-white/10 rounded-xl py-2.5 text-sm font-medium text-white transition-all hover:scale-[1.02]"
              >
                <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/>
                </svg>
                {m.login.continueWithGitHub}
              </a>
              <a
                href="/auth/apple"
                className="flex items-center justify-center gap-2 bg-white/5 hover:bg-white/10 border border-white/10 rounded-xl py-2.5 text-sm font-medium text-white transition-all hover:scale-[1.02]"
              >
                <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M17.05 20.28c-.98.95-2.05.88-3.08.4-1.09-.5-2.08-.48-3.24 0-1.44.62-2.2.44-3.06-.4C2.79 15.25 3.51 7.59 9.05 7.31c1.35.07 2.29.74 3.08.8 1.18-.24 2.31-.93 3.57-.84 1.51.12 2.65.72 3.4 1.8-3.12 1.87-2.38 5.98.48 7.13-.57 1.5-1.31 2.99-2.54 4.09zM12.03 7.25c-.15-2.23 1.66-4.07 3.74-4.25.29 2.58-2.34 4.5-3.74 4.25z"/>
                </svg>
                {m.login.continueWithApple}
              </a>
            </div>
          </div>

          {/* Toggle login/register */}
          <div className="mt-6 pt-6 border-t border-white/10 text-center">
            <p className="text-sm text-indigo-400">
              {isRegister ? m.login.alreadyHaveAccount : m.login.dontHaveAccount}{' '}
              <button
                onClick={() => { setIsRegister(!isRegister); setError(''); }}
                className="text-accent hover:text-amber-300 font-semibold cursor-pointer"
              >
                {isRegister ? m.login.signInButton : m.login.register}
              </button>
            </p>
          </div>
        </div>
      </motion.div>
    </div>
  );
}
