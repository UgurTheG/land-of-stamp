import { motion } from 'motion/react';
import { Check, Gift } from 'lucide-react';

interface Props {
  index: number;
  filled: boolean;
  isRewardSlot: boolean;
  color: string;
}

export default function StampSlot({ index, filled, isRewardSlot, color }: Props) {
  return (
    <motion.div
      className={`relative aspect-square rounded-2xl border-2 border-dashed flex items-center justify-center transition-colors duration-300 ${
        filled
          ? 'border-transparent'
          : isRewardSlot
          ? 'border-accent/40 bg-accent/5'
          : 'border-white/20 bg-white/5'
      }`}
      whileHover={{ scale: 1.05 }}
    >
      {filled ? (
        <motion.div
          initial={{ scale: 0, rotate: -180 }}
          animate={{ scale: 1, rotate: 0 }}
          transition={{ type: 'spring', stiffness: 260, damping: 20, delay: index * 0.05 }}
          className="w-full h-full rounded-2xl flex items-center justify-center"
          style={{
            background: isRewardSlot
              ? 'linear-gradient(135deg, #f59e0b, #ef4444)'
              : `linear-gradient(135deg, ${color}, ${color}88)`,
          }}
        >
          {isRewardSlot ? (
            <Gift className="w-5 h-5 sm:w-6 sm:h-6 text-white" />
          ) : (
            <Check className="w-5 h-5 sm:w-6 sm:h-6 text-white" />
          )}
        </motion.div>
      ) : (
        <span className="text-xs text-white/30 font-medium">
          {isRewardSlot ? <Gift className="w-4 h-4 text-accent/40" /> : index + 1}
        </span>
      )}
    </motion.div>
  );
}

