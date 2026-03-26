import { useState } from 'react';
import { useNavigate } from 'react-router';
import { motion } from 'motion/react';
import { useAuth } from '../hooks/useAuth';
import { useLocale } from '../hooks/useLocale';
import { apiChooseRole, persistSession } from '../lib/api';
import { Stamp, Store } from 'lucide-react';
import { toast } from 'sonner';

export default function ChooseRolePage() {
  const navigate = useNavigate();
  const { refreshUser } = useAuth();
  const { m } = useLocale();
  const [loading, setLoading] = useState(false);

  const handleChoice = async (role: 'user' | 'admin') => {
    setLoading(true);
    try {
      const user = await apiChooseRole(role);
      persistSession(user);
      refreshUser(user);
      navigate(role === 'admin' ? '/admin' : '/dashboard', { replace: true });
    } catch {
      toast.error(m.chooseRole.error);
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center px-4 pt-20 pb-10">
      <motion.div
        initial={{ opacity: 0, y: 30 }}
        animate={{ opacity: 1, y: 0 }}
        className="relative w-full max-w-lg"
      >
        <div className="text-center mb-8">
          <h1 className="text-2xl sm:text-3xl font-black text-white">
            {m.chooseRole.title}
          </h1>
          <p className="text-indigo-300 mt-2">
            {m.chooseRole.subtitle}
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {/* Customer card */}
          <button
            onClick={() => handleChoice('user')}
            disabled={loading}
            className="group bg-linear-to-br from-white/8 to-white/3 backdrop-blur-xl border border-white/10 rounded-3xl p-6 text-left transition-all hover:scale-[1.03] hover:border-indigo-400/40 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
          >
            <div className="w-14 h-14 bg-linear-to-br from-indigo-500 to-purple-500 rounded-2xl flex items-center justify-center mb-4 group-hover:scale-110 transition-transform">
              <Stamp className="w-7 h-7 text-white" />
            </div>
            <h2 className="text-lg font-bold text-white mb-1">
              {m.chooseRole.customer}
            </h2>
            <p className="text-sm text-indigo-300">
              {m.chooseRole.customerDesc}
            </p>
          </button>

          {/* Shop Owner card */}
          <button
            onClick={() => handleChoice('admin')}
            disabled={loading}
            className="group bg-linear-to-br from-white/8 to-white/3 backdrop-blur-xl border border-white/10 rounded-3xl p-6 text-left transition-all hover:scale-[1.03] hover:border-amber-400/40 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
          >
            <div className="w-14 h-14 bg-linear-to-br from-amber-500 to-orange-500 rounded-2xl flex items-center justify-center mb-4 group-hover:scale-110 transition-transform">
              <Store className="w-7 h-7 text-white" />
            </div>
            <h2 className="text-lg font-bold text-white mb-1">
              {m.chooseRole.shopOwner}
            </h2>
            <p className="text-sm text-indigo-300">
              {m.chooseRole.shopOwnerDesc}
            </p>
          </button>
        </div>
      </motion.div>
    </div>
  );
}

