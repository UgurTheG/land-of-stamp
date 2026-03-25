import { useRef } from 'react';
import { Link } from 'react-router';
import { motion, useScroll, useTransform } from 'motion/react';
import {
  Stamp,
  Gift,
  Shield,
  Smartphone,
  Star,
  ArrowRight,
  ChevronDown,
  Sparkles,
  Users,
  Store,
} from 'lucide-react';

export default function LandingPage() {
  const containerRef = useRef<HTMLDivElement>(null);
  const { scrollYProgress } = useScroll({ target: containerRef });

  const bgY = useTransform(scrollYProgress, [0, 1], ['0%', '50%']);
  const midY = useTransform(scrollYProgress, [0, 1], ['0%', '25%']);
  const opacity = useTransform(scrollYProgress, [0, 0.3], [1, 0]);
  const scale = useTransform(scrollYProgress, [0, 0.3], [1, 0.9]);

  const features = [
    {
      icon: Stamp,
      title: 'Digital Stamps',
      desc: 'Collect stamps digitally — no more lost paper cards. Every visit counts.',
      color: 'from-indigo-500 to-purple-500',
    },
    {
      icon: Gift,
      title: 'Custom Rewards',
      desc: 'Shop owners set their own rewards. Free coffee, discounts, exclusive perks.',
      color: 'from-amber-500 to-orange-500',
    },
    {
      icon: Shield,
      title: 'Secure & Simple',
      desc: 'Easy sign-in, tamper-proof tracking. Your loyalty, your data.',
      color: 'from-emerald-500 to-teal-500',
    },
    {
      icon: Smartphone,
      title: 'Works Everywhere',
      desc: 'Optimized for every device — phone, tablet, or desktop. Always accessible.',
      color: 'from-rose-500 to-pink-500',
    },
  ];

  const stats = [
    { icon: Users, value: '10K+', label: 'Happy Users' },
    { icon: Store, value: '500+', label: 'Partner Shops' },
    { icon: Stamp, value: '1M+', label: 'Stamps Collected' },
    { icon: Gift, value: '50K+', label: 'Rewards Redeemed' },
  ];

  return (
    <div ref={containerRef} className="min-h-screen">
      {/* ── Hero Section with Parallax ── */}
      <section className="relative min-h-screen flex items-center justify-center overflow-hidden">
        {/* Background layer - slow */}
        <motion.div
          style={{ y: bgY }}
          className="absolute inset-0 pointer-events-none"
        >
          <div className="absolute inset-0 bg-linear-to-b from-indigo-950/60 via-transparent to-transparent" />
          {/* Floating orbs - enhanced */}
          <div className="absolute top-20 left-[10%] w-72 h-72 bg-primary/30 rounded-full blur-3xl animate-pulse" />
          <div className="absolute top-40 right-[15%] w-96 h-96 bg-accent/15 rounded-full blur-3xl animate-pulse [animation-delay:1s]" />
          <div className="absolute bottom-20 left-[30%] w-64 h-64 bg-purple-500/20 rounded-full blur-3xl animate-pulse [animation-delay:2s]" />
        </motion.div>

        {/* Mid layer - medium speed */}
        <motion.div
          style={{ y: midY }}
          className="absolute inset-0 pointer-events-none"
        >
          {/* Decorative stamps */}
          {[...Array(6)].map((_, i) => (
            <motion.div
              key={i}
              className="absolute w-8 h-8 sm:w-12 sm:h-12 rounded-xl bg-white/5 border border-white/10"
              style={{
                top: `${15 + i * 14}%`,
                left: `${5 + (i % 3) * 35 + (i > 2 ? 10 : 0)}%`,
              }}
              animate={{
                rotate: [0, 360],
                y: [0, -20, 0],
              }}
              transition={{
                duration: 6 + i * 2,
                repeat: Infinity,
                ease: 'linear',
              }}
            />
          ))}
        </motion.div>

        {/* Content layer - foreground */}
        <motion.div style={{ opacity, scale }} className="relative z-10 max-w-5xl mx-auto px-4 text-center pt-24">
          <motion.div
            initial={{ opacity: 0, y: 30 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8 }}
          >
            <div className="inline-flex items-center gap-2 bg-white/5 border border-white/10 rounded-full px-4 py-1.5 mb-6">
              <Sparkles className="w-4 h-4 text-accent" />
              <span className="text-sm text-indigo-200">Your Digital Loyalty Companion</span>
            </div>
          </motion.div>

          <motion.h1
            initial={{ opacity: 0, y: 30 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, delay: 0.1 }}
            className="text-5xl sm:text-6xl md:text-7xl lg:text-8xl font-black text-white leading-tight tracking-tight mb-6"
          >
            Collect Stamps.
            <br />
            <span className="text-transparent bg-clip-text bg-linear-to-r from-accent via-amber-300 to-orange-400">
              Earn Rewards.
            </span>
          </motion.h1>

          <motion.p
            initial={{ opacity: 0, y: 30 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, delay: 0.2 }}
            className="text-lg sm:text-xl text-indigo-200 max-w-2xl mx-auto mb-10"
          >
            The modern digital stamp card for your favorite shops. No paper, no hassle — just pure
            loyalty rewards at your fingertips.
          </motion.p>

          <motion.div
            initial={{ opacity: 0, y: 30 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, delay: 0.3 }}
            className="flex flex-col sm:flex-row items-center justify-center gap-4"
          >
            <Link
              to="/login"
              className="group flex items-center gap-2 bg-linear-to-r from-accent to-amber-400 text-surface font-bold px-8 py-4 rounded-2xl hover:shadow-lg hover:shadow-accent/25 transition-all hover:scale-105 text-lg"
            >
              Get Started
              <ArrowRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" />
            </Link>
            <button
              onClick={() => document.getElementById('features')?.scrollIntoView({ behavior: 'smooth' })}
              className="flex items-center gap-2 text-indigo-200 hover:text-white font-medium px-6 py-4 rounded-2xl border border-white/10 hover:bg-white/5 transition-all cursor-pointer"
            >
              Learn More
              <ChevronDown className="w-5 h-5" />
            </button>
          </motion.div>

          {/* Hero stamp card preview */}
          <motion.div
            initial={{ opacity: 0, y: 60 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 1, delay: 0.5 }}
            className="mt-16 max-w-md mx-auto"
          >
            <div className="bg-linear-to-br from-white/10 to-white/5 backdrop-blur-xl border border-white/15 rounded-3xl p-6 shadow-2xl">
              <div className="flex items-center gap-3 mb-4">
                <div className="w-10 h-10 bg-linear-to-br from-amber-400 to-orange-500 rounded-xl flex items-center justify-center">
                  <Star className="w-5 h-5 text-white" />
                </div>
                <div>
                  <h3 className="text-white font-bold text-sm">Café Sonnenschein</h3>
                  <p className="text-xs text-indigo-300">5 / 8 stamps</p>
                </div>
              </div>
              <div className="grid grid-cols-8 gap-2">
                {Array.from({ length: 8 }).map((_, i) => (
                  <motion.div
                    key={i}
                    initial={{ scale: 0 }}
                    animate={{ scale: 1 }}
                    transition={{ delay: 0.8 + i * 0.1, type: 'spring', stiffness: 200 }}
                    className={`aspect-square rounded-lg flex items-center justify-center text-xs font-bold ${
                      i < 5
                        ? 'bg-linear-to-br from-amber-400 to-orange-500 text-white'
                        : 'border border-dashed border-white/20 text-white/20'
                    }`}
                  >
                    {i < 5 ? '✓' : i + 1}
                  </motion.div>
                ))}
              </div>
            </div>
          </motion.div>
        </motion.div>

        {/* Scroll indicator */}
        <motion.div
          animate={{ y: [0, 8, 0] }}
          transition={{ duration: 2, repeat: Infinity }}
          className="absolute bottom-8 left-1/2 -translate-x-1/2"
        >
          <ChevronDown className="w-6 h-6 text-indigo-400" />
        </motion.div>
      </section>

      <hr className="glow-divider" />

      {/* ── Stats ── */}
      <section className="py-16 relative">
        <div className="max-w-6xl mx-auto px-4">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-6">
            {stats.map((stat, i) => (
              <motion.div
                key={i}
                initial={{ opacity: 0, y: 30 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ delay: i * 0.1 }}
                className="text-center"
              >
                <div className="w-12 h-12 bg-white/5 rounded-2xl flex items-center justify-center mx-auto mb-3 border border-white/10">
                  <stat.icon className="w-6 h-6 text-accent" />
                </div>
                <div className="text-3xl sm:text-4xl font-black text-white mb-1">{stat.value}</div>
                <div className="text-sm text-indigo-300">{stat.label}</div>
              </motion.div>
            ))}
          </div>
        </div>
      </section>

      <hr className="glow-divider" />

      {/* ── Features ── */}
      <section id="features" className="py-20 sm:py-28 relative">
        <div className="max-w-6xl mx-auto px-4">
          <motion.div
            initial={{ opacity: 0, y: 30 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            className="text-center mb-16"
          >
            <h2 className="text-3xl sm:text-5xl font-black text-white mb-4">
              Why <span className="text-accent">Länd of Stamp</span>?
            </h2>
            <p className="text-indigo-300 text-lg max-w-2xl mx-auto">
              Everything you need for a seamless loyalty experience — for customers and shop owners
              alike.
            </p>
          </motion.div>

          <div className="grid sm:grid-cols-2 gap-6">
            {features.map((f, i) => (
              <motion.div
                key={i}
                initial={{ opacity: 0, y: 30 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ delay: i * 0.1 }}
                className="group relative bg-linear-to-br from-white/[0.07] to-white/2 backdrop-blur-sm border border-white/10 rounded-3xl p-6 sm:p-8 hover:border-white/20 transition-all"
              >
                <div
                  className={`w-14 h-14 bg-linear-to-br ${f.color} rounded-2xl flex items-center justify-center mb-5 group-hover:scale-110 transition-transform`}
                >
                  <f.icon className="w-7 h-7 text-white" />
                </div>
                <h3 className="text-xl font-bold text-white mb-2">{f.title}</h3>
                <p className="text-indigo-300 leading-relaxed">{f.desc}</p>
              </motion.div>
            ))}
          </div>
        </div>
      </section>

      <hr className="glow-divider" />

      {/* ── How it Works ── */}
      <section className="py-20 sm:py-28 relative">
        <div className="max-w-5xl mx-auto px-4">
          <motion.div
            initial={{ opacity: 0, y: 30 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            className="text-center mb-16"
          >
            <h2 className="text-3xl sm:text-5xl font-black text-white mb-4">
              How it <span className="text-accent">Works</span>
            </h2>
          </motion.div>

          <div className="grid sm:grid-cols-3 gap-8">
            {[
              { title: 'Visit a Shop', desc: 'Go to any participating shop and make a purchase.', emoji: '🏪' },
              {  title: 'Get Stamped', desc: 'The shop owner grants you a digital stamp instantly.', emoji: '✅' },
              { title: 'Earn Rewards', desc: 'Collect all stamps and redeem your exclusive reward!', emoji: '🎁' },
            ].map((item, i) => (
              <motion.div
                key={i}
                initial={{ opacity: 0, y: 30 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ delay: i * 0.15 }}
                className="text-center"
              >
                <div className="text-5xl mb-4">{item.emoji}</div>
                <h3 className="text-xl font-bold text-white mb-2">{item.title}</h3>
                <p className="text-indigo-300">{item.desc}</p>
              </motion.div>
            ))}
          </div>
        </div>
      </section>

      <hr className="glow-divider" />

      {/* ── CTA ── */}
      <section className="py-20 sm:py-28 relative">
        <div className="max-w-4xl mx-auto px-4">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            whileInView={{ opacity: 1, scale: 1 }}
            viewport={{ once: true }}
            className="relative bg-linear-to-br from-primary/30 to-accent/10 border border-white/10 rounded-3xl p-8 sm:p-12 text-center overflow-hidden"
          >
            <div className="absolute top-0 right-0 w-64 h-64 bg-accent/10 rounded-full blur-3xl -translate-y-1/2 translate-x-1/2" />
            <h2 className="relative text-3xl sm:text-4xl font-black text-white mb-4">
              Ready to start collecting?
            </h2>
            <p className="relative text-indigo-200 text-lg mb-8 max-w-xl mx-auto">
              Join thousands of happy customers and shop owners. Your next reward is just a few stamps
              away.
            </p>
            <Link
              to="/login"
              className="relative inline-flex items-center gap-2 bg-linear-to-r from-accent to-amber-400 text-surface font-bold px-8 py-4 rounded-2xl hover:shadow-lg hover:shadow-accent/25 transition-all hover:scale-105 text-lg"
            >
              Sign Up Now
              <ArrowRight className="w-5 h-5" />
            </Link>
          </motion.div>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="border-t border-white/10 py-8">
        <div className="max-w-6xl mx-auto px-4 flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 bg-linear-to-br from-accent to-amber-400 rounded-lg flex items-center justify-center">
              <Stamp className="w-4 h-4 text-surface" />
            </div>
            <span className="text-white font-bold">
              Länd of <span className="text-accent">Stamp</span>
            </span>
          </div>
          <p className="text-indigo-400 text-sm">© 2026 Länd of Stamp. All rights reserved.</p>
        </div>
      </footer>
    </div>
  );
}

