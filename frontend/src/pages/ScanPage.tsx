import { useState, useEffect, useRef, useCallback } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { Html5Qrcode } from 'html5-qrcode';
import { apiClaimStamp, type ClaimStampResponse } from '../lib/api';
import { toast } from 'sonner';
import {
  ScanLine,
  Camera,
  CameraOff,
  CheckCircle,
  XCircle,
  Stamp,
  Trophy,
  ArrowLeft,
  Sparkles,
  Smartphone,
} from 'lucide-react';
import { useNavigate } from 'react-router';

type ScanState = 'idle' | 'scanning' | 'processing' | 'success' | 'error';

export default function ScanPage() {
  const navigate = useNavigate();
  const [scanState, setScanState] = useState<ScanState>('idle');
  const [result, setResult] = useState<ClaimStampResponse | null>(null);
  const [errorMsg, setErrorMsg] = useState('');
  const [cameraReady, setCameraReady] = useState(false);
  const scannerRef = useRef<Html5Qrcode | null>(null);
  const processingRef = useRef(false);

  const stopScanner = useCallback(async () => {
    if (scannerRef.current) {
      try {
        const state = scannerRef.current.getState();
        if (state === 2) { // SCANNING
          await scannerRef.current.stop();
        }
      } catch (err) {
        console.warn('stopScanner: failed to stop scanner', err);
      }
      try {
        scannerRef.current.clear();
      } catch {
        // element may already be cleared
      }
      scannerRef.current = null;
    }
  }, []);

  const handleScan = useCallback(async (decodedText: string) => {
    if (processingRef.current) return;
    processingRef.current = true;
    setScanState('processing');

    await stopScanner();

    // Extract token from URL or use raw value
    let token = decodedText;
    try {
      const url = new URL(decodedText);
      const match = url.pathname.match(/\/claim\/(.+)/);
      if (match) token = match[1];
    } catch {
      // not a URL, use as raw token
    }

    try {
      const claim = await apiClaimStamp(token);
      setResult(claim);
      setScanState('success');
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to claim stamp';
      setErrorMsg(msg);
      toast.error(msg);
      setScanState('error');
    } finally {
      processingRef.current = false;
    }
  }, [stopScanner]);

  const startScanner = useCallback(async () => {
    // Guard against double-start
    if (scannerRef.current) {
      await stopScanner();
    }

    setScanState('scanning');
    setCameraReady(false);
    setResult(null);
    setErrorMsg('');
    processingRef.current = false;

    // Wait for the always-present qr-reader div to become visible
    const readerId = 'qr-reader';
    const waitForElement = (): Promise<HTMLElement> => {
      return new Promise((resolve, reject) => {
        let attempts = 0;
        const check = () => {
          const el = document.getElementById(readerId);
          if (el && el.offsetParent !== null) {
            resolve(el);
          } else if (attempts++ > 50) {
            reject(new Error('Camera container not found. Please try again.'));
          } else {
            setTimeout(check, 50);
          }
        };
        check();
      });
    };

    try {
      const el = await waitForElement();
      // Clear any leftover content from previous scanner
      el.innerHTML = '';

      const scanner = new Html5Qrcode(readerId);
      scannerRef.current = scanner;

      const config = {
        fps: 10,
        qrbox: { width: 250, height: 250 },
      };

      let started = false;
      try {
        // Try rear camera first
        await scanner.start({ facingMode: 'environment' }, config, handleScan, () => {});
        started = true;
      } catch {
        // Fall back to any available camera
        const cameras = await Html5Qrcode.getCameras();
        if (cameras.length > 0) {
          await scanner.start(cameras[0].id, config, handleScan, () => {});
          started = true;
        }
      }

      if (!started) {
        setErrorMsg('No cameras found on this device');
        toast.error('No cameras found on this device');
        setScanState('error');
        return;
      }

      setCameraReady(true);
    } catch (err) {
      const msg = err instanceof Error
        ? err.message.includes('NotAllowed')
          ? 'Camera permission denied. Please allow camera access.'
          : err.message
        : 'Failed to start camera';
      setErrorMsg(msg);
      toast.error(msg);
      setScanState('error');
    }
  }, [handleScan, stopScanner]);

  useEffect(() => {
    return () => {
      void stopScanner();
    };
  }, [stopScanner]);

  const handleReset = () => {
    setScanState('idle');
    setResult(null);
    setErrorMsg('');
  };

  // Floating stamp particles for success animation
  const particles = Array.from({ length: 12 }, (_, i) => ({
    id: i,
    x: Math.cos((i * Math.PI * 2) / 12) * 120,
    y: Math.sin((i * Math.PI * 2) / 12) * 120,
    delay: i * 0.05,
    scale: 0.5 + Math.random() * 0.5,
  }));

  return (
    <div className="min-h-screen bg-surface pt-20 pb-12">
      {/* Background */}
      <div className="fixed inset-0 pointer-events-none">
        <div className="absolute top-40 left-[10%] w-80 h-80 bg-primary/10 rounded-full blur-3xl" />
        <div className="absolute bottom-20 right-[5%] w-72 h-72 bg-accent/8 rounded-full blur-3xl" />
      </div>

      <div className="relative max-w-lg mx-auto px-4">
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="mb-8"
        >
          <button
            onClick={() => navigate('/dashboard')}
            className="flex items-center gap-1.5 text-indigo-400 hover:text-white transition-colors mb-4 cursor-pointer"
          >
            <ArrowLeft className="w-4 h-4" />
            <span className="text-sm font-medium">Back to Dashboard</span>
          </button>
          <h1 className="text-3xl sm:text-4xl font-black text-white">
            Collect <span className="text-accent">Stamps</span> 📷
          </h1>
          <p className="text-indigo-300 mt-2">
            Scan a shop's QR code to earn a stamp
          </p>
        </motion.div>

        {/* 
          Always-present scanner container. Shown/hidden via CSS so the DOM element
          is always available when Html5Qrcode tries to mount into it.
        */}
        <div className={scanState === 'scanning' ? 'block' : 'hidden'}>
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            className="flex flex-col items-center"
          >
            <div className="bg-linear-to-br from-white/8 to-white/3 backdrop-blur-xl border border-white/10 rounded-3xl p-5 w-full overflow-hidden">
              {/* Camera viewport */}
              <div className="relative rounded-2xl overflow-hidden bg-black aspect-square">
                <div id="qr-reader" className="w-full min-h-75" />

                {/* Scanning overlay */}
                {cameraReady && (
                  <div className="absolute inset-0 pointer-events-none">
                    <motion.div
                      className="absolute left-[15%] right-[15%] h-0.5 bg-linear-to-r from-transparent via-accent to-transparent shadow-[0_0_15px_rgba(245,158,11,0.8)]"
                      animate={{ top: ['25%', '75%', '25%'] }}
                      transition={{ duration: 2, repeat: Infinity, ease: 'easeInOut' }}
                    />
                  </div>
                )}

                {!cameraReady && (
                  <div className="absolute inset-0 flex items-center justify-center bg-surface/80">
                    <motion.div
                      animate={{ rotate: 360 }}
                      transition={{ duration: 1.5, repeat: Infinity, ease: 'linear' }}
                    >
                      <Camera className="w-8 h-8 text-indigo-400" />
                    </motion.div>
                  </div>
                )}
              </div>

              <div className="flex items-center justify-between mt-4">
                <div className="flex items-center gap-2">
                  <motion.div
                    className="w-2 h-2 rounded-full bg-emerald-400"
                    animate={{ opacity: [1, 0.3, 1] }}
                    transition={{ duration: 1.5, repeat: Infinity }}
                  />
                  <span className="text-sm text-indigo-300">
                    {cameraReady ? 'Scanning...' : 'Starting camera...'}
                  </span>
                </div>
                <button
                  onClick={async () => {
                    await stopScanner();
                    handleReset();
                  }}
                  className="flex items-center gap-1.5 text-sm text-indigo-400 hover:text-white transition-colors cursor-pointer"
                >
                  <CameraOff className="w-4 h-4" />
                  Stop
                </button>
              </div>
            </div>
          </motion.div>
        </div>

        <AnimatePresence mode="wait">
          {/* ── Idle State ── */}
          {scanState === 'idle' && (
            <motion.div
              key="idle"
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -20 }}
              className="flex flex-col items-center gap-5"
            >
              {/* Recommended: Native camera */}
              <div className="bg-linear-to-br from-white/8 to-white/3 backdrop-blur-xl border border-accent/20 rounded-3xl p-6 w-full">
                <div className="flex flex-col items-center gap-4">
                  <motion.div
                    className="w-20 h-20 rounded-2xl bg-accent/20 flex items-center justify-center"
                    animate={{ scale: [1, 1.05, 1] }}
                    transition={{ duration: 2, repeat: Infinity, ease: 'easeInOut' }}
                  >
                    <Smartphone className="w-10 h-10 text-accent" />
                  </motion.div>

                  <div className="text-center">
                    <div className="inline-block bg-accent/20 text-accent text-xs font-bold px-2.5 py-1 rounded-full mb-2">
                      ✨ Recommended
                    </div>
                    <h2 className="text-lg font-bold text-white">Use Your Phone Camera</h2>
                    <p className="text-indigo-400 text-sm mt-1 max-w-xs">
                      Simply open your phone's camera app and point it at the shop's QR code. It will open automatically!
                    </p>
                  </div>
                </div>
              </div>

              {/* Divider */}
              <div className="flex items-center gap-3 w-full">
                <div className="flex-1 h-px bg-white/10" />
                <span className="text-xs text-indigo-500 font-medium">OR</span>
                <div className="flex-1 h-px bg-white/10" />
              </div>

              {/* Fallback: In-app scanner */}
              <div className="bg-linear-to-br from-white/8 to-white/3 backdrop-blur-xl border border-white/10 rounded-3xl p-6 w-full">
                <div className="flex flex-col items-center gap-4">
                  <motion.div
                    className="relative w-16 h-16 rounded-2xl bg-primary/15 flex items-center justify-center"
                  >
                    <Camera className="w-8 h-8 text-primary-light" />
                    <motion.div
                      className="absolute left-2 right-2 h-0.5 bg-linear-to-r from-transparent via-accent to-transparent"
                      animate={{ top: ['25%', '75%', '25%'] }}
                      transition={{ duration: 2.5, repeat: Infinity, ease: 'easeInOut' }}
                    />
                  </motion.div>

                  <div className="text-center">
                    <h2 className="text-base font-bold text-white">In-App Scanner</h2>
                    <p className="text-indigo-400 text-xs mt-1">
                      Use the built-in camera scanner if your phone camera doesn't detect QR codes
                    </p>
                  </div>

                  <button
                    onClick={startScanner}
                    className="flex items-center gap-2 bg-white/10 border border-white/10 text-white font-semibold px-6 py-2.5 rounded-xl hover:bg-white/15 transition-all cursor-pointer text-sm"
                  >
                    <ScanLine className="w-4 h-4" />
                    Open Scanner
                  </button>
                </div>
              </div>

              {/* Tips */}
              <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ delay: 0.3 }}
                className="w-full"
              >
                <div className="bg-white/5 border border-white/10 rounded-2xl p-4">
                  <h3 className="text-sm font-semibold text-white mb-2">💡 Tips</h3>
                  <ul className="text-xs text-indigo-400 space-y-1.5">
                    <li>• Hold your phone steady over the QR code</li>
                    <li>• Make sure the QR code is well-lit</li>
                    <li>• Each QR code can be scanned once per customer</li>
                    <li>• QR codes expire after 60 seconds</li>
                  </ul>
                </div>
              </motion.div>
            </motion.div>
          )}

          {/* ── Processing State ── */}
          {scanState === 'processing' && (
            <motion.div
              key="processing"
              initial={{ opacity: 0, scale: 0.9 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.9 }}
              className="flex flex-col items-center py-16"
            >
              <motion.div
                className="w-24 h-24 rounded-full bg-linear-to-br from-primary/30 to-accent/20 border border-white/20 flex items-center justify-center"
                animate={{ scale: [1, 1.1, 1], rotate: [0, 180, 360] }}
                transition={{ duration: 1.5, repeat: Infinity, ease: 'easeInOut' }}
              >
                <Stamp className="w-10 h-10 text-accent" />
              </motion.div>
              <motion.p
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ delay: 0.3 }}
                className="text-indigo-300 mt-6 text-lg font-medium"
              >
                Claiming your stamp...
              </motion.p>
            </motion.div>
          )}

          {/* ── Success State ── */}
          {scanState === 'success' && result && (
            <motion.div
              key="success"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="flex flex-col items-center"
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
                      transition={{
                        duration: 1.2,
                        delay: p.delay,
                        ease: 'easeOut',
                      }}
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
                      animate={{ boxShadow: [
                        '0 0 0 0 rgba(99,102,241,0)',
                        '0 0 0 20px rgba(99,102,241,0.15)',
                        '0 0 0 40px rgba(99,102,241,0)',
                      ]}}
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
                      {result.stamps >= result.stampsRequired ? 'Card Complete! 🏆' : 'Stamp Collected! 🎉'}
                    </h2>
                    <p className="text-indigo-300 text-lg font-medium">{result.shopName}</p>
                  </motion.div>

                  {/* Stamp counter */}
                  <motion.div
                    initial={{ opacity: 0, scale: 0.8 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ delay: 0.7 }}
                    className="mt-6 flex flex-wrap items-center justify-center gap-1"
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
                        {i < result.stamps ? (
                          <Stamp className="w-4 h-4" />
                        ) : (
                          i + 1
                        )}
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
                    {result.stamps} / {result.stampsRequired} stamps
                  </motion.p>

                  {/* Message */}
                  <motion.p
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ delay: 1.2 }}
                    className="text-accent font-medium mt-3"
                  >
                    {result.message}
                  </motion.p>
                </div>

                {/* Actions */}
                <motion.div
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 1.4 }}
                  className="flex flex-col sm:flex-row gap-3 mt-8"
                >
                  <button
                    onClick={() => {
                      handleReset();
                      setTimeout(startScanner, 100);
                    }}
                    className="flex-1 flex items-center justify-center gap-2 bg-linear-to-r from-accent to-amber-400 text-surface font-bold px-6 py-3 rounded-xl hover:scale-[1.02] transition-transform cursor-pointer"
                  >
                    <ScanLine className="w-4 h-4" />
                    Scan Another
                  </button>
                  <button
                    onClick={() => navigate('/dashboard')}
                    className="flex-1 flex items-center justify-center gap-2 bg-white/10 border border-white/10 text-white font-semibold px-6 py-3 rounded-xl hover:bg-white/15 transition-all cursor-pointer"
                  >
                    <ArrowLeft className="w-4 h-4" />
                    Dashboard
                  </button>
                </motion.div>
              </div>
            </motion.div>
          )}

          {/* ── Error State ── */}
          {scanState === 'error' && (
            <motion.div
              key="error"
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -20 }}
              className="flex flex-col items-center"
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
                    <h2 className="text-xl font-bold text-white mb-2">Oops!</h2>
                    <p className="text-red-300 text-sm max-w-xs">{errorMsg}</p>
                  </div>

                  <div className="flex flex-col sm:flex-row gap-3 w-full">
                    <button
                      onClick={() => {
                        handleReset();
                        setTimeout(startScanner, 100);
                      }}
                      className="flex-1 flex items-center justify-center gap-2 bg-linear-to-r from-accent to-amber-400 text-surface font-bold px-6 py-3 rounded-xl hover:scale-[1.02] transition-transform cursor-pointer"
                    >
                      <ScanLine className="w-4 h-4" />
                      Try Again
                    </button>
                    <button
                      onClick={() => navigate('/dashboard')}
                      className="flex-1 flex items-center justify-center gap-2 bg-white/10 border border-white/10 text-white font-semibold px-6 py-3 rounded-xl hover:bg-white/15 transition-all cursor-pointer"
                    >
                      <ArrowLeft className="w-4 h-4" />
                      Dashboard
                    </button>
                  </div>
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}

