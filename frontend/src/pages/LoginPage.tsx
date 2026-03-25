import { useState } from 'react';
import { useNavigate, Link, useLocation } from 'react-router';
import { motion } from 'motion/react';
import { useAuth } from '../hooks/useAuth';
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
  const navigate = useNavigate();
  const location = useLocation();

  // Redirect target after login (e.g. /claim/abc123)
  const redirectTo = (location.state as { from?: string })?.from;

  const handleSubmit = async () => {
    setError('');

    if (!username.trim()) {
      setError('Please enter a username');
      return;
    }
    if (username.trim().length < 2) {
      setError('Username must be at least 2 characters');
      return;
    }
    if (!password || password.length < 4) {
      setError('Password must be at least 4 characters');
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
      const msg = err instanceof Error ? err.message : 'Something went wrong';
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
            {isRegister ? 'Create Account' : 'Welcome back'}
          </h1>
          <p className="text-indigo-300 mt-1">
            {isRegister ? 'Join Länd of Stamp today' : 'Sign in to your Länd of Stamp account'}
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
                  Customer
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
                  Shop Owner
                </button>
              </div>
            )}

            {/* Username */}
            <div>
              <label className="block text-sm font-medium text-indigo-200 mb-1.5">Username</label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="Enter your username"
                className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white placeholder:text-indigo-400/50 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all"
              />
            </div>

            {/* Password */}
            <div>
              <label className="block text-sm font-medium text-indigo-200 mb-1.5">Password</label>
              <div className="relative">
                <input
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Enter your password"
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
                'Please wait...'
              ) : isRegister ? (
                <>
                  Create Account
                  <UserPlus className="w-5 h-5" />
                </>
              ) : (
                <>
                  Sign In
                  <ArrowRight className="w-5 h-5" />
                </>
              )}
            </button>
          </form>

          {/* Toggle login/register */}
          <div className="mt-6 pt-6 border-t border-white/10 text-center">
            <p className="text-sm text-indigo-400">
              {isRegister ? 'Already have an account?' : "Don't have an account?"}{' '}
              <button
                onClick={() => { setIsRegister(!isRegister); setError(''); }}
                className="text-accent hover:text-amber-300 font-semibold cursor-pointer"
              >
                {isRegister ? 'Sign In' : 'Register'}
              </button>
            </p>
          </div>
        </div>
      </motion.div>
    </div>
  );
}
