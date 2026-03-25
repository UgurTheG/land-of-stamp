const API_BASE = import.meta.env.VITE_API_URL ?? '';

// ── Types (generated from proto) ────
export type { User, Shop, StampCard, StampToken, ClaimStampResponse } from '../gen/proto/stempelkarte';
import type { User, Shop, StampCard, StampToken, ClaimStampResponse, AuthResponse } from '../gen/proto/stempelkarte';

export interface StampTokenStatus {
  active: boolean;
  expiresAt?: string;
}

// ── Session storage (user info only — JWT is in HttpOnly cookie) ──

export function clearSession(): void {
  localStorage.removeItem('land_of_stamp_current_user');
}

function saveUser(user: User): void {
  localStorage.setItem('land_of_stamp_current_user', JSON.stringify(user));
}

// ── HTTP helpers ───────────────────────────────────────

/** Paths that are allowed to return 401 without triggering auto-logout. */
const AUTH_PATHS = ['/api/auth/login', '/api/auth/register', '/api/auth/logout'];

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) ?? {}),
  };

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
    credentials: 'include', // send HttpOnly cookie automatically
  });

  if (!res.ok) {
    // If we get a 401 on a non-auth endpoint, the session is stale
    // (e.g. backend restarted with a new JWT secret). Auto-logout.
    if (res.status === 401 && !AUTH_PATHS.includes(path)) {
      clearSession();
      // Only redirect if we're in a browser context (not SSR)
      if (typeof window !== 'undefined') {
        window.location.href = '/login';
      }
      throw new Error('Session expired — please log in again');
    }

    const body = await res.json().catch(() => ({}));
    throw new Error(body.error ?? `Request failed: ${res.status}`);
  }

  return res.json();
}

// ── Auth (credentials via Authorization: Basic header — never in the body) ──

function basicHeader(username: string, password: string): string {
  return 'Basic ' + btoa(`${username}:${password}`);
}

export async function apiRegister(username: string, password: string, role: 'user' | 'admin'): Promise<User> {
  const data = await request<AuthResponse>('/api/auth/register', {
    method: 'POST',
    headers: { Authorization: basicHeader(username, password) },
    body: JSON.stringify({ role }),
  });
  const user = data.user!;
  saveUser(user);
  return user;
}

export async function apiLogin(username: string, password: string): Promise<User> {
  const data = await request<AuthResponse>('/api/auth/login', {
    method: 'POST',
    headers: { Authorization: basicHeader(username, password) },
  });
  const user = data.user!;
  saveUser(user);
  return user;
}

export async function apiLogout(): Promise<void> {
  await request('/api/auth/logout', { method: 'POST' });
  clearSession();
}

export async function apiGetMe(): Promise<User> {
  return request<User>('/api/auth/me');
}

// ── Shops ──────────────────────────────────────────────

export async function apiGetShops(): Promise<Shop[]> {
  return request<Shop[]>('/api/shops');
}

export async function apiGetMyShops(): Promise<Shop[]> {
  return request<Shop[]>('/api/shops/mine');
}

export async function apiCreateShop(data: Omit<Shop, 'id' | 'ownerId'>): Promise<Shop> {
  return request<Shop>('/api/shops', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function apiUpdateShop(id: string, data: Omit<Shop, 'id' | 'ownerId'>): Promise<Shop> {
  return request<Shop>(`/api/shops/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

// ── Cards & Stamps ─────────────────────────────────────

export async function apiGetMyCards(): Promise<StampCard[]> {
  return request<StampCard[]>('/api/users/me/cards');
}

export async function apiGetShopCards(shopId: string): Promise<StampCard[]> {
  return request<StampCard[]>(`/api/shops/${shopId}/cards`);
}

export async function apiGrantStamp(shopId: string, userId: string): Promise<StampCard> {
  return request<StampCard>(`/api/shops/${shopId}/stamps`, {
    method: 'POST',
    body: JSON.stringify({ userId }),
  });
}

export async function apiUpdateStampCount(shopId: string, userId: string, stamps: number): Promise<StampCard> {
  return request<StampCard>(`/api/shops/${shopId}/stamps`, {
    method: 'PATCH',
    body: JSON.stringify({ userId, stamps }),
  });
}

export async function apiRedeemCard(cardId: string): Promise<void> {
  await request(`/api/cards/${cardId}/redeem`, { method: 'POST' });
}

// ── Customers (admin) ──────────────────────────────────

export async function apiGetCustomers(): Promise<User[]> {
  return request<User[]>('/api/users/customers');
}

// ── QR Code Stamps ─────────────────────────────────────

export async function apiCreateStampToken(shopId: string): Promise<StampToken> {
  return request<StampToken>(`/api/shops/${shopId}/stamp-token`, {
    method: 'POST',
  });
}

export async function apiGetStampTokenStatus(shopId: string): Promise<StampTokenStatus> {
  return request<StampTokenStatus>(`/api/shops/${shopId}/stamp-token/status`);
}

export async function apiClaimStamp(token: string): Promise<ClaimStampResponse> {
  return request<ClaimStampResponse>('/api/stamps/claim', {
    method: 'POST',
    body: JSON.stringify({ token }),
  });
}

