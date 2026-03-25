import { motion, AnimatePresence } from 'motion/react';
import { Trophy, PartyPopper } from 'lucide-react';
import StampSlot from './StampSlot';
import { useLocale } from '../../hooks/useLocale';
import type { Shop, StampCard as StampCardType } from '../../lib/api';

// Helper function to get progress bar color based on progress percentage
const getProgressColor = (progress: number): string => {
  if (progress < 33) return '#ef4444'; // Red for 0-33%
  if (progress < 66) return '#f59e0b'; // Orange for 33-66%
  return '#10b981'; // Green for 66-100%
};

interface Props {
  shop: Shop;
  card: StampCardType;
  onRedeem?: () => void;
}

export default function StampCard({ shop, card, onRedeem }: Props) {
  const { m } = useLocale();
  const isComplete = card.stamps >= shop.stampsRequired;
  const progress = Math.min((card.stamps / shop.stampsRequired) * 100, 100);

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      className={`relative overflow-hidden rounded-3xl bg-linear-to-br from-white/10 to-white/5 backdrop-blur-sm border p-5 sm:p-6 transition-all duration-500 ${
        isComplete && !card.redeemed
          ? 'border-emerald-400/50 shadow-lg shadow-emerald-400/20 from-white/15 to-white/8'
          : 'border-white/10'
      }`}
    >
      {/* Header */}
      <div className="flex items-start justify-between mb-4">
        <div>
          <h3 className="text-lg sm:text-xl font-bold text-white">{shop.name}</h3>
          <p className="text-sm text-indigo-300 mt-0.5">{shop.description}</p>
        </div>
        <div
          className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
          style={{ backgroundColor: shop.color + '33' }}
        >
          <Trophy className="w-5 h-5" style={{ color: shop.color }} />
        </div>
      </div>

      {/* Progress bar */}
      <div className="mb-4">
        <div className="flex justify-between text-xs text-indigo-300 mb-1.5">
          <span>{m.stampCard.stamps(card.stamps, shop.stampsRequired)}</span>
          <span>{Math.round(progress)}%</span>
        </div>
        <div className="h-2 bg-white/10 rounded-full overflow-hidden">
          <motion.div
            className="h-full rounded-full"
            initial={{ width: 0 }}
            animate={{ width: `${progress}%` }}
            transition={{ duration: 0.8, ease: 'easeOut' }}
            style={{ backgroundColor: getProgressColor(progress) }}
          />
        </div>
      </div>

      {/* Stamp grid */}
      <div className="grid grid-cols-5 gap-2 sm:gap-3 mb-4">
        {Array.from({ length: shop.stampsRequired }).map((_, i) => (
          <StampSlot
            key={i}
            index={i}
            filled={i < card.stamps}
            isRewardSlot={i === shop.stampsRequired - 1}
            color={shop.color}
          />
        ))}
      </div>

      {/* Reward info */}
      <div className="flex items-center gap-2 text-sm bg-white/5 rounded-xl px-3 py-2">
        <PartyPopper className="w-4 h-4 text-accent shrink-0" />
        <span className="text-indigo-200">
          <span className="text-white font-medium">{m.stampCard.reward}</span> {shop.rewardDescription}
        </span>
      </div>

      {/* Complete overlay */}
      <AnimatePresence>
        {isComplete && !card.redeemed && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="absolute inset-0 bg-surface/95 backdrop-blur-md flex flex-col items-center justify-center p-6 rounded-3xl border-2 border-emerald-400/30"
          >
            {/* Celebration particles */}
            <div className="absolute inset-0 pointer-events-none overflow-hidden rounded-3xl">
              {[...Array(12)].map((_, i) => {
                const startX = `${10 + (i % 4) * 25 + Math.random() * 15}%`;
                const driftX = (Math.random() - 0.5) * 80;
                return (
                  <motion.div
                    key={i}
                    initial={{
                      top: '-8%',
                      left: startX,
                      opacity: 1,
                      scale: 1,
                      x: 0,
                    }}
                    animate={{
                      top: '110%',
                      x: driftX,
                      opacity: 0,
                      scale: 0.4,
                      rotate: Math.random() * 360,
                    }}
                    transition={{
                      duration: 2.5 + Math.random(),
                      delay: i * 0.12,
                      ease: 'easeOut',
                    }}
                    className="absolute text-2xl"
                  >
                    {['🎉', '⭐', '🏆', '✨', '🎊', '🌟'][i % 6]}
                  </motion.div>
                );
              })}
            </div>

            <motion.div
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
              transition={{ type: 'spring', stiffness: 200, damping: 15, delay: 0.2 }}
              className="relative z-10"
            >
              <motion.div
                animate={{ 
                  boxShadow: ['0 0 0 0 rgba(16, 185, 129, 0.4)', '0 0 0 20px rgba(16, 185, 129, 0)']
                }}
                transition={{ duration: 1.5, repeat: Infinity }}
                className="w-20 h-20 bg-linear-to-br from-emerald-400 to-green-500 rounded-full flex items-center justify-center mb-4 mx-auto"
              >
                <Trophy className="w-10 h-10 text-white" />
              </motion.div>
            </motion.div>

            <motion.h3
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.4 }}
              className="text-2xl font-bold text-white text-center mb-2 relative z-10"
            >
              {m.stampCard.congratulations}
            </motion.h3>

            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.45 }}
              className="text-center mb-1 relative z-10"
            >
              <p className="text-lg text-emerald-300 font-semibold">
                ✨ {m.stampCard.cardComplete} ✨
              </p>
            </motion.div>

            <motion.p
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.5 }}
              className="text-indigo-300 text-center mb-4 relative z-10"
            >
              {m.stampCard.earned} <span className="text-emerald-400 font-semibold">{shop.rewardDescription}</span>
            </motion.p>

            {onRedeem && (
              <motion.button
                initial={{ opacity: 0, y: 10, scale: 0.95 }}
                animate={{ opacity: 1, y: 0, scale: 1 }}
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
                transition={{ delay: 0.6 }}
                onClick={onRedeem}
                className="bg-linear-to-r from-emerald-400 to-green-500 text-white font-bold px-8 py-3 rounded-xl hover:shadow-lg hover:shadow-emerald-400/50 transition-all cursor-pointer relative z-10"
              >
                {m.stampCard.redeemReward} 🎁
              </motion.button>
            )}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Redeemed badge */}
      {card.redeemed && (
        <div className="absolute top-4 right-4 bg-green-500/20 text-green-400 text-xs font-bold px-3 py-1 rounded-full border border-green-500/30">
          {m.stampCard.redeemed}
        </div>
      )}
    </motion.div>
  );
}

