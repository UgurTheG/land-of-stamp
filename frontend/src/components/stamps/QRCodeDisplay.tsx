import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { QRCodeSVG } from 'qrcode.react';
import { RefreshCw, QrCode, Clock, Sparkles } from 'lucide-react';
import { useLocale } from '../../hooks/useLocale';
import { apiCreateStampToken } from '../../lib/api';
import { toast } from 'sonner';

interface Props {
  shopId: string;
  shopColor: string;
  shopName: string;
}

export default function QRCodeDisplay({ shopId, shopColor, shopName }: Props) {
  const { m } = useLocale();
  const [token, setToken] = useState<string | null>(null);
  const [expiresAt, setExpiresAt] = useState<Date | null>(null);
  const [secondsLeft, setSecondsLeft] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval>>(undefined);

  const generateToken = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await apiCreateStampToken(shopId);
      setToken(result.token);
      setExpiresAt(new Date(result.expiresAt));
    } catch (e) {
      const msg = e instanceof Error ? e.message : m.qrCodeDisplay.generateFailed;
      setError(msg);
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }, [m.qrCodeDisplay.generateFailed, shopId]);

  // Reset state when the selected shop changes
  useEffect(() => {
    setToken(null);
    setExpiresAt(null);
    setError(null);
    setSecondsLeft(0);
    clearInterval(timerRef.current);
  }, [shopId]);

  // Countdown timer
  useEffect(() => {
    if (!expiresAt) return;
    const update = () => {
      const diff = Math.max(0, Math.floor((expiresAt.getTime() - Date.now()) / 1000));
      setSecondsLeft(diff);
      if (diff <= 0) {
        setToken(null);
        setExpiresAt(null);
      }
    };
    update();
    timerRef.current = setInterval(update, 1000);
    return () => clearInterval(timerRef.current);
  }, [expiresAt]);

  const progressPercent = expiresAt ? Math.max(0, (secondsLeft / 60) * 100) : 0;

  // Build a full URL so native phone cameras open it directly
  const claimUrl = token ? `${window.location.origin}/claim/${token}` : '';

  // Corner sparkle positions (top-left, top-right, bottom-right, bottom-left)
  const corners = useMemo(
    () => [
      { x: -4, y: -4 },
      { x: 250, y: -4 },
      { x: 250, y: 250 },
      { x: -4, y: 250 },
    ],
    [],
  );

  // The conic-gradient colors for the animated border
  const borderGradient = `conic-gradient(from var(--border-angle), ${shopColor}, #f59e0b, #ec4899, #06b6d4, #8b5cf6, #10b981, ${shopColor})`;

  return (
    <div className="flex flex-col items-center">
      {/* Inject the @property + keyframes for the border rotation */}
      <style>{`
        @property --border-angle {
          syntax: "<angle>";
          initial-value: 0deg;
          inherits: false;
        }
        @keyframes spin-border {
          to { --border-angle: 360deg; }
        }
        .animated-border-box {
          --border-angle: 0deg;
          animation: spin-border 4s linear infinite;
        }
      `}</style>

      <AnimatePresence mode="wait">
        {!token && !loading ? (
          <motion.div
            key="generate"
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9 }}
            className="flex flex-col items-center gap-6 py-8"
          >
            <motion.div
              className="w-32 h-32 rounded-3xl bg-linear-to-br from-white/10 to-white/5 border border-white/10 flex items-center justify-center"
              animate={{ rotate: [0, 5, -5, 0] }}
              transition={{ duration: 4, repeat: Infinity, ease: 'easeInOut' }}
            >
              <QrCode className="w-16 h-16 text-indigo-400" />
            </motion.div>

            {error && (
              <motion.p initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="text-red-400 text-sm text-center">
                {error}
              </motion.p>
            )}

            <p className="text-white font-semibold text-lg">{shopName}</p>

            <button
              onClick={generateToken}
              className="flex items-center gap-2 bg-linear-to-r from-primary to-primary-dark text-white font-bold px-8 py-3.5 rounded-2xl hover:scale-[1.03] active:scale-[0.98] transition-transform cursor-pointer shadow-lg shadow-primary/30"
            >
              <Sparkles className="w-5 h-5" />
              {m.qrCodeDisplay.generateButton}
            </button>
            <p className="text-indigo-400 text-sm text-center max-w-xs">
              {m.qrCodeDisplay.generateHint}
            </p>
          </motion.div>
        ) : loading ? (
          <motion.div
            key="loading"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="flex flex-col items-center gap-4 py-16"
          >
            <motion.div animate={{ rotate: 360 }} transition={{ duration: 1, repeat: Infinity, ease: 'linear' }}>
              <RefreshCw className="w-8 h-8 text-primary-light" />
            </motion.div>
            <p className="text-indigo-300 text-sm">{m.qrCodeDisplay.generating}</p>
          </motion.div>
        ) : (
          <motion.div
            key="qr"
            initial={{ opacity: 0, scale: 0.8, rotateY: 90 }}
            animate={{ opacity: 1, scale: 1, rotateY: 0 }}
            exit={{ opacity: 0, scale: 0.8, rotateY: -90 }}
            transition={{ type: 'spring', stiffness: 200, damping: 20 }}
            className="flex flex-col items-center gap-5"
          >
            {/* ─── QR Card with Animated Gradient Border ─── */}
            <div className="relative">
              {/*
                Outer wrapper: the conic-gradient IS the border.
                We use padding to create the border width, and the
                inner child masks the centre so only the edges show.
              */}
                <div
                  className="animated-border-box rounded-2xl p-0.75 relative"
                style={{ background: borderGradient }}
              >
                {/* Glow layer behind the border (blurred copy) */}
                <div
                  className="animated-border-box absolute -inset-1 rounded-2xl blur-md opacity-50 -z-10"
                  style={{ background: borderGradient }}
                />

                {/* Second glow — wider, softer */}
                <motion.div
                  className="absolute -inset-3 rounded-3xl blur-2xl -z-20"
                  style={{ backgroundColor: shopColor }}
                  animate={{ opacity: [0.12, 0.28, 0.12] }}
                  transition={{ duration: 3, repeat: Infinity, ease: 'easeInOut' }}
                />

                {/* QR Code card (white interior) */}
                <div className="relative bg-white rounded-[13px] p-3">
                  {/* Corner sparkles that pulse on the card corners */}
                  {corners.map((c, i) => (
                    <motion.div
                      key={i}
                      className="absolute w-2 h-2 rounded-full z-20"
                      style={{
                        left: c.x,
                        top: c.y,
                        backgroundColor: i % 2 === 0 ? '#f59e0b' : shopColor,
                        boxShadow: `0 0 8px ${i % 2 === 0 ? '#f59e0b' : shopColor}`,
                      }}
                      animate={{
                        scale: [0.6, 1.4, 0.6],
                        opacity: [0.4, 1, 0.4],
                      }}
                      transition={{
                        duration: 2,
                        repeat: Infinity,
                        ease: 'easeInOut',
                        delay: i * 0.5,
                      }}
                    />
                  ))}

                  <QRCodeSVG
                    value={claimUrl}
                    size={240}
                    level="M"
                    bgColor="#ffffff"
                    fgColor="#1e1b4b"
                    style={{ display: 'block' }}
                  />
                </div>
              </div>
            </div>

            {/* Instruction text */}
            <motion.p
              className="text-indigo-300 text-sm text-center max-w-xs"
              animate={{ opacity: [0.6, 1, 0.6] }}
              transition={{ duration: 2, repeat: Infinity }}
            >
              {m.qrCodeDisplay.phoneCameraHint}
            </motion.p>

            {/* Timer */}
            <div className="flex flex-col items-center gap-2 w-full max-w-64">
              <div className="flex items-center gap-2 text-sm">
                <Clock className="w-4 h-4 text-indigo-400" />
                <span
                  className={`font-mono font-bold text-lg ${secondsLeft <= 10 ? 'text-red-400' : 'text-white'}`}
                >
                  {secondsLeft}s
                </span>
                <span className="text-indigo-400">{m.qrCodeDisplay.remaining}</span>
              </div>

              {/* Progress bar */}
              <div className="w-full h-1.5 bg-white/10 rounded-full overflow-hidden">
                <motion.div
                  className="h-full rounded-full"
                  style={{
                    backgroundColor: secondsLeft <= 10 ? '#ef4444' : shopColor,
                    width: `${progressPercent}%`,
                  }}
                  transition={{ duration: 0.3 }}
                />
              </div>
            </div>

            {/* Regenerate button */}
            <button
              onClick={generateToken}
              disabled={loading}
              className="flex items-center gap-2 bg-white/10 border border-white/10 text-white font-semibold px-5 py-2.5 rounded-xl hover:bg-white/15 transition-all cursor-pointer text-sm"
            >
              <RefreshCw className="w-4 h-4" />
              {m.qrCodeDisplay.newQrCode}
            </button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
