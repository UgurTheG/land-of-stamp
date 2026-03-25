import { motion, AnimatePresence } from 'motion/react';
import { Trophy, PartyPopper } from 'lucide-react';
import StampSlot from './StampSlot';
import { useLocale } from '../../hooks/useLocale';
import type { Shop, StampCard as StampCardType } from '../../lib/api';

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
      className="relative overflow-hidden rounded-3xl bg-linear-to-br from-white/10 to-white/5 backdrop-blur-sm border border-white/10 p-5 sm:p-6"
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
            style={{
              background: `linear-gradient(90deg, ${shop.color}, ${shop.color}cc)`,
            }}
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
            className="absolute inset-0 bg-surface/90 backdrop-blur-sm flex flex-col items-center justify-center p-6 rounded-3xl"
          >
            <motion.div
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
              transition={{ type: 'spring', stiffness: 200, damping: 15, delay: 0.2 }}
            >
              <div className="w-20 h-20 bg-linear-to-br from-accent to-amber-400 rounded-full flex items-center justify-center mb-4 mx-auto">
                <Trophy className="w-10 h-10 text-surface" />
              </div>
            </motion.div>
            <motion.h3
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.4 }}
              className="text-xl font-bold text-white text-center mb-2"
            >
              {m.stampCard.congratulations}
            </motion.h3>
            <motion.p
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.5 }}
              className="text-indigo-300 text-center mb-4"
            >
              {m.stampCard.earned} <span className="text-accent font-semibold">{shop.rewardDescription}</span>
            </motion.p>
            {onRedeem && (
              <motion.button
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: 0.6 }}
                onClick={onRedeem}
                className="bg-linear-to-r from-accent to-amber-400 text-surface font-bold px-6 py-3 rounded-xl hover:scale-105 transition-transform cursor-pointer"
              >
                {m.stampCard.redeemReward}
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

