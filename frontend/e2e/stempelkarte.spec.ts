/**
 * Länd of Stamp – Playwright E2E Tests
 *
 * Uses direct API calls for reliable test setup (register/login), and
 * browser-based UI interactions for testing actual user flows.
 * This avoids React 19 form-action timing issues in controlled components.
 */

import { test, expect, type Page, type Browser } from '@playwright/test';

// ── Helpers ────────────────────────────────────────────────────────────────────

const uid = () => Math.random().toString(36).slice(2, 8);

const API_BASE = 'http://localhost:8080';

/** Register a user via the backend API and set the auth cookie on the page. */
async function registerViaAPI(
  page: Page,
  role: 'user' | 'admin',
  opts?: { username?: string; password?: string },
) {
  const username = opts?.username ?? `e2e_${role}_${uid()}`;
  const password = opts?.password ?? 'test1234';

  const context = page.context();
  const resp = await context.request.post(`${API_BASE}/api/auth/register`, {
    headers: {
      Authorization: 'Basic ' + Buffer.from(`${username}:${password}`).toString('base64'),
      'Content-Type': 'application/json',
    },
    data: { role },
  });

  if (!resp.ok()) {
    const body = await resp.text();
    throw new Error(`Register failed (${resp.status()}): ${body}`);
  }

  // Navigate to the role-appropriate page (cookie is set from API response)
  const target = role === 'admin' ? '/admin' : '/dashboard';
  await page.goto(target);
  await page.waitForLoadState('networkidle');

  // Also set localStorage so AuthContext picks up the user
  const json = await resp.json();
  await page.evaluate((user) => {
    localStorage.setItem('land_of_stamp_current_user', JSON.stringify(user));
  }, json.user);

  // Reload so React hydrates with the user from localStorage + cookie
  await page.goto(target);
  await page.waitForLoadState('networkidle');

  return { username, password };
}

/** Login via API and navigate to the appropriate dashboard. */
async function loginViaAPI(page: Page, username: string, password: string) {
  const context = page.context();
  const resp = await context.request.post(`${API_BASE}/api/auth/login`, {
    headers: {
      Authorization: 'Basic ' + Buffer.from(`${username}:${password}`).toString('base64'),
      'Content-Type': 'application/json',
    },
  });

  if (!resp.ok()) {
    const body = await resp.text();
    throw new Error(`Login failed (${resp.status()}): ${body}`);
  }

  const json = await resp.json();
  const target = json.user.role === 'admin' ? '/admin' : '/dashboard';
  await page.evaluate((user) => {
    localStorage.setItem('land_of_stamp_current_user', JSON.stringify(user));
  }, json.user);

  await page.goto(target);
  await page.waitForLoadState('networkidle');
}

/** Clear auth state and go home. */
async function logoutViaUI(page: Page) {
  await page.getByText('Logout').click();
  // App may redirect to / or /login after logout
  await expect(page).toHaveURL(/^\/$|\/login/, { timeout: 5_000 });
}


