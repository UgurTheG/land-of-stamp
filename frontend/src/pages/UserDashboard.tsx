import { useState, useEffect, useCallback } from 'react';
import { motion } from 'motion/react';
import { useNavigate } from 'react-router';
import { useAuth } from '../hooks/useAuth';
import { toast } from 'sonner';
import {
  apiGetShops,
  apiGetMyCards,
  apiRedeemCard,
  type Shop,
  type StampCard as StampCardType,
} from '../lib/api';
import StampCard from '../components/stamps/StampCard';
import { Stamp, Star, Gift, TrendingUp, ScanLine } from 'lucide-react';

export default function UserDashboard() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [shops, setShops] = useState<Shop[]>([]);
  const [cards, setCards] = useState<StampCardType[]>([]);

  const refresh = useCallback(async () => {
    try {
      const [s, c] = await Promise.all([apiGetShops(), apiGetMyCards()]);
      setShops(s);
      setCards(c);
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to refresh data';
      toast.error(msg);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [s, c] = await Promise.all([apiGetShops(), apiGetMyCards()]);
        if (!cancelled) { setShops(s); setCards(c); }
      } catch (e) {
        if (!cancelled) {
          const msg = e instanceof Error ? e.message : 'Failed to load dashboard';
          toast.error(msg);
        }
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const handleRedeem = async (cardId: string) => {
    try {
      await apiRedeemCard(cardId);
      await refresh();
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to redeem card';
      toast.error(msg);
    }
  };

  const totalStamps = cards.reduce((sum, c) => sum + c.stamps, 0);
  const completedCards = cards.filter((c) => {
    const shop = shops.find((s) => s.id === c.shopId);
    return shop && c.stamps >= shop.stampsRequired;
  }).length;
  const redeemedCards = cards.filter((c) => c.redeemed).length;

  return (
    <div className="min-h-screen bg-surface pt-20 pb-12">
      {/* Background */}
      <div className="fixed inset-0 pointer-events-none">
        <div className="absolute top-20 right-[5%] w-80 h-80 bg-primary/10 rounded-full blur-3xl" />
        <div className="absolute bottom-20 left-[10%] w-72 h-72 bg-accent/8 rounded-full blur-3xl" />
      </div>

      <div className="relative max-w-6xl mx-auto px-4">
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="mb-8 flex items-start justify-between"
        >
          <div>
            <h1 className="text-3xl sm:text-4xl font-black text-white">
              Welcome back, <span className="text-accent">{user?.username}</span>! 👋
            </h1>
            <p className="text-indigo-300 mt-2">Here are your loyalty stamp cards</p>
          </div>
          <button
            onClick={() => navigate('/scan')}
            className="shrink-0 flex items-center gap-2 bg-gradient-to-r from-accent to-amber-400 text-surface font-bold px-5 py-3 rounded-2xl hover:scale-[1.03] active:scale-[0.98] transition-transform cursor-pointer shadow-lg shadow-accent/30"
          >
            <ScanLine className="w-5 h-5" />
            <span className="hidden sm:inline">Scan QR</span>
          </button>
        </motion.div>

        {/* Stats */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-10"
        >
          {[
            { icon: Stamp, label: 'Total Stamps', value: totalStamps, color: 'text-primary-light' },
            { icon: Star, label: 'Active Cards', value: cards.filter((c) => !c.redeemed).length, color: 'text-accent' },
            { icon: TrendingUp, label: 'Completed', value: completedCards, color: 'text-emerald-400' },
            { icon: Gift, label: 'Redeemed', value: redeemedCards, color: 'text-rose-400' },
          ].map((stat, i) => (
            <div
              key={i}
              className="bg-gradient-to-br from-white/[0.07] to-white/[0.02] border border-white/10 rounded-2xl p-4"
            >
              <stat.icon className={`w-5 h-5 ${stat.color} mb-2`} />
              <div className="text-2xl font-bold text-white">{stat.value}</div>
              <div className="text-xs text-indigo-400">{stat.label}</div>
            </div>
          ))}
        </motion.div>

        {/* Cards grid */}
        {shops.length === 0 ? (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="text-center py-20"
          >
            <Stamp className="w-16 h-16 text-indigo-600 mx-auto mb-4" />
            <h2 className="text-xl font-bold text-white mb-2">No shops available yet</h2>
            <p className="text-indigo-400">Check back soon — new partner shops are joining every day!</p>
          </motion.div>
        ) : (
          <div className="grid sm:grid-cols-2 gap-6">
            {shops.map((shop, i) => {
              const card = cards.find((c) => c.shopId === shop.id && !c.redeemed) ||
                cards.find((c) => c.shopId === shop.id);
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
      </div>
    </div>
  );
}
