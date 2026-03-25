import { useState, useEffect, useCallback } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { useAuth } from '../hooks/useAuth';
import { toast } from 'sonner';
import {
  apiGetMyShops,
  apiCreateShop,
  apiUpdateShop,
  apiGetCustomers,
  apiGetShopCards,
  apiGrantStamp,
  apiUpdateStampCount,
  type Shop,
  type User,
  type StampCard,
} from '../lib/api';
import {
  Store,
  Save,
  Stamp,
  Users as UsersIcon,
  CheckCircle,
  Settings,
  PlusCircle,
  MinusCircle,
  Gift,
  BarChart3,
  Search,
  QrCode,
  ChevronDown,
  Pencil,
  X,
} from 'lucide-react';
import QRCodeDisplay from '../components/stamps/QRCodeDisplay';

type Tab = 'shop' | 'stamps' | 'qr' | 'stats';

export default function AdminDashboard() {
  const { user } = useAuth();
  const [activeTab, setActiveTab] = useState<Tab>('shop');
  const [saved, setSaved] = useState(false);
  const [stamped, setStamped] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');

  // Multi-shop state
  const [shops, setShops] = useState<Shop[]>([]);
  const [selectedShopId, setSelectedShopId] = useState<string | null>(null);
  const [shopSelectorOpen, setShopSelectorOpen] = useState(false);
  const [customers, setCustomers] = useState<User[]>([]);
  const [shopCards, setShopCards] = useState<StampCard[]>([]);
  // null = no modal open, 'new' = creating, string = editing shop by id
  const [editingMode, setEditingMode] = useState<string | null>(null);

  const selectedShop = shops.find((s) => s.id === selectedShopId) ?? null;
  const isModalOpen = editingMode !== null;

  // Shop form
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [rewardDescription, setRewardDescription] = useState('');
  const [stampsRequired, setStampsRequired] = useState(8);
  const [color, setColor] = useState('#6366f1');

  const populateForm = (shop: Shop | null) => {
    if (shop) {
      setName(shop.name);
      setDescription(shop.description);
      setRewardDescription(shop.rewardDescription);
      setStampsRequired(shop.stampsRequired);
      setColor(shop.color);
    } else {
      setName('');
      setDescription('');
      setRewardDescription('');
      setStampsRequired(8);
      setColor('#6366f1');
    }
  };

  const loadCardsForShop = useCallback(async (shopId: string) => {
    try {
      const cards = await apiGetShopCards(shopId);
      setShopCards(cards);
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to load cards';
      toast.error(msg);
      setShopCards([]);
    }
  }, []);

  const refresh = useCallback(async () => {
    try {
      const [fetchedShops, c] = await Promise.all([apiGetMyShops(), apiGetCustomers()]);
      setShops(fetchedShops);
      setCustomers(c);

      if (fetchedShops.length > 0) {
        const current = fetchedShops.find((s) => s.id === selectedShopId);
        const active = current ?? fetchedShops[0];
        setSelectedShopId(active.id);
        const cards = await apiGetShopCards(active.id);
        setShopCards(cards);
      } else {
        setSelectedShopId(null);
        setShopCards([]);
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to refresh data';
      toast.error(msg);
    }
  }, [selectedShopId]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [fetchedShops, c] = await Promise.all([apiGetMyShops(), apiGetCustomers()]);
        if (cancelled) return;
        setShops(fetchedShops);
        setCustomers(c);
        if (fetchedShops.length > 0) {
          const first = fetchedShops[0];
          setSelectedShopId(first.id);
          const cards = await apiGetShopCards(first.id);
          if (!cancelled) setShopCards(cards);
        }
      } catch (e) {
        if (!cancelled) {
          const msg = e instanceof Error ? e.message : 'Failed to load dashboard';
          toast.error(msg);
        }
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const handleSelectShop = (shop: Shop) => {
    setSelectedShopId(shop.id);
    setShopSelectorOpen(false);
    void loadCardsForShop(shop.id);
  };

  const handleNewShop = () => {
    setEditingMode('new');
    populateForm(null);
    setShopSelectorOpen(false);
  };

  const handleEditShop = (shop: Shop) => {
    setEditingMode(shop.id);
    setSelectedShopId(shop.id);
    populateForm(shop);
  };

  const handleSaveShop = async () => {
    if (!user) return;
    try {
      if (editingMode === 'new') {
        const created = await apiCreateShop({ name, description, rewardDescription, stampsRequired, color });
        setSelectedShopId(created.id);
        toast.success('Stamp card created!');
      } else if (editingMode && selectedShop) {
        await apiUpdateShop(selectedShop.id, { name, description, rewardDescription, stampsRequired, color });
        toast.success('Stamp card updated!');
      }
      setEditingMode(null);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
      await refresh();
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to save stamp card';
      toast.error(msg);
    }
  };

  const handleGrantStamp = async (userId: string) => {
    if (!selectedShop) return;
    try {
      await apiGrantStamp(selectedShop.id, userId);
      setStamped(userId);
      setTimeout(() => setStamped(null), 1500);
      await loadCardsForShop(selectedShop.id);
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to grant stamp';
      toast.error(msg);
    }
  };

  const handleReduceStamp = async (userId: string) => {
    if (!selectedShop) return;
    const card = getCardForUser(userId);
    const currentStamps = card?.stamps ?? 0;
    if (currentStamps <= 0) return;
    try {
      await apiUpdateStampCount(selectedShop.id, userId, currentStamps - 1);
      await loadCardsForShop(selectedShop.id);
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Failed to reduce stamp';
      toast.error(msg);
    }
  };

  const getCardForUser = (userId: string): StampCard | undefined => {
    return shopCards.find((c) => c.userId === userId && !c.redeemed);
  };

  const filteredCustomers = customers.filter((c) =>
    c.username.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const totalStampsGiven = shopCards.reduce((sum, c) => sum + c.stamps, 0);
  const completedCards = shopCards.filter((c) => selectedShop && c.stamps >= selectedShop.stampsRequired).length;
  const activeCustomers = shopCards.filter((c) => !c.redeemed && c.stamps > 0).length;

  const colorPresets = ['#6366f1', '#f59e0b', '#ef4444', '#10b981', '#ec4899', '#8b5cf6', '#06b6d4', '#f97316'];

  const tabs: { id: Tab; label: string; icon: typeof Store }[] = [
    { id: 'shop', label: 'Stamp Cards', icon: Settings },
    { id: 'stamps', label: 'Grant Stamps', icon: Stamp },
    { id: 'qr', label: 'QR Code', icon: QrCode },
    { id: 'stats', label: 'Statistics', icon: BarChart3 },
  ];

  const hasShop = selectedShop !== null;

  return (
    <div className="min-h-screen bg-surface pt-20 pb-12">
      {/* Background */}
      <div className="fixed inset-0 pointer-events-none">
        <div className="absolute top-20 left-[5%] w-80 h-80 bg-primary/10 rounded-full blur-3xl" />
        <div className="absolute bottom-20 right-[10%] w-72 h-72 bg-accent/8 rounded-full blur-3xl" />
      </div>

      <div className="relative max-w-5xl mx-auto px-4">
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="mb-6"
        >
          <h1 className="text-3xl sm:text-4xl font-black text-white">
            Shop <span className="text-accent">Dashboard</span> 🏪
          </h1>
          <p className="text-indigo-300 mt-2">
            {shops.length === 0
              ? 'Create your first stamp card to get started'
              : `Managing ${shops.length} stamp card${shops.length > 1 ? 's' : ''}`}
          </p>
        </motion.div>

        {/* ── Shop Selector ── */}
        {shops.length > 0 && (
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            className="mb-6 relative"
          >
            <div className="flex items-center gap-3">
              <button
                onClick={() => setShopSelectorOpen(!shopSelectorOpen)}
                className="flex items-center gap-3 bg-white/[0.07] border border-white/10 rounded-2xl px-4 py-3 hover:bg-white/10 transition-colors cursor-pointer flex-1 sm:flex-none"
              >
                {hasShop ? (
                  <>
                    <div
                      className="w-8 h-8 rounded-lg flex items-center justify-center text-white font-bold text-sm shrink-0"
                      style={{ backgroundColor: selectedShop.color }}
                    >
                      {selectedShop.name.charAt(0).toUpperCase()}
                    </div>
                    <div className="text-left">
                      <div className="text-white font-semibold text-sm">{selectedShop.name}</div>
                      <div className="text-indigo-400 text-xs">{selectedShop.stampsRequired} stamps required</div>
                    </div>
                  </>
                ) : (
                  <span className="text-indigo-400 text-sm font-medium">Select a shop…</span>
                )}
                <ChevronDown className={`w-4 h-4 text-indigo-400 ml-auto transition-transform ${shopSelectorOpen ? 'rotate-180' : ''}`} />
              </button>
            </div>

            {/* Dropdown */}
            <AnimatePresence>
              {shopSelectorOpen && (
                <motion.div
                  initial={{ opacity: 0, y: -8, scale: 0.97 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: -8, scale: 0.97 }}
                  transition={{ duration: 0.15 }}
                  className="absolute z-30 top-full left-0 mt-2 w-full sm:w-80 bg-surface border border-white/10 rounded-2xl shadow-2xl overflow-hidden"
                >
                  {shops.map((s) => (
                    <button
                      key={s.id}
                      onClick={() => handleSelectShop(s)}
                      className={`w-full flex items-center gap-3 px-4 py-3 hover:bg-white/10 transition-colors cursor-pointer ${
                        s.id === selectedShopId ? 'bg-white/[0.07]' : ''
                      }`}
                    >
                      <div
                        className="w-8 h-8 rounded-lg flex items-center justify-center text-white font-bold text-sm shrink-0"
                        style={{ backgroundColor: s.color }}
                      >
                        {s.name.charAt(0).toUpperCase()}
                      </div>
                      <div className="text-left flex-1 min-w-0">
                        <div className="text-white font-semibold text-sm truncate">{s.name}</div>
                        <div className="text-indigo-400 text-xs truncate">{s.description || s.rewardDescription}</div>
                      </div>
                      {s.id === selectedShopId && (
                        <CheckCircle className="w-4 h-4 text-accent shrink-0" />
                      )}
                    </button>
                  ))}
                </motion.div>
              )}
            </AnimatePresence>
          </motion.div>
        )}

        {/* Tabs */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="flex gap-2 mb-8 overflow-x-auto pb-2"
        >
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-2 px-4 py-2.5 rounded-xl text-sm font-semibold whitespace-nowrap transition-all cursor-pointer ${
                activeTab === tab.id
                  ? 'bg-primary text-white shadow-lg shadow-primary/25'
                  : 'bg-white/5 text-indigo-300 hover:bg-white/10 hover:text-white border border-white/10'
              }`}
            >
              <tab.icon className="w-4 h-4" />
              {tab.label}
            </button>
          ))}
        </motion.div>

        <AnimatePresence mode="wait">
          {/* ── Shop Settings Tab (Card Grid only — form is a modal) ── */}
          {activeTab === 'shop' && (
            <motion.div
              key="shop"
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 20 }}
            >
              {shops.length === 0 ? (
                /* No shops yet */
                <div className="bg-linear-to-br from-white/8 to-white/3 border border-white/10 rounded-3xl p-10 text-center">
                  <Store className="w-14 h-14 text-indigo-500 mx-auto mb-4" />
                  <h3 className="text-xl font-bold text-white mb-2">No stamp cards yet</h3>
                  <p className="text-indigo-400 mb-6 max-w-sm mx-auto">Create your first stamp card to start managing rewards for your customers.</p>
                  <button
                    onClick={handleNewShop}
                    className="inline-flex items-center gap-2 bg-linear-to-r from-accent to-amber-400 text-surface font-bold px-6 py-3 rounded-xl hover:scale-[1.02] transition-transform cursor-pointer"
                  >
                    <PlusCircle className="w-5 h-5" />
                    Create Your First Stamp Card
                  </button>
                </div>
              ) : (
                /* Shop cards */
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <h3 className="text-lg font-bold text-white flex items-center gap-2">
                      <Store className="w-5 h-5 text-primary-light" />
                      My Stamp Cards ({shops.length})
                    </h3>
                    <button
                      onClick={handleNewShop}
                      className="flex items-center gap-2 bg-linear-to-r from-accent to-amber-400 text-surface font-bold px-4 py-2 rounded-xl hover:scale-[1.02] transition-transform cursor-pointer text-sm"
                    >
                      <PlusCircle className="w-4 h-4" />
                      Create Stamp Card
                    </button>
                  </div>

                  <div className="grid gap-4 sm:grid-cols-2">
                    {shops.map((s) => (
                      <motion.div
                        key={s.id}
                        layout
                        className={`relative bg-linear-to-br from-white/8 to-white/3 border rounded-2xl p-5 transition-all ${
                          s.id === selectedShopId
                            ? 'border-white/20 shadow-lg'
                            : 'border-white/10'
                        }`}
                      >
                        {/* Shop color accent bar */}
                        <div className="absolute top-0 left-6 right-6 h-1 rounded-b-full" style={{ backgroundColor: s.color }} />

                        <div className="flex items-start gap-3 mt-1">
                          <div
                            className="w-12 h-12 rounded-xl flex items-center justify-center text-white font-bold text-lg shrink-0"
                            style={{ backgroundColor: s.color }}
                          >
                            {s.name.charAt(0).toUpperCase()}
                          </div>
                          <div className="flex-1 min-w-0">
                            <h4 className="text-white font-bold truncate">{s.name}</h4>
                            <p className="text-indigo-400 text-xs mt-0.5 truncate">{s.description || 'No description'}</p>
                          </div>
                        </div>

                        <div className="mt-3 flex items-center gap-3 text-xs text-indigo-400">
                          <span className="bg-white/5 px-2 py-1 rounded-lg">{s.stampsRequired} stamps</span>
                          <span className="bg-white/5 px-2 py-1 rounded-lg truncate flex-1">🎁 {s.rewardDescription}</span>
                        </div>

                        <div className="mt-4 flex items-center gap-2">
                          <button
                            onClick={() => handleSelectShop(s)}
                            className={`flex-1 text-center text-sm font-semibold py-2 rounded-xl transition-all cursor-pointer ${
                              s.id === selectedShopId
                                ? 'bg-primary/20 text-primary-light'
                                : 'bg-white/5 text-indigo-300 hover:bg-white/10'
                            }`}
                          >
                            {s.id === selectedShopId ? '✓ Selected' : 'Select'}
                          </button>
                          <button
                            onClick={() => handleEditShop(s)}
                            className="flex items-center gap-1.5 px-3 py-2 text-sm font-semibold bg-white/5 text-indigo-300 hover:bg-white/10 hover:text-white rounded-xl transition-all cursor-pointer"
                          >
                            <Pencil className="w-3.5 h-3.5" />
                            Edit
                          </button>
                        </div>
                      </motion.div>
                    ))}
                  </div>
                </div>
              )}
            </motion.div>
          )}

          {/* ── Grant Stamps Tab ── */}
          {activeTab === 'stamps' && (
            <motion.div
              key="stamps"
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 20 }}
            >
              {!hasShop ? (
                <div className="bg-linear-to-br from-white/8 to-white/3 border border-white/10 rounded-3xl p-8 text-center">
                  <Store className="w-12 h-12 text-indigo-500 mx-auto mb-3" />
                  <h3 className="text-lg font-bold text-white mb-2">No shop selected</h3>
                  <p className="text-indigo-400 mb-4">Create or select a shop first.</p>
                  <button
                    onClick={() => setActiveTab('shop')}
                    className="inline-flex items-center gap-2 bg-primary text-white font-semibold px-5 py-2.5 rounded-xl hover:bg-primary-dark transition-colors cursor-pointer"
                  >
                    <PlusCircle className="w-4 h-4" />
                    Set Up Shop
                  </button>
                </div>
              ) : (
                <div className="bg-linear-to-br from-white/8 to-white/3 backdrop-blur-xl border border-white/10 rounded-3xl p-6 sm:p-8">
                  <div className="flex items-center gap-3 mb-6">
                    <div className="w-10 h-10 bg-accent/20 rounded-xl flex items-center justify-center">
                      <Stamp className="w-5 h-5 text-accent" />
                    </div>
                    <div>
                      <h2 className="text-xl font-bold text-white">Grant Stamps</h2>
                      <p className="text-sm text-indigo-400">
                        Managing stamps for <span className="text-white font-medium">{selectedShop.name}</span>
                      </p>
                    </div>
                  </div>

                  {/* Search */}
                  <div className="relative mb-5">
                    <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-indigo-400" />
                    <input
                      type="text"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      placeholder="Search customers..."
                      className="w-full bg-white/5 border border-white/10 rounded-xl pl-10 pr-4 py-3 text-white placeholder:text-indigo-400/50 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all"
                    />
                  </div>

                  {filteredCustomers.length === 0 ? (
                    <div className="text-center py-10">
                      <UsersIcon className="w-10 h-10 text-indigo-600 mx-auto mb-2" />
                      <p className="text-indigo-400">No customers found</p>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {filteredCustomers.map((customer) => {
                        const card = getCardForUser(customer.id);
                        const stamps = card?.stamps ?? 0;
                        const progress = (stamps / selectedShop.stampsRequired) * 100;
                        const isComplete = stamps >= selectedShop.stampsRequired;

                        return (
                          <motion.div
                            key={customer.id}
                            layout
                            className="flex items-center gap-4 bg-white/5 border border-white/10 rounded-2xl p-4 hover:bg-white/8 transition-colors"
                          >
                            {/* Avatar */}
                            <div
                              className="w-11 h-11 rounded-xl flex items-center justify-center text-white font-bold text-lg shrink-0"
                              style={{ backgroundColor: selectedShop.color + '44' }}
                            >
                              {customer.username.charAt(0).toUpperCase()}
                            </div>

                            {/* Info */}
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center gap-2">
                                <span className="font-semibold text-white truncate">
                                  {customer.username}
                                </span>
                                {isComplete && (
                                  <span className="text-xs bg-accent/20 text-accent px-2 py-0.5 rounded-full font-medium">
                                    Complete!
                                  </span>
                                )}
                              </div>
                              <div className="flex items-center gap-2 mt-1">
                                <div className="flex-1 h-1.5 bg-white/10 rounded-full overflow-hidden">
                                  <div
                                    className="h-full rounded-full transition-all duration-500"
                                    style={{
                                      width: `${Math.min(progress, 100)}%`,
                                      backgroundColor: selectedShop.color,
                                    }}
                                  />
                                </div>
                                <span className="text-xs text-indigo-400 whitespace-nowrap">
                                  {stamps}/{selectedShop.stampsRequired}
                                </span>
                              </div>
                            </div>

                            {/* Stamp buttons */}
                            <div className="shrink-0 flex items-center gap-1.5">
                              <button
                                onClick={() => handleReduceStamp(customer.id)}
                                disabled={stamps <= 0}
                                className={`flex items-center gap-1 px-3 py-2 rounded-xl text-sm font-semibold transition-all cursor-pointer ${
                                  stamps <= 0
                                    ? 'bg-white/5 text-indigo-500 cursor-not-allowed'
                                    : 'bg-red-500/15 text-red-400 hover:bg-red-500/25'
                                }`}
                                title="Remove stamp"
                              >
                                <MinusCircle className="w-4 h-4" />
                              </button>
                              <button
                                onClick={() => handleGrantStamp(customer.id)}
                                disabled={isComplete}
                                className={`flex items-center gap-1.5 px-4 py-2 rounded-xl text-sm font-semibold transition-all cursor-pointer ${
                                  stamped === customer.id
                                    ? 'bg-emerald-500/20 text-emerald-400'
                                    : isComplete
                                    ? 'bg-white/5 text-indigo-500 cursor-not-allowed'
                                    : 'bg-accent/20 text-accent hover:bg-accent/30'
                                }`}
                              >
                                {stamped === customer.id ? (
                                  <>
                                    <CheckCircle className="w-4 h-4" />
                                    Done
                                  </>
                                ) : (
                                  <>
                                    <PlusCircle className="w-4 h-4" />
                                    Stamp
                                  </>
                                )}
                              </button>
                            </div>
                          </motion.div>
                        );
                      })}
                    </div>
                  )}
                </div>
              )}
            </motion.div>
          )}

          {/* ── QR Code Tab ── */}
          {activeTab === 'qr' && (
            <motion.div
              key="qr"
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 20 }}
            >
              {!hasShop ? (
                <div className="bg-linear-to-br from-white/8 to-white/3 border border-white/10 rounded-3xl p-8 text-center">
                  <Store className="w-12 h-12 text-indigo-500 mx-auto mb-3" />
                  <h3 className="text-lg font-bold text-white mb-2">No shop selected</h3>
                  <p className="text-indigo-400 mb-4">Create or select a shop first.</p>
                  <button
                    onClick={() => setActiveTab('shop')}
                    className="inline-flex items-center gap-2 bg-primary text-white font-semibold px-5 py-2.5 rounded-xl hover:bg-primary-dark transition-colors cursor-pointer"
                  >
                    <PlusCircle className="w-4 h-4" />
                    Set Up Shop
                  </button>
                </div>
              ) : (
                <div className="bg-linear-to-br from-white/8 to-white/3 backdrop-blur-xl border border-white/10 rounded-3xl p-6 sm:p-8">
                  <div className="flex items-center gap-3 mb-6">
                    <div className="w-10 h-10 bg-primary/20 rounded-xl flex items-center justify-center">
                      <QrCode className="w-5 h-5 text-primary-light" />
                    </div>
                    <div>
                      <h2 className="text-xl font-bold text-white">QR Code Stamps</h2>
                      <p className="text-sm text-indigo-400">
                        Generate a QR code for <span className="text-white font-medium">{selectedShop.name}</span>
                      </p>
                    </div>
                  </div>
                  <QRCodeDisplay shopId={selectedShop.id} shopColor={selectedShop.color} shopName={selectedShop.name} />
                </div>
              )}
            </motion.div>
          )}

          {/* ── Statistics Tab ── */}
          {activeTab === 'stats' && (
            <motion.div
              key="stats"
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 20 }}
            >
              {!hasShop ? (
                <div className="bg-linear-to-br from-white/8 to-white/3 border border-white/10 rounded-3xl p-8 text-center">
                  <BarChart3 className="w-12 h-12 text-indigo-500 mx-auto mb-3" />
                  <h3 className="text-lg font-bold text-white mb-2">No data yet</h3>
                  <p className="text-indigo-400">Create or select a shop to see statistics.</p>
                </div>
              ) : (
                <div className="space-y-6">
                  {/* Stat cards */}
                  <div className="grid grid-cols-2 sm:grid-cols-3 gap-4">
                    {[
                      { label: 'Total Stamps Given', value: totalStampsGiven, icon: Stamp, color: 'text-accent' },
                      { label: 'Completed Cards', value: completedCards, icon: CheckCircle, color: 'text-emerald-400' },
                      { label: 'Active Customers', value: activeCustomers, icon: UsersIcon, color: 'text-primary-light' },
                    ].map((stat, i) => (
                      <motion.div
                        key={i}
                        initial={{ opacity: 0, y: 20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ delay: i * 0.1 }}
                        className="bg-linear-to-br from-white/8 to-white/3 border border-white/10 rounded-2xl p-5"
                      >
                        <stat.icon className={`w-6 h-6 ${stat.color} mb-2`} />
                        <div className="text-3xl font-bold text-white">{stat.value}</div>
                        <div className="text-sm text-indigo-400 mt-1">{stat.label}</div>
                      </motion.div>
                    ))}
                  </div>

                  {/* Customer breakdown */}
                  <div className="bg-linear-to-br from-white/8 to-white/3 border border-white/10 rounded-3xl p-6 sm:p-8">
                    <h3 className="text-lg font-bold text-white mb-4">
                      Customer Progress — {selectedShop.name}
                    </h3>
                    {shopCards.length === 0 ? (
                      <p className="text-indigo-400 text-center py-8">No customer activity yet</p>
                    ) : (
                      <div className="space-y-3">
                        {shopCards
                          .sort((a, b) => b.stamps - a.stamps)
                          .map((card) => {
                            const customer = customers.find((c) => c.id === card.userId);
                            if (!customer) return null;
                            const progress = (card.stamps / selectedShop.stampsRequired) * 100;
                            return (
                              <div key={card.id} className="flex items-center gap-3">
                                <div
                                  className="w-8 h-8 rounded-lg flex items-center justify-center text-white text-sm font-bold shrink-0"
                                  style={{ backgroundColor: selectedShop.color + '44' }}
                                >
                                  {customer.username.charAt(0).toUpperCase()}
                                </div>
                                <div className="flex-1 min-w-0">
                                  <div className="flex items-center justify-between mb-1">
                                    <span className="text-sm font-medium text-white truncate">
                                      {customer.username}
                                    </span>
                                    <span className="text-xs text-indigo-400">
                                      {card.stamps}/{selectedShop.stampsRequired}
                                      {card.redeemed && ' ✓'}
                                    </span>
                                  </div>
                                  <div className="h-1.5 bg-white/10 rounded-full overflow-hidden">
                                    <motion.div
                                      initial={{ width: 0 }}
                                      animate={{ width: `${Math.min(progress, 100)}%` }}
                                      transition={{ duration: 0.8, ease: 'easeOut' }}
                                      className="h-full rounded-full"
                                      style={{ backgroundColor: selectedShop.color }}
                                    />
                                  </div>
                                </div>
                              </div>
                            );
                          })}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      {/* ══════════════ Modal Overlay for Create / Edit Stamp Card ══════════════ */}
      <AnimatePresence>
        {isModalOpen && (
          <motion.div
            key="modal-backdrop"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
            onClick={() => setEditingMode(null)}
          >
            <motion.div
              initial={{ opacity: 0, scale: 0.95, y: 20 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 20 }}
              transition={{ type: 'spring', stiffness: 300, damping: 25 }}
              className="bg-surface border border-white/10 rounded-3xl p-6 sm:p-8 w-full max-w-lg max-h-[90vh] overflow-y-auto shadow-2xl"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-center justify-between mb-6">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 bg-primary/20 rounded-xl flex items-center justify-center">
                    <Store className="w-5 h-5 text-primary-light" />
                  </div>
                  <div>
                    <h2 className="text-xl font-bold text-white">
                      {editingMode === 'new' ? 'Create Stamp Card' : 'Edit Stamp Card'}
                    </h2>
                    <p className="text-sm text-indigo-400">
                      {editingMode === 'new' ? 'Set up a new stamp card with its rewards' : `Editing ${selectedShop?.name ?? ''}`}
                    </p>
                  </div>
                </div>
                <button
                  onClick={() => setEditingMode(null)}
                  className="text-indigo-400 hover:text-white transition-colors cursor-pointer p-2 rounded-xl hover:bg-white/10"
                  title="Close"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              <form action={handleSaveShop} className="space-y-5">
                <div className="grid sm:grid-cols-2 gap-5">
                  <div>
                    <label className="block text-sm font-medium text-indigo-200 mb-1.5">Shop Name</label>
                    <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="My Awesome Shop" required className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white placeholder:text-indigo-400/50 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all" />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-indigo-200 mb-1.5">Stamps Required</label>
                    <input type="number" value={stampsRequired} onChange={(e) => setStampsRequired(Math.max(2, Math.min(20, +e.target.value)))} min={2} max={20} className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all" />
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-medium text-indigo-200 mb-1.5">Description</label>
                  <textarea value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Tell customers about your shop..." rows={2} className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white placeholder:text-indigo-400/50 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all resize-none" />
                </div>

                <div>
                  <label className="block text-sm font-medium text-indigo-200 mb-1.5">
                    <Gift className="w-4 h-4 inline mr-1" />
                    Reward Description
                  </label>
                  <input type="text" value={rewardDescription} onChange={(e) => setRewardDescription(e.target.value)} placeholder="e.g. 1 free coffee of your choice!" required className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white placeholder:text-indigo-400/50 focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all" />
                </div>

                <div>
                  <label className="block text-sm font-medium text-indigo-200 mb-1.5">Brand Color</label>
                  <div className="flex items-center gap-3 flex-wrap">
                    {colorPresets.map((c) => (
                      <button key={c} type="button" onClick={() => setColor(c)} className={`w-9 h-9 rounded-xl transition-all cursor-pointer ${color === c ? 'ring-2 ring-white ring-offset-2 ring-offset-surface scale-110' : 'hover:scale-110'}`} style={{ backgroundColor: c }} />
                    ))}
                    <input type="color" value={color} onChange={(e) => setColor(e.target.value)} className="w-9 h-9 rounded-xl cursor-pointer bg-transparent border border-white/20" />
                  </div>
                </div>

                <div className="flex items-center gap-3 pt-2">
                  <button type="submit" className="flex items-center gap-2 bg-linear-to-r from-accent to-amber-400 text-surface font-bold px-6 py-3 rounded-xl hover:scale-[1.02] transition-transform cursor-pointer">
                    <Save className="w-4 h-4" />
                    {editingMode === 'new' ? 'Create Stamp Card' : 'Save Changes'}
                  </button>
                  <button type="button" onClick={() => setEditingMode(null)} className="text-indigo-400 hover:text-white text-sm font-medium transition-colors cursor-pointer">
                    Cancel
                  </button>
                  <AnimatePresence>
                    {saved && (
                      <motion.span initial={{ opacity: 0, x: -10 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0 }} className="flex items-center gap-1 text-emerald-400 text-sm font-medium">
                        <CheckCircle className="w-4 h-4" />
                        Saved!
                      </motion.span>
                    )}
                  </AnimatePresence>
                </div>
              </form>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

