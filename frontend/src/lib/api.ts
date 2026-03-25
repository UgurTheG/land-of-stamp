import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { create } from '@bufbuild/protobuf';

import { AuthService, ShopService, StampService } from '../gen/proto/stempelkarte_pb';
import {
  RegisterRequestSchema,
  LoginRequestSchema,
  LogoutRequestSchema,
  GetMeRequestSchema,
  ListShopsRequestSchema,
  CreateShopRequestSchema,
  UpdateShopRequestSchema,
  GetMyShopsRequestSchema,
  GetMyCardsRequestSchema,
  JoinShopRequestSchema,
  GetShopCardsRequestSchema,
  GrantStampRequestSchema,
  UpdateStampCountRequestSchema,
  RedeemCardRequestSchema,
  GetShopCustomersRequestSchema,
  CreateStampTokenRequestSchema,
  GetStampTokenStatusRequestSchema,
  ClaimStampRequestSchema,
} from '../gen/proto/stempelkarte_pb';

// Re-export entity types so existing imports keep working.
export type { User, Shop, StampCard, StampToken, ClaimStampResponse } from '../gen/proto/stempelkarte_pb';
import type { User } from '../gen/proto/stempelkarte_pb';

export interface StampTokenStatus {
  active: boolean;
  expiresAt?: string;
}

// ── Transport ──────────────────────────────────────────

const API_BASE = import.meta.env.VITE_API_URL ?? '';

const transport = createConnectTransport({
  baseUrl: API_BASE,
  // Send cookies (HttpOnly JWT) with every request.
  fetch: (input, init) => fetch(input, { ...init, credentials: 'include' }),
});

const authClient  = createClient(AuthService, transport);
const shopClient  = createClient(ShopService, transport);
const stampClient = createClient(StampService, transport);

// ── Session storage (user info only — JWT is in HttpOnly cookie) ──

export function clearSession(): void {
  localStorage.removeItem('land_of_stamp_current_user');
}

function saveUser(user: User): void {
  localStorage.setItem('land_of_stamp_current_user', JSON.stringify(user));
}

/** Persist user to localStorage (used after OAuth callback). */
export const persistSession = saveUser;

// ── Auth ───────────────────────────────────────────────

export async function apiRegister(username: string, password: string, role: 'user' | 'admin'): Promise<User> {
  const res = await authClient.register(create(RegisterRequestSchema, { username, password, role }));
  const user = res.user!;
  saveUser(user);
  return user;
}

export async function apiLogin(username: string, password: string): Promise<User> {
  const res = await authClient.login(create(LoginRequestSchema, { username, password }));
  const user = res.user!;
  saveUser(user);
  return user;
}

export async function apiLogout(): Promise<void> {
  await authClient.logout(create(LogoutRequestSchema, {}));
  clearSession();
}

export async function apiGetMe(): Promise<User> {
  return authClient.getMe(create(GetMeRequestSchema, {}));
}

// ── Shops ──────────────────────────────────────────────

export async function apiGetShops() {
  const res = await shopClient.listShops(create(ListShopsRequestSchema, {}));
  return res.shops;
}

export async function apiGetMyShops() {
  const res = await shopClient.getMyShops(create(GetMyShopsRequestSchema, {}));
  return res.shops;
}

export async function apiCreateShop(data: { name: string; description: string; rewardDescription: string; stampsRequired: number; color: string }) {
  return shopClient.createShop(create(CreateShopRequestSchema, data));
}

export async function apiUpdateShop(id: string, data: { name: string; description: string; rewardDescription: string; stampsRequired: number; color: string }) {
  return shopClient.updateShop(create(UpdateShopRequestSchema, { id, ...data }));
}

// ── Cards & Stamps ─────────────────────────────────────

export async function apiGetMyCards() {
  const res = await stampClient.getMyCards(create(GetMyCardsRequestSchema, {}));
  return res.cards;
}

export async function apiGetShopCards(shopId: string) {
  const res = await stampClient.getShopCards(create(GetShopCardsRequestSchema, { shopId }));
  return res.cards;
}

export async function apiGrantStamp(shopId: string, userId: string) {
  return stampClient.grantStamp(create(GrantStampRequestSchema, { shopId, userId }));
}

export async function apiUpdateStampCount(shopId: string, userId: string, stamps: number) {
  return stampClient.updateStampCount(create(UpdateStampCountRequestSchema, { shopId, userId, stamps }));
}

export async function apiRedeemCard(cardId: string): Promise<void> {
  await stampClient.redeemCard(create(RedeemCardRequestSchema, { cardId }));
}

// ── Customers (admin — per shop) ───────────────────────

export async function apiGetShopCustomers(shopId: string) {
  const res = await stampClient.getShopCustomers(create(GetShopCustomersRequestSchema, { shopId }));
  return res.users;
}

// ── Join Shop (user) ───────────────────────────────────

export async function apiJoinShop(shopId: string) {
  return stampClient.joinShop(create(JoinShopRequestSchema, { shopId }));
}

// ── QR Code Stamps ─────────────────────────────────────

export async function apiCreateStampToken(shopId: string) {
  return stampClient.createStampToken(create(CreateStampTokenRequestSchema, { shopId }));
}

export async function apiGetStampTokenStatus(shopId: string): Promise<StampTokenStatus> {
  const res = await stampClient.getStampTokenStatus(create(GetStampTokenStatusRequestSchema, { shopId }));
  return { active: res.active, expiresAt: res.expiresAt || undefined };
}

export async function apiClaimStamp(token: string) {
  return stampClient.claimStamp(create(ClaimStampRequestSchema, { token }));
}
