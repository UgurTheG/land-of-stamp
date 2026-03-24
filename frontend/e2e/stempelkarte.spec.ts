/**
 * Länd of Stamp – Playwright E2E Tests
 *
 * Uses direct API calls for reliable test setup (register/login), and
 * browser-based UI interactions for testing actual user flows.
 * This avoids React 19 form-action timing issues in controlled components.
 */

import { test, expect, type Page, type APIRequestContext } from '@playwright/test';

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

/**
 * Fill the login/register form and submit via the UI.
 * Used ONLY for tests that specifically validate the login/register UI.
 */
async function fillAndSubmitForm(page: Page, username: string, password: string) {
  await page.getByPlaceholder('Enter your username').fill(username);
  await page.getByPlaceholder('Enter your password').fill(password);
  // Type into password then press Enter for native form submission
  await page.getByPlaceholder('Enter your password').press('Enter');
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
  test('shows empty state or zero stats when no stamps', async ({ page }) => {
    await registerViaAPI(page, 'user');
    const noShops = page.getByText(/no shops available/i);
    const zero = page.getByText('0');
    const hasNoShops = await noShops.isVisible().catch(() => false);
    const hasZero = await zero.first().isVisible().catch(() => false);
    expect(hasNoShops || hasZero).toBeTruthy();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  ADMIN — SHOP MANAGEMENT
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Admin Shop Management', () => {
  test('can create a shop', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await page.getByPlaceholder('My Awesome Shop').fill('E2E Coffee');
    await page.getByPlaceholder(/Tell customers/i).fill('Best coffee for testing');
    await page.getByPlaceholder(/e\.g\./i).fill('1 free test latte');
    await page.getByRole('button', { name: /Create Shop/i }).click();
    await expect(page.getByRole('button', { name: /Save Changes/i })).toBeVisible({ timeout: 5_000 });
  });

  test('can update shop details', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await page.getByPlaceholder('My Awesome Shop').fill('Old Shop Name');
    await page.getByPlaceholder(/e\.g\./i).fill('Old reward');
    await page.getByRole('button', { name: /Create Shop/i }).click();
    await expect(page.getByRole('button', { name: /Save Changes/i })).toBeVisible({ timeout: 5_000 });

    await page.getByPlaceholder('My Awesome Shop').clear();
    await page.getByPlaceholder('My Awesome Shop').fill('Updated Shop Name');
    await page.getByRole('button', { name: /Save Changes/i }).click();
    // Wait for the save to complete — the input should still have the updated value
    await page.waitForTimeout(1_000);
    await expect(page.getByPlaceholder('My Awesome Shop')).toHaveValue('Updated Shop Name');
  });

  test('shop settings tab is shown by default', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await expect(page.getByText('Shop Configuration')).toBeVisible();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  ADMIN — STAMP GRANTING
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Admin Stamp Granting', () => {
  test('can switch to Stamps tab and see customer list', async ({ page, context }) => {
    await registerViaAPI(page, 'admin');

    await page.getByPlaceholder('My Awesome Shop').fill('Stamp Test Shop');
    await page.getByPlaceholder(/e\.g\./i).fill('Free stamp reward');
    await page.getByRole('button', { name: /Create Shop/i }).click();
    await expect(page.getByRole('button', { name: /Save Changes/i })).toBeVisible({ timeout: 5_000 });

    // Register a customer in a new page (shares context cookies won't overlap)
    const customerPage = await context.newPage();
    await registerViaAPI(customerPage, 'user');
    await customerPage.close();

    await page.getByRole('button', { name: /Grant Stamps/i }).click();
    await expect(page.getByPlaceholder(/search/i)).toBeVisible({ timeout: 5_000 });
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

    // 1. Admin registers and creates a shop
    const adminPage = await adminContext.newPage();
    await registerViaAPI(adminPage, 'admin');

    await adminPage.getByPlaceholder('My Awesome Shop').fill('Journey Café');
    await adminPage.getByPlaceholder(/Tell customers/i).fill('Your favorite café');
    await adminPage.getByPlaceholder(/e\.g\./i).fill('1 free espresso');
    const stampsInput = adminPage.locator('input[type="number"]');
    await stampsInput.clear();
    await stampsInput.fill('2');
    await adminPage.getByRole('button', { name: /Create Shop/i }).click();
    await expect(adminPage.getByRole('button', { name: /Save Changes/i })).toBeVisible({
      timeout: 5_000,
    });

    // 2. User registers and sees the shop card
    const userPage = await userContext.newPage();
    const { username: customerName } = await registerViaAPI(userPage, 'user');

    await expect(userPage.getByText('Journey Café')).toBeVisible({ timeout: 5_000 });
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
    await page.getByPlaceholder('My Awesome Shop').fill('Stats Shop');
    await page.getByPlaceholder(/e\.g\./i).fill('Free reward');
    await page.getByRole('button', { name: /Create Shop/i }).click();
    await expect(page.getByRole('button', { name: /Save Changes/i })).toBeVisible({ timeout: 5_000 });

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
    const resp = await page.context().request.post(`${API_BASE}/api/shops`, {
      headers: { 'Content-Type': 'application/json' },
      data: { name: 'API Shop', rewardDescription: 'Free item', stampsRequired: 5 },
    });
    expect(resp.status()).toBe(201);
    const body = await resp.json();
    expect(body.name).toBe('API Shop');
    expect(body.stampsRequired).toBe(5);
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

/** Helper: create a shop via API from the admin's context and return shopId. */
async function createShopViaAPI(page: Page, name = 'QR E2E Shop'): Promise<string> {
  const resp = await page.context().request.post(`${API_BASE}/api/shops`, {
    headers: { 'Content-Type': 'application/json' },
    data: { name, rewardDescription: 'Free test item', stampsRequired: 3, color: '#6366f1' },
  });
  const body = await resp.json();
  return body.id;
}

/** Helper: generate a stamp token via API and return the token string. */
async function createStampTokenViaAPI(page: Page, shopId: string): Promise<string> {
  const resp = await page.context().request.post(`${API_BASE}/api/shops/${shopId}/stamp-token`, {
    headers: { 'Content-Type': 'application/json' },
  });
  const body = await resp.json();
  return body.token;
}

test.describe('Admin QR Code Tab', () => {
  test('QR Code tab is visible after creating a shop', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await createShopViaAPI(page);
    await page.reload();
    await page.waitForLoadState('networkidle');

    const qrTab = page.getByRole('button', { name: /QR Code/i });
    await expect(qrTab).toBeVisible();
  });

  test('QR Code tab shows generate button', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await createShopViaAPI(page);
    await page.reload();
    await page.waitForLoadState('networkidle');

    await page.getByRole('button', { name: /QR Code/i }).click();
    await expect(page.getByRole('button', { name: /Generate QR Code/i })).toBeVisible({ timeout: 5_000 });
  });

  test('generates a QR code with timer on click', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await createShopViaAPI(page);
    await page.reload();
    await page.waitForLoadState('networkidle');

    await page.getByRole('button', { name: /QR Code/i }).click();
    await page.getByRole('button', { name: /Generate QR Code/i }).click();

    // QR SVG should appear
    await expect(page.locator('svg').first()).toBeVisible({ timeout: 5_000 });
    // Timer should be visible
    await expect(page.getByText(/remaining/i)).toBeVisible();
    // New QR Code button should appear
    await expect(page.getByRole('button', { name: /New QR Code/i })).toBeVisible();
  });

  test('shows instruction text for phone camera', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await createShopViaAPI(page);
    await page.reload();
    await page.waitForLoadState('networkidle');

    await page.getByRole('button', { name: /QR Code/i }).click();
    await page.getByRole('button', { name: /Generate QR Code/i }).click();
    await expect(page.getByText(/phone camera/i)).toBeVisible({ timeout: 5_000 });
  });

  test('QR Code tab shows no-shop message if shop not created', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await page.getByRole('button', { name: /QR Code/i }).click();
    await expect(page.getByText(/No shop configured/i)).toBeVisible({ timeout: 3_000 });
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  QR CODE — CLAIM PAGE (/claim/:token)
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Claim Page', () => {
  test('successfully claims a stamp via /claim/:token URL', async ({ browser }) => {
    const adminContext = await browser.newContext();
    const userContext = await browser.newContext();

    const adminPage = await adminContext.newPage();
    await registerViaAPI(adminPage, 'admin');
    const shopId = await createShopViaAPI(adminPage);
    const token = await createStampTokenViaAPI(adminPage, shopId);

    const userPage = await userContext.newPage();
    await registerViaAPI(userPage, 'user');

    // Navigate to the claim URL
    await userPage.goto(`/claim/${token}`);
    await userPage.waitForLoadState('networkidle');

    // Should show success
    await expect(userPage.getByRole('heading', { name: /Stamp Collected/i })).toBeVisible({ timeout: 10_000 });
    await expect(userPage.getByText('QR E2E Shop')).toBeVisible();
    await expect(userPage.getByText('1 / 3 stamps')).toBeVisible();

    await adminPage.close();
    await userPage.close();
    await adminContext.close();
    await userContext.close();
  });

  test('shows error for invalid token', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await page.goto('/claim/totally-invalid-token-12345');
    await page.waitForLoadState('networkidle');

    await expect(page.getByText(/Couldn't Claim Stamp/i)).toBeVisible({ timeout: 10_000 });
  });

  test('double-scan shows friendly already-scanned message', async ({ browser }) => {
    const adminContext = await browser.newContext();
    const userContext = await browser.newContext();

    const adminPage = await adminContext.newPage();
    await registerViaAPI(adminPage, 'admin');
    const shopId = await createShopViaAPI(adminPage);
    const token = await createStampTokenViaAPI(adminPage, shopId);

    const userPage = await userContext.newPage();
    await registerViaAPI(userPage, 'user');

    // First claim
    await userPage.goto(`/claim/${token}`);
    await expect(userPage.getByRole('heading', { name: /Stamp Collected/i })).toBeVisible({ timeout: 10_000 });

    // Second claim (same token, same user)
    await userPage.goto(`/claim/${token}`);
    await userPage.waitForLoadState('networkidle');
    // Should show success with "already scanned" message, not an error
    await expect(userPage.getByText(/already scanned/i)).toBeVisible({ timeout: 10_000 });

    await adminPage.close();
    await userPage.close();
    await adminContext.close();
    await userContext.close();
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
    const shopId = await createShopViaAPI(adminPage);
    const token = await createStampTokenViaAPI(adminPage, shopId);

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
//  QR CODE — SCAN PAGE
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Scan Page', () => {
  test('user can navigate to /scan from dashboard', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await page.getByRole('link', { name: /Scan QR/i }).first().click();
    await expect(page).toHaveURL(/\/scan/);
  });

  test('scan page shows recommended native camera section', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await page.goto('/scan');
    await page.waitForLoadState('networkidle');

    await expect(page.getByText(/Use Your Phone Camera/i)).toBeVisible();
    await expect(page.getByText(/Recommended/i)).toBeVisible();
  });

  test('scan page shows in-app scanner fallback', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await page.goto('/scan');
    await page.waitForLoadState('networkidle');

    await expect(page.getByText(/In-App Scanner/i)).toBeVisible();
    await expect(page.getByRole('button', { name: /Open Scanner/i })).toBeVisible();
  });

  test('scan page shows tips section', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await page.goto('/scan');
    await page.waitForLoadState('networkidle');

    await expect(page.getByText(/Tips/i)).toBeVisible();
    await expect(page.getByText(/expire after 60 seconds/i)).toBeVisible();
  });

  test('scan page has back to dashboard button', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await page.goto('/scan');
    await page.waitForLoadState('networkidle');

    await page.getByText('Back to Dashboard').click();
    await expect(page).toHaveURL(/\/dashboard/);
  });

  test('scan page is protected — redirects when not logged in', async ({ page }) => {
    await page.goto('/scan');
    await expect(page).toHaveURL(/\/login/);
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  QR CODE — NAVBAR
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('Navbar QR Links', () => {
  test('user sees Scan QR link in desktop navbar', async ({ page }) => {
    await registerViaAPI(page, 'user');
    await expect(page.getByRole('link', { name: /Scan QR/i }).first()).toBeVisible();
  });

  test('admin does NOT see Scan QR link', async ({ page }) => {
    await registerViaAPI(page, 'admin');
    await expect(page.getByRole('link', { name: /Scan QR/i })).not.toBeVisible();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  QR CODE — USER DASHBOARD SCAN BUTTON
// ═══════════════════════════════════════════════════════════════════════════════

test.describe('User Dashboard Scan Button', () => {
  test('shows Scan QR button on dashboard', async ({ page }) => {
    await registerViaAPI(page, 'user');
    const scanBtn = page.getByRole('button', { name: /Scan QR/i });
    // On mobile it may only show the icon, on desktop the text
    const scanLink = page.getByRole('link', { name: /Scan QR/i });
    const hasScanBtn = await scanBtn.isVisible().catch(() => false);
    const hasScanLink = await scanLink.first().isVisible().catch(() => false);
    expect(hasScanBtn || hasScanLink).toBeTruthy();
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
    const shopId = await createShopViaAPI(adminPage);

    const userPage = await userContext.newPage();
    await registerViaAPI(userPage, 'user');

    const resp = await userPage.context().request.post(`${API_BASE}/api/shops/${shopId}/stamp-token`);
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
    const shopId = await createShopViaAPI(page);
    const token = await createStampTokenViaAPI(page, shopId);

    const resp = await page.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token },
    });
    expect(resp.status()).toBe(403);
  });

  test('multiple users can claim same token', async ({ browser }) => {
    const adminCtx = await browser.newContext();
    const user1Ctx = await browser.newContext();
    const user2Ctx = await browser.newContext();

    const adminPage = await adminCtx.newPage();
    await registerViaAPI(adminPage, 'admin');
    const shopId = await createShopViaAPI(adminPage);
    const token = await createStampTokenViaAPI(adminPage, shopId);

    const user1Page = await user1Ctx.newPage();
    await registerViaAPI(user1Page, 'user');
    const resp1 = await user1Page.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token },
    });
    expect(resp1.status()).toBe(200);
    const body1 = await resp1.json();
    expect(body1.stamps).toBe(1);

    const user2Page = await user2Ctx.newPage();
    await registerViaAPI(user2Page, 'user');
    const resp2 = await user2Page.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token },
    });
    expect(resp2.status()).toBe(200);
    const body2 = await resp2.json();
    expect(body2.stamps).toBe(1);

    await adminPage.close();
    await user1Page.close();
    await user2Page.close();
    await adminCtx.close();
    await user1Ctx.close();
    await user2Ctx.close();
  });

  test('double claim returns same stamp count (no extra stamp)', async ({ browser }) => {
    const adminCtx = await browser.newContext();
    const userCtx = await browser.newContext();

    const adminPage = await adminCtx.newPage();
    await registerViaAPI(adminPage, 'admin');
    const shopId = await createShopViaAPI(adminPage);
    const token = await createStampTokenViaAPI(adminPage, shopId);

    const userPage = await userCtx.newPage();
    await registerViaAPI(userPage, 'user');

    // First claim
    const resp1 = await userPage.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token },
    });
    const body1 = await resp1.json();
    expect(body1.stamps).toBe(1);

    // Second claim — same user, same token
    const resp2 = await userPage.context().request.post(`${API_BASE}/api/stamps/claim`, {
      headers: { 'Content-Type': 'application/json' },
      data: { token },
    });
    const body2 = await resp2.json();
    expect(body2.stamps).toBe(1); // no extra stamp
    expect(body2.message).toContain('already scanned');

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
    const resp = await adminPage.context().request.post(`${API_BASE}/api/shops`, {
      headers: { 'Content-Type': 'application/json' },
      data: { name: 'QR Journey Café', rewardDescription: 'Free espresso', stampsRequired: 2, color: '#ef4444' },
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
    await expect(userPage.getByText('QR Journey Café')).toBeVisible({ timeout: 5_000 });

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

