import { useState, useEffect, useRef, useMemo } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { useParams, useNavigate } from 'react-router';
import { useLocale } from '../hooks/useLocale';
import { apiClaimStamp, type ClaimStampResponse } from '../lib/api';
import { toast } from 'sonner';
import {
  CheckCircle,
  Stamp,
  Trophy,
  ArrowLeft,
  Sparkles,
  XCircle,
  Loader2,
} from 'lucide-react';

type ClaimState = 'loading' | 'success' | 'error';

export default function ClaimPage() {
  const { token } = useParams<{ token: string }>();
  const { m } = useLocale();
  const navigate = useNavigate();
  const [state, setState] = useState<ClaimState>('loading');
  const [result, setResult] = useState<ClaimStampResponse | null>(null);
  const [errorMsg, setErrorMsg] = useState('');
  const claimedRef = useRef(false);

  useEffect(() => {
    if (!token || claimedRef.current) return;
    claimedRef.current = true;

    (async () => {
      try {
        const claim = await apiClaimStamp(token);
        setResult(claim);
        setState('success');
      } catch (e) {
        const msg = e instanceof Error ? e.message : m.claim.claimFailed;
        setErrorMsg(msg);
        toast.error(msg);
        setState('error');
      }
    })();
  }, [m.claim.claimFailed, token]);

  // Floating stamp particles for success animation (stable across re-renders)
  const particles = useMemo(
    () =>
      Array.from({ length: 16 }, (_, i) => ({
        id: i,
        x: Math.cos((i * Math.PI * 2) / 16) * (100 + (((i * 7 + 3) % 11) / 11) * 60),
        y: Math.sin((i * Math.PI * 2) / 16) * (100 + (((i * 13 + 5) % 11) / 11) * 60),
        delay: i * 0.04,
        scale: 0.4 + (((i * 17 + 1) % 11) / 11) * 0.6,
      })),
    [],
  );

  return (
    <div className="min-h-screen pt-20 pb-12">

      <div className="relative max-w-lg mx-auto px-4">
        <AnimatePresence mode="wait">
          {/* ── Loading ── */}
          {state === 'loading' && (
            <motion.div
              key="loading"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="flex flex-col items-center py-24"
            >
              <motion.div
                className="w-28 h-28 rounded-full bg-linear-to-br from-primary/30 to-accent/20 border border-white/20 flex items-center justify-center"
                animate={{ scale: [1, 1.1, 1], rotate: [0, 180, 360] }}
                transition={{ duration: 1.5, repeat: Infinity, ease: 'easeInOut' }}
              >
                <Loader2 className="w-10 h-10 text-accent animate-spin" />
              </motion.div>
              <motion.p
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ delay: 0.3 }}
                className="text-indigo-300 mt-6 text-lg font-medium"
              >
                {m.claim.claiming}
              </motion.p>
            </motion.div>
          )}

          {/* ── Success ── */}
          {state === 'success' && result && (
            <motion.div
              key="success"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="flex flex-col items-center pt-8"
            >
              <div className="bg-linear-to-br from-white/8 to-white/3 backdrop-blur-xl border border-white/10 rounded-3xl p-8 w-full">
                <div className="flex flex-col items-center relative">
                  {/* Particle burst */}
                  {particles.map((p) => (
                    <motion.div
                      key={p.id}
                      className="absolute text-accent"
                      initial={{ x: 0, y: 0, opacity: 0, scale: 0 }}
                      animate={{
                        x: p.x,
                        y: p.y,
                        opacity: [0, 1, 0],
                        scale: [0, p.scale, 0],
                      }}
                      transition={{ duration: 1.4, delay: p.delay, ease: 'easeOut' }}
                    >
                      <Sparkles className="w-4 h-4" />
                    </motion.div>
                  ))}

                  {/* Success icon */}
                  <motion.div
                    initial={{ scale: 0, rotate: -180 }}
                    animate={{ scale: 1, rotate: 0 }}
                    transition={{ type: 'spring', stiffness: 200, damping: 15, delay: 0.2 }}
                    className="relative"
                  >
                    <motion.div
                      className="w-28 h-28 rounded-full flex items-center justify-center"
                      style={{
                        background: result.stamps >= result.stampsRequired
                          ? 'linear-gradient(135deg, #f59e0b, #ef4444)'
                          : 'linear-gradient(135deg, #6366f1, #818cf8)',
                      }}
                      animate={{
                        boxShadow: [
                          '0 0 0 0 rgba(99,102,241,0)',
                          '0 0 0 20px rgba(99,102,241,0.15)',
                          '0 0 0 40px rgba(99,102,241,0)',
                        ],
                      }}
                      transition={{ duration: 2, repeat: Infinity }}
                    >
                      {result.stamps >= result.stampsRequired ? (
                        <Trophy className="w-12 h-12 text-white" />
                      ) : (
                        <CheckCircle className="w-12 h-12 text-white" />
                      )}
                    </motion.div>
                  </motion.div>

                  {/* Text */}
                  <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: 0.5 }}
                    className="text-center mt-6"
                  >
                    <h2 className="text-2xl font-black text-white mb-1">
                      {result.stamps >= result.stampsRequired
                        ? m.claim.cardComplete
                        : m.claim.stampCollected}
                    </h2>
                    <p className="text-indigo-300 text-lg font-medium">{result.shopName}</p>
                  </motion.div>

                  {/* Stamp counter */}
                  <motion.div
                    initial={{ opacity: 0, scale: 0.8 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ delay: 0.7 }}
                    className="mt-6 flex flex-wrap items-center justify-center gap-1.5"
                  >
                    {Array.from({ length: result.stampsRequired }).map((_, i) => (
                      <motion.div
                        key={i}
                        initial={{ scale: 0 }}
                        animate={{ scale: 1 }}
                        transition={{ delay: 0.8 + i * 0.06, type: 'spring', stiffness: 300 }}
                        className={`w-8 h-8 rounded-lg flex items-center justify-center text-xs font-bold ${
                          i < result.stamps
                            ? 'bg-linear-to-br from-primary to-primary-dark text-white'
                            : 'bg-white/10 border border-white/10 text-indigo-500'
                        } ${i === result.stamps - 1 ? 'ring-2 ring-accent ring-offset-2 ring-offset-surface' : ''}`}
                      >
                        {i < result.stamps ? <Stamp className="w-4 h-4" /> : i + 1}
                      </motion.div>
                    ))}
                  </motion.div>

                  {/* Progress text */}
                  <motion.p
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ delay: 1 }}
                    className="text-sm text-indigo-400 mt-4"
                  >
                    {m.claim.stamps(result.stamps, result.stampsRequired)}
                  </motion.p>

                  {/* Message */}
                  <motion.p
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ delay: 1.2 }}
                    className="text-accent font-medium mt-3 text-center"
                  >
                    {result.message}
                  </motion.p>
                </div>

                {/* Actions */}
                <motion.div
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 1.4 }}
                  className="mt-8"
                >
                  <button
                    onClick={() => navigate('/dashboard')}
                    className="w-full flex items-center justify-center gap-2 bg-linear-to-r from-accent to-amber-400 text-surface font-bold px-6 py-3.5 rounded-xl hover:scale-[1.02] transition-transform cursor-pointer"
                  >
                    <ArrowLeft className="w-4 h-4" />
                    {m.claim.goToMyCards}
                  </button>
                </motion.div>
              </div>
            </motion.div>
          )}

          {/* ── Error ── */}
          {state === 'error' && (
            <motion.div
              key="error"
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -20 }}
              className="flex flex-col items-center pt-8"
            >
              <div className="bg-linear-to-br from-white/8 to-white/3 backdrop-blur-xl border border-red-500/20 rounded-3xl p-8 w-full">
                <div className="flex flex-col items-center gap-5">
                  <motion.div
                    initial={{ scale: 0 }}
                    animate={{ scale: 1 }}
                    transition={{ type: 'spring', stiffness: 200, damping: 15 }}
                    className="w-20 h-20 rounded-full bg-red-500/20 flex items-center justify-center"
                  >
                    <XCircle className="w-10 h-10 text-red-400" />
                  </motion.div>

                  <div className="text-center">
                    <h2 className="text-xl font-bold text-white mb-2">{m.claim.couldntClaim}</h2>
                    <p className="text-red-300 text-sm max-w-xs">{errorMsg}</p>
                  </div>

                  <button
                    onClick={() => navigate('/dashboard')}
                    className="w-full flex items-center justify-center gap-2 bg-white/10 border border-white/10 text-white font-semibold px-6 py-3 rounded-xl hover:bg-white/15 transition-all cursor-pointer"
                  >
                    <ArrowLeft className="w-4 h-4" />
                    {m.claim.goToDashboard}
                  </button>
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}

