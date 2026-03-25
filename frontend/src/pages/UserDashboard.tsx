import { useState, useEffect, useCallback } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { useAuth } from '../hooks/useAuth';
import { useLocale } from '../hooks/useLocale';
import { toast } from 'sonner';
import {
  apiGetShops,
  apiGetMyCards,
  apiRedeemCard,
  type Shop,
  type StampCard as StampCardType,
} from '../lib/api';
import StampCard from '../components/stamps/StampCard';
import { Stamp, Star, Gift, TrendingUp, History, ChevronDown } from 'lucide-react';

export default function UserDashboard() {
  const { user } = useAuth();
  const { m, locale } = useLocale();
  const [shops, setShops] = useState<Shop[]>([]);
  const [cards, setCards] = useState<StampCardType[]>([]);

  const refresh = useCallback(async () => {
    try {
      const [s, c] = await Promise.all([apiGetShops(), apiGetMyCards()]);
      setShops(s);
      setCards(c);
    } catch (e) {
      const msg = e instanceof Error ? e.message : m.userDashboard.errors.refreshFailed;
      toast.error(msg);
    }
  }, [m.userDashboard.errors.refreshFailed]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [s, c] = await Promise.all([apiGetShops(), apiGetMyCards()]);
        if (!cancelled) { setShops(s); setCards(c); }
      } catch (e) {
        if (!cancelled) {
          const msg = e instanceof Error ? e.message : m.userDashboard.errors.loadFailed;
          toast.error(msg);
        }
      }
    })();
    return () => { cancelled = true; };
  }, [m.userDashboard.errors.loadFailed]);

  const handleRedeem = async (cardId: string) => {
    try {
      await apiRedeemCard(cardId);
      await refresh();
    } catch (e) {
      const msg = e instanceof Error ? e.message : m.userDashboard.errors.redeemFailed;
      toast.error(msg);
    }
  };

  const totalStamps = cards.reduce((sum, c) => sum + c.stamps, 0);
  const activeCards = cards.filter((c) => !c.redeemed);
  const redeemedCardsList = cards.filter((c) => c.redeemed);
  const completedCards = activeCards.filter((c) => {
    const shop = shops.find((s) => s.id === c.shopId);
    return shop && c.stamps >= shop.stampsRequired;
  }).length;
  const redeemedCards = redeemedCardsList.length;
  const [showHistory, setShowHistory] = useState(false);

  return (
    <div className="min-h-screen pt-20 pb-12">

      <div className="relative max-w-6xl mx-auto px-4">
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="mb-8"
        >
          <div>
            <h1 className="text-3xl sm:text-4xl font-black text-white">
              {m.userDashboard.welcomeBack(user?.username ?? '')}
            </h1>
            <p className="text-indigo-300 mt-2">{m.userDashboard.subtitle}</p>
          </div>
        </motion.div>

        {/* Stats */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-10"
        >
          {[
            { icon: Stamp, label: m.userDashboard.stats.totalStamps, value: totalStamps, color: 'text-primary-light' },
            { icon: Star, label: m.userDashboard.stats.activeCards, value: cards.filter((c) => !c.redeemed).length, color: 'text-accent' },
            { icon: TrendingUp, label: m.userDashboard.stats.completed, value: completedCards, color: 'text-emerald-400' },
            { icon: Gift, label: m.userDashboard.stats.redeemed, value: redeemedCards, color: 'text-rose-400' },
          ].map((stat, i) => (
            <div
              key={i}
              className="bg-linear-to-br from-white/7 to-white/2 border border-white/10 rounded-2xl p-4"
            >
              <stat.icon className={`w-5 h-5 ${stat.color} mb-2`} />
              <div className="text-2xl font-bold text-white">{stat.value}</div>
              <div className="text-xs text-indigo-400">{stat.label}</div>
            </div>
          ))}
        </motion.div>

        {/* Active Cards */}
        {shops.length === 0 ? (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="text-center py-20"
          >
            <Stamp className="w-16 h-16 text-indigo-600 mx-auto mb-4" />
            <h2 className="text-xl font-bold text-white mb-2">{m.userDashboard.empty.title}</h2>
            <p className="text-indigo-400">{m.userDashboard.empty.description}</p>
          </motion.div>
        ) : (
          <div className="grid sm:grid-cols-2 gap-6">
            {shops.map((shop, i) => {
              const card = activeCards.find((c) => c.shopId === shop.id);
              if (!card) return null;
              return (
                <motion.div
                  key={shop.id}
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.15 + i * 0.1 }}
                >
                  <StampCard
                    shop={shop}
                    card={card}
                    onRedeem={() => handleRedeem(card.id)}
                  />
                </motion.div>
              );
            })}
          </div>
        )}

        {/* Redeemed History */}
        {redeemedCardsList.length > 0 && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 0.3 }}
            className="mt-10"
          >
            <button
              onClick={() => setShowHistory((v) => !v)}
              className="flex items-center gap-2 text-indigo-300 hover:text-white transition-colors mb-4 cursor-pointer"
            >
              <History className="w-5 h-5" />
              <span className="font-semibold">{m.userDashboard.redeemedRewards(redeemedCardsList.length)}</span>
              <motion.div animate={{ rotate: showHistory ? 180 : 0 }} transition={{ duration: 0.2 }}>
                <ChevronDown className="w-4 h-4" />
              </motion.div>
            </button>

            <AnimatePresence>
              {showHistory && (
                <motion.div
                  initial={{ height: 0, opacity: 0 }}
                  animate={{ height: 'auto', opacity: 1 }}
                  exit={{ height: 0, opacity: 0 }}
                  transition={{ duration: 0.3 }}
                  className="overflow-hidden"
                >
                  <div className="grid sm:grid-cols-2 gap-4">
                    {redeemedCardsList.map((card) => {
                      const shop = shops.find((s) => s.id === card.shopId);
                      if (!shop) return null;
                      return (
                        <div
                          key={card.id}
                          className="relative rounded-2xl bg-white/5 border border-white/10 p-4 opacity-60"
                        >
                          <div className="flex items-center justify-between">
                            <div>
                              <h4 className="text-sm font-bold text-white">{shop.name}</h4>
                              <p className="text-xs text-indigo-400 mt-0.5">{shop.rewardDescription}</p>
                            </div>
                            <div className="bg-green-500/20 text-green-400 text-xs font-bold px-2.5 py-1 rounded-full border border-green-500/30">
                              {m.userDashboard.redeemedBadge}
                            </div>
                          </div>
                          {card.createdAt && (
                            <p className="text-xs text-indigo-500 mt-2">
                              {new Date(card.createdAt).toLocaleDateString(locale === 'de' ? 'de-DE' : 'en-US')}
                            </p>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </motion.div>
        )}
      </div>
    </div>
  );
}
