import { useState, useEffect, useCallback } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { useAuth } from '../hooks/useAuth';
import { useLocale } from '../hooks/useLocale';
import { toast } from 'sonner';
import {
  apiGetShops,
  apiGetMyCards,
  apiRedeemCard,
  apiJoinShop,
  type Shop,
  type StampCard as StampCardType,
} from '../lib/api';
import StampCard from '../components/stamps/StampCard';
import { Stamp, Star, Gift, TrendingUp, History, ChevronDown, Search, Store, PlusCircle, CheckCircle } from 'lucide-react';

export default function UserDashboard() {
  const { user } = useAuth();
  const { m, locale } = useLocale();
  const [shops, setShops] = useState<Shop[]>([]);
  const [cards, setCards] = useState<StampCardType[]>([]);
  const [shopSearch, setShopSearch] = useState('');
  const [showDiscover, setShowDiscover] = useState(false);
  const [joiningShopId, setJoiningShopId] = useState<string | null>(null);

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

  const handleJoinShop = async (shopId: string) => {
    setJoiningShopId(shopId);
    try {
      await apiJoinShop(shopId);
      toast.success(m.userDashboard.joinedShop);
      await refresh();
    } catch (e) {
      const msg = e instanceof Error ? e.message : m.userDashboard.errors.joinFailed;
      toast.error(msg);
    } finally {
      setJoiningShopId(null);
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

  // Shops the user has joined (has a card for)
  const joinedShopIds = new Set(cards.map((c) => c.shopId));
  const joinedShops = shops.filter((s) => joinedShopIds.has(s.id));

  // Shops available to discover (not yet joined)
  const discoverableShops = shops.filter(
    (s) =>
      !joinedShopIds.has(s.id) &&
      (shopSearch === '' ||
        s.name.toLowerCase().includes(shopSearch.toLowerCase()) ||
        s.description.toLowerCase().includes(shopSearch.toLowerCase()))
  );

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
            { icon: Star, label: m.userDashboard.stats.activeCards, value: activeCards.length, color: 'text-accent' },
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

        {/* My Cards */}
        {joinedShops.length === 0 ? (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="text-center py-16"
          >
            <Stamp className="w-16 h-16 text-indigo-600 mx-auto mb-4" />
            <h2 className="text-xl font-bold text-white mb-2">{m.userDashboard.empty.title}</h2>
            <p className="text-indigo-400 mb-6">{m.userDashboard.empty.description}</p>
            {shops.length > 0 && (
              <button
                onClick={() => setShowDiscover(true)}
                className="inline-flex items-center gap-2 bg-linear-to-r from-primary to-primary-dark text-white font-bold px-6 py-3 rounded-xl hover:scale-[1.02] transition-transform cursor-pointer"
              >
                <Search className="w-5 h-5" />
                {m.userDashboard.discoverShops}
              </button>
            )}
          </motion.div>
        ) : (
          <>
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-bold text-white flex items-center gap-2">
                <Star className="w-5 h-5 text-accent" />
                {m.userDashboard.myCardsTitle}
              </h2>
              {shops.length > joinedShops.length && (
                <button
                  onClick={() => setShowDiscover((v) => !v)}
                  className="flex items-center gap-2 bg-white/[0.07] border border-white/10 text-indigo-300 hover:text-white hover:bg-white/10 font-semibold px-4 py-2 rounded-xl transition-all cursor-pointer text-sm"
                >
                  <Search className="w-4 h-4" />
                  {m.userDashboard.discoverShops}
                </button>
              )}
            </div>

            <div className="grid sm:grid-cols-2 gap-6">
              {joinedShops.map((shop, i) => {
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
          </>
        )}

        {/* Discover Shops Section */}
        <AnimatePresence>
          {showDiscover && (
            <motion.div
              initial={{ height: 0, opacity: 0 }}
              animate={{ height: 'auto', opacity: 1 }}
              exit={{ height: 0, opacity: 0 }}
              transition={{ duration: 0.3 }}
              className="overflow-hidden mt-10"
            >
              <div className="bg-linear-to-br from-white/8 to-white/3 border border-white/10 rounded-3xl p-6 sm:p-8">
                <div className="flex items-center gap-3 mb-6">
                  <div className="w-10 h-10 bg-primary/20 rounded-xl flex items-center justify-center">
                    <Store className="w-5 h-5 text-primary-light" />
                  </div>
                  <div>
                    <h2 className="text-xl font-bold text-white">{m.userDashboard.discoverTitle}</h2>
                    <p className="text-sm text-indigo-400">{m.userDashboard.discoverDescription}</p>
                  </div>
                </div>

                {/* Search input */}
                <div className="relative mb-5">
                  <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-indigo-400" />
                  <input
                    type="text"
                    value={shopSearch}
                    onChange={(e) => setShopSearch(e.target.value)}
                    placeholder={m.userDashboard.searchPlaceholder}
                    className="w-full bg-white/5 border border-white/10 rounded-xl pl-10 pr-4 py-3 text-white placeholder:text-indigo-400/50 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all"
                  />
                </div>

                {discoverableShops.length === 0 ? (
                  <div className="text-center py-8">
                    <CheckCircle className="w-10 h-10 text-emerald-400 mx-auto mb-2" />
                    <p className="text-indigo-400">
                      {shopSearch ? m.userDashboard.noShopsMatchSearch : m.userDashboard.joinedAllShops}
                    </p>
                  </div>
                ) : (
                  <div className="grid sm:grid-cols-2 gap-4">
                    {discoverableShops.map((shop) => (
                      <motion.div
                        key={shop.id}
                        layout
                        className="relative bg-white/5 border border-white/10 rounded-2xl p-5 hover:bg-white/8 transition-colors"
                      >
                        <div className="absolute top-0 left-6 right-6 h-1 rounded-b-full" style={{ backgroundColor: shop.color }} />

                        <div className="flex items-start gap-3 mt-1">
                          <div
                            className="w-12 h-12 rounded-xl flex items-center justify-center text-white font-bold text-lg shrink-0"
                            style={{ backgroundColor: shop.color }}
                          >
                            {shop.name.charAt(0).toUpperCase()}
                          </div>
                          <div className="flex-1 min-w-0">
                            <h4 className="text-white font-bold truncate">{shop.name}</h4>
                            <p className="text-indigo-400 text-xs mt-0.5 truncate">{shop.description}</p>
                          </div>
                        </div>

                        <div className="mt-3 flex items-center gap-3 text-xs text-indigo-400">
                          <span className="bg-white/5 px-2 py-1 rounded-lg">{shop.stampsRequired} stamps</span>
                          <span className="bg-white/5 px-2 py-1 rounded-lg truncate flex-1">🎁 {shop.rewardDescription}</span>
                        </div>

                        <button
                          onClick={() => handleJoinShop(shop.id)}
                          disabled={joiningShopId === shop.id}
                          className="mt-4 w-full flex items-center justify-center gap-2 bg-linear-to-r from-primary to-primary-dark text-white font-semibold py-2.5 rounded-xl hover:scale-[1.01] active:scale-[0.99] transition-transform cursor-pointer text-sm disabled:opacity-50"
                        >
                          <PlusCircle className="w-4 h-4" />
                          {joiningShopId === shop.id ? m.userDashboard.joining : m.userDashboard.joinShop}
                        </button>
                      </motion.div>
                    ))}
                  </div>
                )}
              </div>
            </motion.div>
          )}
        </AnimatePresence>

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