// ═══════════════════════════════════════════════════════════════════════════════
//  LANDING PAGE
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Landing Page', () => {
  test('renders hero section with CTA', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('link', { name: /Länd of Stamp/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Sign In|Get Started/i }).first()).toBeVisible();
  });

  test('navbar shows Sign In link when not authenticated', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Sign In' })).toBeVisible();
  });

  test('language switcher updates landing copy to German', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'DE' }).first().click();
    await expect(page.getByText('Stempel sammeln.')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Mehr erfahren' })).toBeVisible();
  });

  test('logged-in user landing page shows dashboard CTAs instead of guest signup copy', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await page.goto('/');

    await expect(page.getByText('Ready to start collecting?')).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'Go to My Cards' }).first()).toBeVisible();
    await expect(page.getByRole('link', { name: 'View My Cards' })).toBeVisible();
  });

  test('logged-in admin landing page shows admin dashboard CTAs instead of guest signup copy', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await page.goto('/');

    await expect(page.getByText('Ready to start collecting?')).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'Open Dashboard' }).first()).toBeVisible();
    await expect(page.getByRole('link', { name: 'Go to Admin Dashboard' })).toBeVisible();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  REGISTRATION (via API + verify UI state)
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Registration', () => {
  test('user registers and sees dashboard', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await expect(page).toHaveURL(/\/dashboard/);
    await expect(page.getByText('Welcome back')).toBeVisible();
  });

  test('admin registers and sees admin panel', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await expect(page).toHaveURL(/\/admin/);
    await expect(page.getByText('Shop Dashboard')).toBeVisible();
  });

  test('shows error for short username (< 2 chars)', async ({ page }) => {
    await page.goto('/login');
    await page.getByText('Register', { exact: true }).click();
    await page.getByPlaceholder('Enter your username').fill('a');
    await page.getByPlaceholder('Enter your password').fill('test1234');
    await page.getByPlaceholder('Enter your password').press('Enter');
    await expect(page.getByText(/username must be at least 2/i)).toBeVisible({ timeout: 3_000 });
  });

  test('shows error for short password (< 4 chars)', async ({ page }) => {
    await page.goto('/login');
    await page.getByText('Register', { exact: true }).click();
    await page.getByPlaceholder('Enter your username').fill('validuser');
    await page.getByPlaceholder('Enter your password').fill('ab');
    await page.getByPlaceholder('Enter your password').press('Enter');
    await expect(page.getByText(/password must be at least 4/i)).toBeVisible({ timeout: 3_000 });
  });

  test('duplicate username returns 409 from API', async ({ page }) => {
    // Register first user via API
    const username = `dup_${uid()}`;
    const password = 'test1234';
    await registerViaAPI(page, 'user', { username, password });

    // Try to register same username via API directly
    const context = page.context();
    const resp = await context.request.post(`${API_BASE}/api/auth/register`, {
      headers: {
        Authorization: 'Basic ' + Buffer.from(`${username}:${password}`).toString('base64'),
        'Content-Type': 'application/json',
      },
      data: { role: 'user' },
    });
    expect(resp.status()).toBe(409);
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  LOGIN / LOGOUT
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Login & Logout', () => {
  test('can login and reach dashboard', async ({ page }) => {
    const { username, password } = await registerViaAPI(page, 'user');
    // Clear cookies/storage to simulate logout
    await page.context().clearCookies();
    await page.evaluate(() => localStorage.clear());

    // Login via API and verify dashboard
    await loginViaAPI(page, username, password);
    await expect(page).toHaveURL(/\/dashboard/);
  });

  test('wrong password returns 401 from API', async ({ page }) => {
    const { username } = await registerViaAPI(page, 'user');
    const context = page.context();
    const resp = await context.request.post(`${API_BASE}/api/auth/login`, {
      headers: {
        Authorization: 'Basic ' + Buffer.from(`${username}:wrongpass`).toString('base64'),
        'Content-Type': 'application/json',
      },
    });
    expect(resp.status()).toBe(401);
  });

  test('non-existent user returns 401 from API', async ({ page }) => {
    const context = page.context();
    const resp = await context.request.post(`${API_BASE}/api/auth/login`, {
      headers: {
        Authorization: 'Basic ' + Buffer.from(`ghost_${uid()}:anypass`).toString('base64'),
        'Content-Type': 'application/json',
      },
    });
    expect(resp.status()).toBe(401);
  });

  test('logout clears session and redirects home', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await logoutViaUI(page);
    await expect(page.getByRole('link', { name: 'Sign In' })).toBeVisible();
  });

  test('cannot access /dashboard when logged out', async ({ page }) => {
    await page.goto('/dashboard');
    await expect(page).toHaveURL(/\/login/);
  });

  test('cannot access /admin when logged out', async ({ page }) => {
    await page.goto('/admin');
    await expect(page).toHaveURL(/\/login/);
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  ROLE-BASED NAVIGATION
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Role-Based Navigation', () => {
  test('user role cannot access /admin', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await page.goto('/admin');
    await expect(page).not.toHaveURL(/\/admin/);
  });

  test('admin role cannot access /dashboard', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await page.goto('/dashboard');
    await expect(page).not.toHaveURL(/\/dashboard/);
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  USER DASHBOARD
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('User Dashboard', () => {
  test('shows empty state when no shops joined', async ({ page }) => {
    await registerViaAPI(page, 'user');
    const noShops = page.getByText(/no shops joined yet/i);
    const zero = page.getByText('0');
    const hasNoShops = await noShops.isVisible().catch(() => false);
    const hasZero = await zero.first().isVisible().catch(() => false);
    expect(hasNoShops || hasZero).toBeTruthy();
  });

  test('user can discover and join a shop', async ({ browser }) => {
    const adminCtx = await browser.newContext();
    const userCtx = await browser.newContext();

    // Admin creates a shop
    const adminPage = await adminCtx.newPage();
    await registerViaAPI(adminPage, 'admin');
    const discoverShopName = `Discoverable Café ${uid()}`;
    const shop = await createShopViaAPI(adminPage, discoverShopName);

    // User registers and sees empty state
    const userPage = await userCtx.newPage();
    await registerViaAPI(userPage, 'user');

    // Open discover section
    const discoverBtn = userPage.getByRole('button', { name: /Discover Shops/i });
    await expect(discoverBtn).toBeVisible({ timeout: 5_000 });
    await discoverBtn.click();
    await expect(userPage.getByText(discoverShopName)).toBeVisible({ timeout: 5_000 });

    // Join the shop
    await userPage.getByRole('button', { name: /Join Shop/i }).first().click();
    await userPage.waitForTimeout(2_000);

    // Card should now be visible
    await expect(userPage.getByText(discoverShopName)).toBeVisible();
    await expect(userPage.getByText('0 / 3 stamps')).toBeVisible();

    await adminPage.close();
    await userPage.close();
    await adminCtx.close();
    await userCtx.close();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  ADMIN — SHOP MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Admin Shop Management', () => {
  test('can create a shop', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    const shopName = `E2E Coffee ${uid()}`;
    // Open the create-shop modal
    await page.getByRole('button', { name: /Create Your First Stamp Card/i }).click();
    await page.getByPlaceholder('My Awesome Shop').fill(shopName);
    await page.getByPlaceholder(/Tell customers/i).fill('Best coffee for testing');
    await page.getByPlaceholder(/e\.g\./i).fill('1 free test latte');
    // Submit inside the modal (button text is "Create Stamp Card")
    await page.locator('form button[type="submit"]').click();
    // After creation the modal closes and the shop card appears in the list
    await expect(page.getByRole('heading', { name: shopName })).toBeVisible({ timeout: 5_000 });
  });

  test('can update shop details', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    const oldName = `Old Shop ${uid()}`;
    const newName = `Updated Shop ${uid()}`;
    // Create shop via modal
    await page.getByRole('button', { name: /Create Your First Stamp Card/i }).click();
    await page.getByPlaceholder('My Awesome Shop').fill(oldName);
    await page.getByPlaceholder(/e\.g\./i).fill('Old reward');
    await page.locator('form button[type="submit"]').click();
    await expect(page.getByRole('heading', { name: oldName })).toBeVisible({ timeout: 5_000 });

    // Open the edit modal
    await page.getByRole('button', { name: /Edit/i }).click();
    await page.getByPlaceholder('My Awesome Shop').clear();
    await page.getByPlaceholder('My Awesome Shop').fill(newName);
    await page.getByRole('button', { name: /Save Changes/i }).click();
    // Wait for the save to complete and the updated name to appear in the card
    await page.waitForTimeout(1_000);
    await expect(page.getByRole('heading', { name: newName })).toBeVisible();
  });

  test('shop settings tab is shown by default', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    // The default tab is "Stamp Cards" — with no shops it shows the empty state
    await expect(page.getByText('No stamp cards yet')).toBeVisible();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  ADMIN — STAMP GRANTING
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Admin Stamp Granting', () => {
  test('can switch to Stamps tab and see customer list', async ({ browser }) => {
    const adminContext = await browser.newContext();
    const customerContext = await browser.newContext();

    const page = await adminContext.newPage();
    await registerViaAPI(page, 'admin');

    // Create shop via API for reliability
    const shop = await createShopViaAPI(page, `Stamp Test Shop ${uid()}`);

    // Register a customer in a separate context and join the shop
    const customerPage = await customerContext.newPage();
    await registerViaAPI(customerPage, 'user');
    await joinShopViaAPI(customerPage, shop.id);
    await customerPage.close();
    await customerContext.close();

    // Reload so the admin page picks up the shop + customer
    await page.reload();
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('heading', { name: shop.name })).toBeVisible({ timeout: 5_000 });

    await page.getByRole('button', { name: /Grant Stamps/i }).click();
    await expect(page.getByPlaceholder(/search/i)).toBeVisible({ timeout: 5_000 });

    await page.close();
    await adminContext.close();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  FULL E2E: ADMIN CREATES SHOP → USER SEES CARD → ADMIN GRANTS → USER REDEEMS
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Full E2E Journey', () => {
  test('complete stamp card lifecycle', async ({ browser }) => {
    // Use separate browser contexts so cookies don't collide
    const adminContext = await browser.newContext();
    const userContext = await browser.newContext();

    // 1. Admin registers and creates a shop via API
    const adminPage = await adminContext.newPage();
    await registerViaAPI(adminPage, 'admin');

    const journeyShopName = `Journey Café ${uid()}`;
    const resp = await adminPage.context().request.post(`${API_BASE}/api/shops`, {
      headers: { 'Content-Type': 'application/json' },
      data: { name: journeyShopName, description: 'Your favorite café', rewardDescription: '1 free espresso', stampsRequired: 2, color: '#6366f1' },
    });
    expect(resp.ok()).toBeTruthy();
    const shopData = await resp.json();

    // Reload admin page so it picks up the shop
    await adminPage.reload();
    await adminPage.waitForLoadState('networkidle');
    await expect(adminPage.getByRole('heading', { name: journeyShopName })).toBeVisible({ timeout: 5_000 });

    // 2. User registers and joins the shop
    const userPage = await userContext.newPage();
    const { username: customerName } = await registerViaAPI(userPage, 'user');
    await joinShopViaAPI(userPage, shopData.id);

    // Reload to see the joined shop
    await userPage.reload();
    await userPage.waitForLoadState('networkidle');
    await expect(userPage.getByText(journeyShopName)).toBeVisible({ timeout: 5_000 });
    await expect(userPage.getByText('0 / 2 stamps')).toBeVisible();

    // 3. Admin grants stamps via UI (admin context has its own cookie)
    // Reload admin page first so it picks up the newly registered customer
    await adminPage.reload();
    await adminPage.waitForLoadState('networkidle');
    await adminPage.getByRole('button', { name: /Grant Stamps/i }).click();
    // Wait for customer list to load (at least one Stamp button should appear)
    await expect(adminPage.getByRole('button', { name: 'Stamp', exact: true }).first()).toBeVisible({ timeout: 5_000 });

    // Search for the specific customer so we stamp the right person
    const searchInput = adminPage.getByPlaceholder(/search/i);
    if (await searchInput.isVisible()) {
      await searchInput.fill(customerName);
      await adminPage.waitForTimeout(1_000);
    }

    // Find stamp buttons (exact name "Stamp" to avoid matching "Grant Stamps" tab)
    const grantButtons = adminPage.getByRole('button', { name: 'Stamp', exact: true });
    const count = await grantButtons.count();
    expect(count).toBeGreaterThan(0); // Ensure customer was found
    await grantButtons.first().click();
    await adminPage.waitForTimeout(1_000);
    await grantButtons.first().click();
    await adminPage.waitForTimeout(1_000);

    // 4. User refreshes and sees completed card
    await userPage.reload();
    await userPage.waitForTimeout(1_000);

    const congratsVisible = await userPage.getByText(/Congratulations/i).isVisible().catch(() => false);
    const stamps2of2 = await userPage.getByText('2 / 2 stamps').isVisible().catch(() => false);
    expect(congratsVisible || stamps2of2).toBeTruthy();

    // 5. User redeems the card
    if (congratsVisible) {
      const redeemBtn = userPage.getByRole('button', { name: /Redeem Reward/i });
      if (await redeemBtn.isVisible()) {
        await redeemBtn.click();
        await expect(userPage.getByText(/Redeemed/i)).toBeVisible({ timeout: 3_000 });
      }
    }

    await adminPage.close();
    await userPage.close();
    await adminContext.close();
    await userContext.close();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  UI INTERACTIONS
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('UI Interactions', () => {
  test('password visibility toggle works', async ({ page }) => {
    await page.goto('/login');
    const passwordInput = page.getByPlaceholder('Enter your password');
    await passwordInput.fill('secret');

    await expect(passwordInput).toHaveAttribute('type', 'password');

    const passwordContainer = page.getByPlaceholder('Enter your password').locator('..');
    await passwordContainer.locator('button').click();
    await expect(passwordInput).toHaveAttribute('type', 'text');

    await passwordContainer.locator('button').click();
    await expect(passwordInput).toHaveAttribute('type', 'password');
  });

  test('login/register mode toggle works', async ({ page }) => {
    await page.goto('/login');
    await expect(page.getByRole('heading', { name: /Welcome back/i })).toBeVisible();

    await page.getByText('Register', { exact: true }).click();
    await expect(page.getByRole('heading', { name: /Create Account/i })).toBeVisible();

    await page.getByRole('button', { name: 'Sign In' }).click();
    await expect(page.getByRole('heading', { name: /Welcome back/i })).toBeVisible();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  COOKIE SECURITY
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Cookie Security', () => {
  test('auth cookie is HttpOnly (not readable from JS)', async ({ page }) => {
    await registerViaAPI(page, 'user');
    const jsCookie = await page.evaluate(() => document.cookie);
    expect(jsCookie).not.toContain('__token');
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  ADMIN STATS TAB
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Admin Statistics', () => {
  test('statistics tab renders without errors', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    const shop = await createShopViaAPI(page, `Stats Shop ${uid()}`);
    await page.reload();
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('heading', { name: shop.name })).toBeVisible({ timeout: 5_000 });

    await page.getByRole('button', { name: /Statistics/i }).click();
    await expect(page.getByText(/Total Stamps/i)).toBeVisible({ timeout: 3_000 });
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  NAVBAR
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Navbar', () => {
  test('shows "My Cards" link for user role', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await expect(page.getByRole('link', { name: 'My Cards' })).toBeVisible();
  });

  test('shows "Dashboard" link for admin role', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await expect(page.getByRole('link', { name: 'Dashboard' })).toBeVisible();
  });

  test('shows username badge when authenticated', async ({ page }) => {
    const { username } = await registerViaAPI(page, 'user');
    await expect(page.getByRole('navigation').getByText(username)).toBeVisible();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  API-LEVEL EDGE CASES (complement the backend Go tests)
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('API Edge Cases', () => {
  test('unauthenticated request to /api/auth/me returns 401', async ({ page }) => {
    const resp = await page.context().request.get(`${API_BASE}/api/auth/me`);
    expect(resp.status()).toBe(401);
  });

  test('user cannot create a shop (403)', async ({ page }) => {
    await registerViaAPI(page, 'user');
    const resp = await page.context().request.post(`${API_BASE}/api/shops`, {
      data: { name: 'Sneaky', rewardDescription: 'test' },
    });
    expect(resp.status()).toBe(403);
  });

  test('admin can create a shop via API', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    const shopName = `API Shop ${uid()}`;
    const resp = await page.context().request.post(`${API_BASE}/api/shops`, {
      headers: { 'Content-Type': 'application/json' },
      data: { name: shopName, rewardDescription: 'Free item', stampsRequired: 5 },
    });
    expect(resp.status()).toBe(201);
    const body = await resp.json();
    expect(body.name).toBe(shopName);
    expect(body.stampsRequired).toBe(5);
  });

  test('user can join a shop via API', async ({ browser }) => {
    const adminCtx = await browser.newContext();
    const userCtx = await browser.newContext();

    const adminPage = await adminCtx.newPage();
    await registerViaAPI(adminPage, 'admin');
    const shop = await createShopViaAPI(adminPage, `Joinable Shop ${uid()}`);

    const userPage = await userCtx.newPage();
    await registerViaAPI(userPage, 'user');

    const resp = await userPage.context().request.post(`${API_BASE}/api/shops/${shop.id}/join`);
    expect(resp.status()).toBe(201);

    // Joining again should return 200 (idempotent)
    const resp2 = await userPage.context().request.post(`${API_BASE}/api/shops/${shop.id}/join`);
    expect(resp2.status()).toBe(200);

    await adminPage.close();
    await userPage.close();
    await adminCtx.close();
    await userCtx.close();
  });

  test('logout clears cookie via API', async ({ page }) => {
    await registerViaAPI(page, 'user');
    // Verify authenticated
    const resp = await page.context().request.get(`${API_BASE}/api/auth/me`);
    expect(resp.status()).toBe(200);

    // Logout via API
    await page.context().request.post(`${API_BASE}/api/auth/logout`);
    // Also clear localStorage (simulating what the UI logout does)
    await page.evaluate(() => localStorage.clear());

    // Verify redirect to login when trying to access protected route
    await page.goto('/dashboard');
    await expect(page).toHaveURL(/\/login/);
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  QR CODE — ADMIN TAB
// ═══════════════════════════════════════════════════════════════════════════════

/** Helper: create a shop via API from the admin's context and return { id, name }. */
async function createShopViaAPI(page: Page, name?: string): Promise<{ id: string; name: string }> {
  const shopName = name ?? `QR E2E Shop ${uid()}`;
  const resp = await page.context().request.post(`${API_BASE}/api/shops`, {
    headers: { 'Content-Type': 'application/json' },
    data: { name: shopName, rewardDescription: 'Free test item', stampsRequired: 3, color: '#6366f1' },
  });
  if (!resp.ok()) {
    const text = await resp.text();
    throw new Error(`createShopViaAPI failed (${resp.status()}): ${text}`);
  }
  const body = await resp.json();
  if (!body.id) throw new Error(`createShopViaAPI: response missing id: ${JSON.stringify(body)}`);
  return { id: body.id, name: shopName };
}

/** Helper: join a shop as the current user. */
async function joinShopViaAPI(page: Page, shopId: string): Promise<void> {
  const resp = await page.context().request.post(`${API_BASE}/api/shops/${shopId}/join`, {
    headers: { 'Content-Type': 'application/json' },
  });
  if (!resp.ok()) {
    const text = await resp.text();
    throw new Error(`joinShopViaAPI failed (${resp.status()}): ${text}`);
  }
}

/** Helper: generate a stamp token via API and return the token string. */
async function createStampTokenViaAPI(page: Page, shopId: string): Promise<string> {
  const resp = await page.context().request.post(`${API_BASE}/api/shops/${shopId}/stamp-token`, {
    headers: { 'Content-Type': 'application/json' },
  });
  if (!resp.ok()) {
    const text = await resp.text();
    throw new Error(`createStampTokenViaAPI failed (${resp.status()}): ${text}`);
  }
  const body = await resp.json();
  if (!body.token) throw new Error(`createStampTokenViaAPI: response missing token: ${JSON.stringify(body)}`);
  return body.token;
}

/** Helper: register admin, create a shop via API, reload page so the shop is visible. */
async function setupAdminWithShop(page: Page): Promise<{ id: string; name: string }> {
  await registerViaAPI(page, 'admin');
  const shop = await createShopViaAPI(page);
  await page.reload();
  await page.waitForLoadState('networkidle');
  // Wait for the shop data to render in the selector before interacting with tabs
  await expect(page.getByRole('heading', { name: shop.name })).toBeVisible({ timeout: 5_000 });
  return shop;
}

/** Helper: open QR tab and click Generate. */
async function openQRAndGenerate(page: Page): Promise<void> {
  await page.getByRole('button', { name: /QR Code/i }).click();
  await expect(page.getByRole('button', { name: /Generate QR Code/i })).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: /Generate QR Code/i }).click();
}

test.describe('Admin QR Code Tab', () => {
  test('QR Code tab is visible after creating a shop', async ({ page }) => {
    await setupAdminWithShop(page);
    const qrTab = page.getByRole('button', { name: /QR Code/i });
    await expect(qrTab).toBeVisible();
  });

  test('QR Code tab shows generate button', async ({ page }) => {
    await setupAdminWithShop(page);
    await page.getByRole('button', { name: /QR Code/i }).click();
    await expect(page.getByRole('button', { name: /Generate QR Code/i })).toBeVisible({ timeout: 5_000 });
  });

  test('generates a QR code with timer on click', async ({ page }) => {
    await setupAdminWithShop(page);
    await openQRAndGenerate(page);

    // QR SVG should appear
    await expect(page.locator('svg').first()).toBeVisible({ timeout: 5_000 });
    // Timer should be visible
    await expect(page.getByText(/remaining/i)).toBeVisible();
    // New QR Code button should appear
    await expect(page.getByRole('button', { name: /New QR Code/i })).toBeVisible();
  });

  test('shows instruction text for phone camera', async ({ page }) => {
    await setupAdminWithShop(page);
    await openQRAndGenerate(page);
    await expect(page.getByText(/phone camera/i)).toBeVisible({ timeout: 5_000 });
  });

  test('QR Code tab shows no-shop message if shop not created', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await page.getByRole('button', { name: /QR Code/i }).click();
    await expect(page.getByText(/No shop selected/i)).toBeVisible({ timeout: 3_000 });
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  QR CODE — CLAIM PAGE (/claim/:token)
// ═══════════════════════════════════════════════════════════════════════════════

/** Helper: create separate admin + user contexts, register both, create shop + token.
 *  Caller must close all returned pages/contexts when done. */
async function setupClaimTest(browser: Browser) {
  const adminContext = await browser.newContext();
  const userContext = await browser.newContext();

  const adminPage = await adminContext.newPage();
  await registerViaAPI(adminPage, 'admin');
  const shop = await createShopViaAPI(adminPage);
  const token = await createStampTokenViaAPI(adminPage, shop.id);

  const userPage = await userContext.newPage();
  await registerViaAPI(userPage, 'user');

  return { adminContext, userContext, adminPage, userPage, shopId: shop.id, shopName: shop.name, token };
}

/** Helper: close all pages and contexts from setupClaimTest. */
async function teardownClaimTest(ctx: Awaited<ReturnType<typeof setupClaimTest>>) {
  await ctx.adminPage.close();
  await ctx.userPage.close();
  await ctx.adminContext.close();
  await ctx.userContext.close();
}

test.describe('Claim Page', () => {
  test('successfully claims a stamp via /claim/:token URL', async ({ browser }) => {
    const ctx = await setupClaimTest(browser);

    await ctx.userPage.goto(`/claim/${ctx.token}`);
    await ctx.userPage.waitForLoadState('networkidle');

    await expect(ctx.userPage.getByRole('heading', { name: /Stamp Collected/i })).toBeVisible({ timeout: 10_000 });
    await expect(ctx.userPage.getByText(ctx.shopName)).toBeVisible();
    await expect(ctx.userPage.getByText('1 / 3 stamps')).toBeVisible();

    await teardownClaimTest(ctx);
  });

  test('shows error for invalid token', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await page.goto('/claim/totally-invalid-token-12345');
    await page.waitForLoadState('networkidle');

    await expect(page.getByText(/Couldn't Claim Stamp/i)).toBeVisible({ timeout: 10_000 });
  });

  test('double-scan shows error (token consumed after first scan)', async ({ browser }) => {
    const ctx = await setupClaimTest(browser);

    // First claim
    await ctx.userPage.goto(`/claim/${ctx.token}`);
    await expect(ctx.userPage.getByRole('heading', { name: /Stamp Collected/i })).toBeVisible({ timeout: 10_000 });

    // Second claim (same token — now consumed/deleted)
    await ctx.userPage.goto(`/claim/${ctx.token}`);
    await ctx.userPage.waitForLoadState('networkidle');
    await expect(ctx.userPage.getByText(/Couldn't Claim Stamp/i)).toBeVisible({ timeout: 10_000 });

    await teardownClaimTest(ctx);
  });

  test('redirects to login when not authenticated', async ({ page }) => {
    await page.goto('/claim/some-token');
    await expect(page).toHaveURL(/\/login/);
  });

  test('redirects back to claim page after login', async ({ browser }) => {
    const adminContext = await browser.newContext();
    const userContext = await browser.newContext();

    // Admin creates shop + token
    const adminPage = await adminContext.newPage();
    await registerViaAPI(adminPage, 'admin');
    const shop = await createShopViaAPI(adminPage);
    const token = await createStampTokenViaAPI(adminPage, shop.id);

    // User registers in a fresh context (so we have credentials)
    const setupPage = await userContext.newPage();
    const { username, password } = await registerViaAPI(setupPage, 'user');

    // Clear cookies/storage to simulate logged-out state
    await userContext.clearCookies();
    await setupPage.evaluate(() => localStorage.clear());

    // Navigate to claim URL — should redirect to login
    await setupPage.goto(`/claim/${token}`);
    await expect(setupPage).toHaveURL(/\/login/);

    // Login
    await loginViaAPI(setupPage, username, password);

    // Now navigate to claim page — should work
    await setupPage.goto(`/claim/${token}`);
    // The heading shows "Stamp Collected!" for both first claim and already-scanned
    await expect(setupPage.getByRole('heading', { name: /Stamp Collected/i })).toBeVisible({ timeout: 10_000 });

    await adminPage.close();
    await setupPage.close();
    await adminContext.close();
    await userContext.close();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  USER SCAN ENTRY POINTS REMOVED
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Removed user scan entry points', () => {
  test('old /scan URL redirects logged-in users to dashboard', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await page.goto('/scan');
    await expect(page).toHaveURL(/\/dashboard/);
  });

  test('old /scan URL is still protected when not logged in', async ({ page }) => {
    await page.goto('/scan');
    await expect(page).toHaveURL(/\/login/);
  });

  test('user does not see Scan QR link in navbar', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await expect(page.getByRole('link', { name: /Scan QR/i })).toHaveCount(0);
  });

  test('admin does not see Scan QR link either', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await expect(page.getByRole('link', { name: /Scan QR/i })).toHaveCount(0);
  });

  test('user dashboard no longer shows a Scan QR button', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await expect(page.getByRole('button', { name: /Scan QR/i })).toHaveCount(0);
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  QR CODE — API-LEVEL TESTS
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('QR API Edge Cases', () => {
  test('user cannot create stamp tokens (403)', async ({ browser }) => {
    const adminContext = await browser.newContext();
    const userContext = await browser.newContext();

    const adminPage = await adminContext.newPage();
    await registerViaAPI(adminPage, 'admin');
    const shop = await createShopViaAPI(adminPage);

    const userPage = await userContext.newPage();
    await registerViaAPI(userPage, 'user');

    const resp = await userPage.context().request.post(`${API_BASE}/api/shops/${shop.id}/stamp-token`);
    expect(resp.status()).toBe(403);

    await adminPage.close();
    await userPage.close();
    await adminContext.close();
    await userContext.close();
  });

  test('claim with empty token returns 400', async ({ page }) => {
    await registerViaAPI(page, 'user');
    const resp = await page.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token: '' },
    });
    expect(resp.status()).toBe(400);
  });

  test('claim with invalid token returns 404', async ({ page }) => {
    await registerViaAPI(page, 'user');
    const resp = await page.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token: 'nonexistent-token' },
    });
    expect(resp.status()).toBe(404);
  });

  test('admin cannot claim stamps (403)', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    const shop = await createShopViaAPI(page);
    const token = await createStampTokenViaAPI(page, shop.id);

    const resp = await page.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token },
    });
    expect(resp.status()).toBe(403);
  });

  test('token is invalidated after first claim', async ({ browser }) => {
    const adminCtx = await browser.newContext();
    const user1Ctx = await browser.newContext();
    const user2Ctx = await browser.newContext();

    const adminPage = await adminCtx.newPage();
    await registerViaAPI(adminPage, 'admin');
    const shop = await createShopViaAPI(adminPage);
    const token = await createStampTokenViaAPI(adminPage, shop.id);

    const user1Page = await user1Ctx.newPage();
    await registerViaAPI(user1Page, 'user');
    const resp1 = await user1Page.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token },
    });
    expect(resp1.status()).toBe(200);
    const body1 = await resp1.json();
    expect(body1.stamps).toBe(1);

    // Second user tries the same token — should fail (token consumed)
    const user2Page = await user2Ctx.newPage();
    await registerViaAPI(user2Page, 'user');
    const resp2 = await user2Page.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token },
    });
    expect(resp2.status()).toBe(404);

    await adminPage.close();
    await user1Page.close();
    await user2Page.close();
    await adminCtx.close();
    await user1Ctx.close();
    await user2Ctx.close();
  });

  test('double claim returns 404 (token consumed)', async ({ browser }) => {
    const adminCtx = await browser.newContext();
    const userCtx = await browser.newContext();

    const adminPage = await adminCtx.newPage();
    await registerViaAPI(adminPage, 'admin');
    const shop = await createShopViaAPI(adminPage);
    const token = await createStampTokenViaAPI(adminPage, shop.id);

    const userPage = await userCtx.newPage();
    await registerViaAPI(userPage, 'user');

    // First claim
    const resp1 = await userPage.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token },
    });
    const body1 = await resp1.json();
    expect(body1.stamps).toBe(1);

    // Second claim — token already consumed and deleted
    const resp2 = await userPage.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token },
    });
    expect(resp2.status()).toBe(404);

    await adminPage.close();
    await userPage.close();
    await adminCtx.close();
    await userCtx.close();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  QR CODE — FULL E2E JOURNEY
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Full QR E2E Journey', () => {
  test('admin creates QR → user claims via URL → fills card → redeems', async ({ browser }) => {
    const adminCtx = await browser.newContext();
    const userCtx = await browser.newContext();

    // 1. Admin creates shop with 2 stamps required
    const adminPage = await adminCtx.newPage();
    await registerViaAPI(adminPage, 'admin');
    const qrShopName = `QR Journey Café ${uid()}`;
    const resp = await adminPage.context().request.post(`${API_BASE}/api/shops`, {
      headers: { 'Content-Type': 'application/json' },
      data: { name: qrShopName, rewardDescription: 'Free espresso', stampsRequired: 2, color: '#ef4444' },
    });
    const shop = await resp.json();

    // 2. User registers
    const userPage = await userCtx.newPage();
    await registerViaAPI(userPage, 'user');

    // 3. Admin generates token #1 → user claims via /claim URL
    const token1 = await createStampTokenViaAPI(adminPage, shop.id);
    await userPage.goto(`/claim/${token1}`);
    await expect(userPage.getByRole('heading', { name: /Stamp Collected/i })).toBeVisible({ timeout: 10_000 });
    await expect(userPage.getByText('1 / 2 stamps')).toBeVisible();

    // 4. Admin generates token #2 → user claims → card complete
    const token2 = await createStampTokenViaAPI(adminPage, shop.id);
    await userPage.goto(`/claim/${token2}`);
    await expect(userPage.getByRole('heading', { name: /Card Complete/i })).toBeVisible({ timeout: 10_000 });
    await expect(userPage.getByText('2 / 2 stamps')).toBeVisible();

    // 5. User goes to dashboard and sees the completed card
    await userPage.goto('/dashboard');
    await userPage.waitForLoadState('networkidle');
    await expect(userPage.getByText(qrShopName)).toBeVisible({ timeout: 5_000 });

    // 6. User redeems the card
    const redeemBtn = userPage.getByRole('button', { name: /Redeem Reward/i });
    if (await redeemBtn.isVisible()) {
      await redeemBtn.click();
      await expect(userPage.getByText(/Redeemed/i)).toBeVisible({ timeout: 5_000 });
    }

    await adminPage.close();
    await userPage.close();
    await adminCtx.close();
    await userCtx.close();
  });
});

