import { useMemo, useState, useEffect } from 'react';
import { useNavigate } from 'react-router';
import { toast } from 'sonner';
import { Camera, Trash2, UserCircle2 } from 'lucide-react';
import { useAuth } from '../hooks/useAuth';
import { useLocale } from '../hooks/useLocale';
import {
  apiDeleteProfilePicture,
  apiGetProfileStats,
  apiUpdateProfile,
  apiUploadProfilePicture,
  persistSession,
  type ProfileStatsResponse,
} from '../lib/api';

async function fileToBase64(file: File): Promise<string> {
  const arr = new Uint8Array(await file.arrayBuffer());
  let binary = '';
  for (const b of arr) binary += String.fromCharCode(b);
  return btoa(binary);
}

export default function ProfilePage() {
  const { user, refreshUser, deleteAccount } = useAuth();
  const { m } = useLocale();
  const navigate = useNavigate();

  const [displayName, setDisplayName] = useState(user?.displayName || user?.username || '');
  const [savingProfile, setSavingProfile] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [stats, setStats] = useState<ProfileStatsResponse | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    if (!user) return;
    apiGetProfileStats().then(setStats).catch(() => {
      toast.error(m.profile.toasts.loadStatsFailed);
    });
  }, [user, m.profile.toasts.loadStatsFailed]);

  const initials = useMemo(() => {
    const source = (user?.displayName || user?.username || 'U').trim();
    return source.slice(0, 2).toUpperCase();
  }, [user?.displayName, user?.username]);

  if (!user) return null;

  const saveProfile = async () => {
    setSavingProfile(true);
    try {
      const updated = await apiUpdateProfile(displayName.trim());
      persistSession(updated);
      refreshUser(updated);
      toast.success(m.profile.toasts.profileSaved);
    } catch {
      toast.error(m.profile.toasts.profileSaveFailed);
    } finally {
      setSavingProfile(false);
    }
  };

  const onAvatarSelected = async (file: File | null) => {
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      toast.error(m.profile.toasts.invalidAvatar);
      return;
    }
    setUploading(true);
    try {
      const b64 = await fileToBase64(file);
      const updated = await apiUploadProfilePicture(file.type, b64);
      persistSession(updated);
      refreshUser(updated);
      toast.success(m.profile.toasts.avatarUploaded);
    } catch {
      toast.error(m.profile.toasts.avatarUploadFailed);
    } finally {
      setUploading(false);
    }
  };

  const removeAvatar = async () => {
    setUploading(true);
    try {
      const updated = await apiDeleteProfilePicture();
      persistSession(updated);
      refreshUser(updated);
      toast.success(m.profile.toasts.avatarRemoved);
    } catch {
      toast.error(m.profile.toasts.avatarRemoveFailed);
    } finally {
      setUploading(false);
    }
  };

  const handleDeleteAccount = async () => {
    setDeleting(true);
    try {
      await deleteAccount();
      toast.success(m.profile.toasts.accountDeleted);
      navigate('/login', { replace: true });
    } catch {
      toast.error(m.profile.toasts.accountDeleteFailed);
      setDeleting(false);
      setConfirmDelete(false);
    }
  };

  return (
    <div className="min-h-screen pt-20 pb-12">
      <div className="max-w-4xl mx-auto px-4 space-y-8">
        <div className="bg-white/5 border border-white/10 rounded-2xl p-6">
          <h1 className="text-3xl font-bold text-white">{m.profile.title}</h1>
          <p className="text-indigo-300 mt-1">{m.profile.subtitle}</p>
        </div>

        <div className="bg-white/5 border border-white/10 rounded-2xl p-6 space-y-6">
          <h2 className="text-xl font-semibold text-white">{m.profile.accountSection}</h2>

          <div className="flex flex-col sm:flex-row gap-6 items-start">
            <div className="relative">
              {user.avatarUrl ? (
                <img src={user.avatarUrl} alt={m.profile.avatarAlt} className="w-24 h-24 rounded-full object-cover border border-white/20" />
              ) : (
                <div className="w-24 h-24 rounded-full bg-indigo-500/30 border border-white/20 flex items-center justify-center text-2xl font-bold text-white">
                  {initials}
                </div>
              )}
              <label className="absolute -bottom-1 -right-1 bg-primary hover:bg-primary-dark rounded-full p-2 cursor-pointer">
                <Camera className="w-4 h-4 text-white" />
                <input
                  type="file"
                  accept="image/*"
                  className="hidden"
                  onChange={(e) => onAvatarSelected(e.target.files?.[0] ?? null)}
                  disabled={uploading}
                />
              </label>
            </div>

            <div className="flex-1 space-y-2">
              <div className="text-sm text-indigo-300">{m.profile.emailLike}</div>
              <div className="text-white font-medium">@{user.username}</div>
              <div className="text-sm text-indigo-300">{m.profile.role}: <span className="text-white">{user.role}</span></div>
              {user.avatarUrl && (
                <button className="text-sm text-rose-300 hover:text-rose-200 cursor-pointer" onClick={removeAvatar} disabled={uploading}>
                  {m.profile.removeAvatar}
                </button>
              )}
            </div>
          </div>

          <div className="space-y-2">
            <label className="text-sm text-indigo-300">{m.profile.displayName}</label>
            <input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              maxLength={64}
              className="w-full bg-white/5 border border-white/10 rounded-xl px-4 py-3 text-white"
              placeholder={m.profile.displayNamePlaceholder}
            />
          </div>

          <button
            className="bg-linear-to-r from-primary to-primary-dark text-white font-semibold px-5 py-2.5 rounded-xl cursor-pointer disabled:opacity-50"
            onClick={saveProfile}
            disabled={savingProfile}
          >
            {savingProfile ? m.profile.saving : m.profile.saveProfile}
          </button>
        </div>

        <div className="bg-white/5 border border-white/10 rounded-2xl p-6">
          <h2 className="text-xl font-semibold text-white mb-4">{m.profile.statsTitle}</h2>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            <div className="bg-white/5 rounded-xl p-4"><p className="text-2xl text-white font-bold">{stats?.joinedShops ?? 0}</p><p className="text-xs text-indigo-300">{m.profile.stats.joinedShops}</p></div>
            <div className="bg-white/5 rounded-xl p-4"><p className="text-2xl text-white font-bold">{stats?.activeCards ?? 0}</p><p className="text-xs text-indigo-300">{m.profile.stats.activeCards}</p></div>
            <div className="bg-white/5 rounded-xl p-4"><p className="text-2xl text-white font-bold">{stats?.redeemedCards ?? 0}</p><p className="text-xs text-indigo-300">{m.profile.stats.redeemedCards}</p></div>
            <div className="bg-white/5 rounded-xl p-4"><p className="text-2xl text-white font-bold">{stats?.totalStamps ?? 0}</p><p className="text-xs text-indigo-300">{m.profile.stats.totalStamps}</p></div>
          </div>
        </div>

        <div className="border border-rose-500/20 rounded-2xl p-6 bg-rose-500/5">
          <h2 className="text-xl font-semibold text-rose-300">{m.profile.dangerTitle}</h2>
          <p className="text-sm text-indigo-300 mt-1">{m.profile.dangerBody}</p>
          {!confirmDelete ? (
            <button
              className="mt-4 inline-flex items-center gap-2 bg-rose-600 hover:bg-rose-500 text-white font-semibold px-4 py-2 rounded-xl cursor-pointer"
              onClick={() => setConfirmDelete(true)}
            >
              <Trash2 className="w-4 h-4" />
              {m.profile.deleteAccount}
            </button>
          ) : (
            <div className="mt-4 flex flex-wrap gap-3">
              <button
                className="inline-flex items-center gap-2 bg-rose-600 hover:bg-rose-500 text-white font-semibold px-4 py-2 rounded-xl cursor-pointer disabled:opacity-60"
                onClick={handleDeleteAccount}
                disabled={deleting}
              >
                <UserCircle2 className="w-4 h-4" />
                {deleting ? m.profile.deleting : m.profile.confirmDelete}
              </button>
              <button className="border border-white/20 text-indigo-200 px-4 py-2 rounded-xl cursor-pointer" onClick={() => setConfirmDelete(false)} disabled={deleting}>
                {m.common.cancel}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

