package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/docs"
	"land-of-stamp-backend/middleware"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"land-of-stamp-backend/handlers"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// setupTestServer creates a fresh in-memory DB, initialises auth, and returns
// an httptest.Server whose routing matches main.go but uses flat registration
// so that tests can call paths without worrying about sub-mux trailing-slash
// redirects.  All middleware (Auth, AdminOnly, CORS) is still exercised.
func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Use a temp-file SQLite DB so it survives across requests in the same test.
	tmpFile, err := os.CreateTemp("", "land-of-stamp-test-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp db: %v", err)
	}
	t.Cleanup(func() { db.Close(context.Background()); _ = os.Remove(tmpFile.Name()) })

	auth.Init("test-secret-key-for-e2e")
	db.Init(context.Background(), tmpFile.Name())

	// Middleware wrappers matching main.go's Auth / AdminOnly chains.
	withAuth := func(fn http.HandlerFunc) http.Handler {
		return middleware.Auth(fn)
	}
	withAdmin := func(fn http.HandlerFunc) http.Handler {
		return middleware.Auth(middleware.AdminOnly(fn))
	}

	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("POST /api/auth/register", handlers.Register)
	mux.HandleFunc("POST /api/auth/login", handlers.Login)
	mux.HandleFunc("POST /api/auth/logout", handlers.Logout)
	mux.HandleFunc("GET /api/shops", handlers.ListShops)

	// Authenticated routes
	mux.Handle("GET /api/auth/me", withAuth(handlers.GetMe))
	mux.Handle("GET /api/users/me/cards", withAuth(handlers.GetMyCards))
	mux.Handle("POST /api/cards/{id}/redeem", withAuth(handlers.RedeemCard))
	mux.Handle("POST /api/stamps/claim", withAuth(handlers.ClaimStamp))
	mux.Handle("POST /api/shops/{id}/join", withAuth(handlers.JoinShop))

	// Admin routes
	mux.Handle("POST /api/shops", withAdmin(handlers.CreateShop))
	mux.Handle("PUT /api/shops/{id}", withAdmin(handlers.UpdateShop))
	mux.Handle("GET /api/shops/mine", withAdmin(handlers.GetMyShops))
	mux.Handle("GET /api/shops/{id}/cards", withAdmin(handlers.GetShopCards))
	mux.Handle("GET /api/shops/{id}/customers", withAdmin(handlers.GetShopCustomers))
	mux.Handle("POST /api/shops/{id}/stamps", withAdmin(handlers.GrantStamp))
	mux.Handle("PATCH /api/shops/{id}/stamps", withAdmin(handlers.UpdateStampCount))
	mux.Handle("POST /api/shops/{id}/stamp-token", withAdmin(handlers.CreateStampToken))

	// Documentation endpoints
	docs.Register(mux)

	handler := middleware.RequestLog(middleware.CORS(mux))
	return httptest.NewServer(handler)
}

func basicAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

type requestOpts struct {
	method  string
	path    string
	body    string
	headers map[string]string
	cookies []*http.Cookie
}

func doRequest(t *testing.T, ts *httptest.Server, opts requestOpts) (*http.Response, map[string]any) {
	t.Helper()
	var bodyReader io.Reader
	if opts.body != "" {
		bodyReader = strings.NewReader(opts.body)
	}
	req, err := http.NewRequest(opts.method, ts.URL+opts.path, bodyReader)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range opts.headers {
		req.Header.Set(k, v)
	}
	for _, c := range opts.cookies {
		req.AddCookie(c)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var result map[string]any
	_ = json.Unmarshal(raw, &result) // may fail for arrays; caller handles
	return resp, result
}

func doRequestArray(t *testing.T, ts *httptest.Server, opts requestOpts) (*http.Response, []map[string]any) {
	t.Helper()
	var bodyReader io.Reader
	if opts.body != "" {
		bodyReader = strings.NewReader(opts.body)
	}
	req, err := http.NewRequest(opts.method, ts.URL+opts.path, bodyReader)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range opts.headers {
		req.Header.Set(k, v)
	}
	for _, c := range opts.cookies {
		req.AddCookie(c)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var result []map[string]any
	_ = json.Unmarshal(raw, &result)
	return resp, result
}

// extractCookie finds a cookie by name from the response.
func extractCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// registerUser is a convenience helper that registers and returns the cookie.
func registerUser(t *testing.T, ts *httptest.Server, username, password, role string) (*http.Cookie, map[string]any) {
	t.Helper()
	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/register",
		body:    fmt.Sprintf(`{"role":"%s"}`, role),
		headers: map[string]string{"Authorization": basicAuth(username, password)},
	})
	cookie := extractCookie(resp, "__token")
	if cookie == nil {
		t.Fatalf("register %s: no __token cookie returned; status=%d body=%v", username, resp.StatusCode, body)
	}
	return cookie, body
}

// createShopHelper registers an admin, creates a shop, and returns the cookie + shop ID.
func createShopHelper(t *testing.T, ts *httptest.Server, adminUsername string) (*http.Cookie, string) {
	t.Helper()
	adminCookie, _ := registerUser(t, ts, adminUsername, "admin1234", "admin")
	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"` + adminUsername + ` Shop","rewardDescription":"Free item","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	return adminCookie, shopBody["id"].(string)
}

// ═══════════════════════════════════════════════════════════════════════════════
//
//	AUTH TESTS
//
// ═══════════════════════════════════════════════════════════════════════════════

func TestRegister_HappyPath_User(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/register",
		body:    `{"role":"user"}`,
		headers: map[string]string{"Authorization": basicAuth("alice", "secret1234")},
	})

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	user := body["user"].(map[string]any)
	if user["username"] != "alice" {
		t.Errorf("expected username alice, got %v", user["username"])
	}
	if user["role"] != "user" {
		t.Errorf("expected role user, got %v", user["role"])
	}
	if extractCookie(resp, "__token") == nil {
		t.Error("expected __token cookie to be set")
	}
}

func TestRegister_HappyPath_Admin(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/register",
		body:    `{"role":"admin"}`,
		headers: map[string]string{"Authorization": basicAuth("shopowner", "pass1234")},
	})

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	user := body["user"].(map[string]any)
	if user["role"] != "admin" {
		t.Errorf("expected role admin, got %v", user["role"])
	}
}

func TestRegister_MissingAuthHeader(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, body := doRequest(t, ts, requestOpts{
		method: http.MethodPost,
		path:   "/api/auth/register",
		body:   `{"role":"user"}`,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %v", resp.StatusCode, body)
	}
}

func TestRegister_ShortUsername(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/register",
		body:    `{"role":"user"}`,
		headers: map[string]string{"Authorization": basicAuth("a", "secret1234")},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %v", resp.StatusCode, body)
	}
	if body["error"] == nil || !strings.Contains(body["error"].(string), "username") {
		t.Errorf("expected username-related error, got %v", body["error"])
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/register",
		body:    `{"role":"user"}`,
		headers: map[string]string{"Authorization": basicAuth("alice", "abc")},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %v", resp.StatusCode, body)
	}
	if body["error"] == nil || !strings.Contains(body["error"].(string), "password") {
		t.Errorf("expected password-related error, got %v", body["error"])
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	registerUser(t, ts, "alice", "secret1234", "user")

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/register",
		body:    `{"role":"user"}`,
		headers: map[string]string{"Authorization": basicAuth("alice", "otherpass")},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %v", resp.StatusCode, body)
	}
}

func TestRegister_InvalidRole_DefaultsToUser(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/register",
		body:    `{"role":"superadmin"}`,
		headers: map[string]string{"Authorization": basicAuth("bob", "pass1234")},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	user := body["user"].(map[string]any)
	if user["role"] != "user" {
		t.Errorf("invalid role should default to 'user', got %v", user["role"])
	}
}

func TestRegister_NoBody_DefaultsToUser(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/register",
		headers: map[string]string{"Authorization": basicAuth("charlie", "pass1234")},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	user := body["user"].(map[string]any)
	if user["role"] != "user" {
		t.Errorf("empty body should default to 'user', got %v", user["role"])
	}
}

func TestRegister_MalformedBasicAuth(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// no colon separator
	badEncoded := base64.StdEncoding.EncodeToString([]byte("justusername"))
	resp, _ := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/register",
		headers: map[string]string{"Authorization": "Basic " + badEncoded},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed basic auth, got %d", resp.StatusCode)
	}
}

func TestLogin_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	registerUser(t, ts, "alice", "secret1234", "user")

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/login",
		headers: map[string]string{"Authorization": basicAuth("alice", "secret1234")},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	user := body["user"].(map[string]any)
	if user["username"] != "alice" {
		t.Errorf("expected alice, got %v", user["username"])
	}
	if extractCookie(resp, "__token") == nil {
		t.Error("expected __token cookie")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	registerUser(t, ts, "alice", "secret1234", "user")

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/login",
		headers: map[string]string{"Authorization": basicAuth("alice", "wrongpassword")},
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %v", resp.StatusCode, body)
	}
}

func TestLogin_NonExistentUser(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/auth/login",
		headers: map[string]string{"Authorization": basicAuth("ghost", "pass1234")},
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %v", resp.StatusCode, body)
	}
}

func TestLogin_MissingAuthHeader(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: http.MethodPost,
		path:   "/api/auth/login",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, body := doRequest(t, ts, requestOpts{
		method: http.MethodPost,
		path:   "/api/auth/logout",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	cookie := extractCookie(resp, "__token")
	if cookie == nil || cookie.MaxAge >= 0 {
		t.Error("expected __token cookie to be cleared (MaxAge < 0)")
	}
}

func TestGetMe_Authenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	cookie, _ := registerUser(t, ts, "alice", "secret1234", "user")

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/auth/me",
		cookies: []*http.Cookie{cookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["username"] != "alice" {
		t.Errorf("expected alice, got %v", body["username"])
	}
	if body["role"] != "user" {
		t.Errorf("expected role user, got %v", body["role"])
	}
}

func TestGetMe_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: http.MethodGet,
		path:   "/api/auth/me",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetMe_InvalidToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: http.MethodGet,
		path:   "/api/auth/me",
		cookies: []*http.Cookie{
			{Name: "__token", Value: "totally-bogus-jwt-token"},
		},
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetMe_BearerToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Register and extract the token from the cookie
	cookie, _ := registerUser(t, ts, "alice", "secret1234", "user")

	// Use Bearer auth header instead of cookie
	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/auth/me",
		headers: map[string]string{"Authorization": "Bearer " + cookie.Value},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["username"] != "alice" {
		t.Errorf("expected alice, got %v", body["username"])
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//
//	SHOP TESTS
//
// ═══════════════════════════════════════════════════════════════════════════════

func TestListShops_Empty(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, arr := doRequestArray(t, ts, requestOpts{
		method: http.MethodGet,
		path:   "/api/shops",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(arr) != 0 {
		t.Errorf("expected 0 shops, got %d", len(arr))
	}
}

func TestCreateShop_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Coffee House","description":"Best coffee in town","rewardDescription":"1 free coffee","stampsRequired":5,"color":"#ef4444"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	if body["name"] != "Coffee House" {
		t.Errorf("expected Coffee House, got %v", body["name"])
	}
	if body["stampsRequired"] != float64(5) {
		t.Errorf("expected 5 stamps, got %v", body["stampsRequired"])
	}
	if body["color"] != "#ef4444" {
		t.Errorf("expected #ef4444, got %v", body["color"])
	}
	if body["id"] == nil || body["id"] == "" {
		t.Error("expected a non-empty shop ID")
	}
}

func TestCreateShop_DefaultStampsAndColor(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Bakery","rewardDescription":"Free bread","stampsRequired":0}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	// stampsRequired < 2 → default 8
	if body["stampsRequired"] != float64(8) {
		t.Errorf("expected default 8 stamps, got %v", body["stampsRequired"])
	}
	// no color → default #6366f1
	if body["color"] != "#6366f1" {
		t.Errorf("expected default #6366f1, got %v", body["color"])
	}
}

func TestCreateShop_StampsRequiredOutOfRange(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	// stampsRequired > 20 → default 8
	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Pizza Place","rewardDescription":"Free pizza","stampsRequired":50}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	if body["stampsRequired"] != float64(8) {
		t.Errorf("expected default 8 for out-of-range, got %v", body["stampsRequired"])
	}
}

func TestCreateShop_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: http.MethodPost,
		path:   "/api/shops",
		body:   `{"name":"No Auth Shop","rewardDescription":"test"}`,
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateShop_UserRole_Forbidden(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	userCookie, _ := registerUser(t, ts, "regularuser", "pass1234", "user")

	resp, _ := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Sneaky Shop","rewardDescription":"test"}`,
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCreateShop_MissingRequiredFields(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	// Missing name
	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"rewardDescription":"Free item"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d: %v", resp.StatusCode, body)
	}

	// Missing rewardDescription
	resp, body = doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop Without Reward"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing reward description, got %d: %v", resp.StatusCode, body)
	}
}

func TestCreateShop_InvalidBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	resp, _ := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{invalid json`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateShop_MultipleShops(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	// Create first shop
	resp1, body1 := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop 1","rewardDescription":"Free item"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp1.StatusCode, body1)
	}

	// Create second shop — should succeed
	resp2, body2 := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop 2","rewardDescription":"Another free item"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp2.StatusCode, body2)
	}

	// Verify both shops appear in /api/shops/mine
	resp3, arr := doRequestArray(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/shops/mine",
		cookies: []*http.Cookie{adminCookie},
	})
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp3.StatusCode)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 shops, got %d", len(arr))
	}
}

func TestUpdateShop_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	_, createBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Old Name","rewardDescription":"Old reward","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := createBody["id"].(string)

	resp, body := doRequest(t, ts, requestOpts{
		method:  "PUT",
		path:    "/api/shops/" + shopID,
		body:    `{"name":"New Name","description":"Updated desc","rewardDescription":"New reward","stampsRequired":10,"color":"#10b981"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["name"] != "New Name" {
		t.Errorf("expected New Name, got %v", body["name"])
	}
	if body["stampsRequired"] != float64(10) {
		t.Errorf("expected 10 stamps, got %v", body["stampsRequired"])
	}
}

func TestUpdateShop_NotOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	admin1Cookie, _ := registerUser(t, ts, "owner1", "admin1234", "admin")
	admin2Cookie, _ := registerUser(t, ts, "owner2", "admin5678", "admin")

	_, createBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Owner1 Shop","rewardDescription":"Free item"}`,
		cookies: []*http.Cookie{admin1Cookie},
	})
	shopID := createBody["id"].(string)

	// admin2 tries to update admin1's shop
	resp, body := doRequest(t, ts, requestOpts{
		method:  "PUT",
		path:    "/api/shops/" + shopID,
		body:    `{"name":"Hijacked","rewardDescription":"Stolen reward"}`,
		cookies: []*http.Cookie{admin2Cookie},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %v", resp.StatusCode, body)
	}
}

func TestUpdateShop_NonExistent(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	resp, body := doRequest(t, ts, requestOpts{
		method:  "PUT",
		path:    "/api/shops/non-existent-id",
		body:    `{"name":"Ghost Shop","rewardDescription":"Boo"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %v", resp.StatusCode, body)
	}
}

func TestGetMyShops_NoShop(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "newadmin", "admin1234", "admin")

	resp, arr := doRequestArray(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/shops/mine",
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(arr) != 0 {
		t.Fatalf("expected empty array, got %d items", len(arr))
	}
}

func TestGetMyShops_WithShop(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"My Shop","rewardDescription":"Free thing"}`,
		cookies: []*http.Cookie{adminCookie},
	})

	resp, arr := doRequestArray(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/shops/mine",
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 shop, got %d", len(arr))
	}
	if arr[0]["name"] != "My Shop" {
		t.Errorf("expected My Shop, got %v", arr[0]["name"])
	}
}

func TestListShops_AfterCreation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Public Shop","rewardDescription":"Free item"}`,
		cookies: []*http.Cookie{adminCookie},
	})

	resp, arr := doRequestArray(t, ts, requestOpts{
		method: http.MethodGet,
		path:   "/api/shops",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 shop, got %d", len(arr))
	}
	if arr[0]["name"] != "Public Shop" {
		t.Errorf("expected Public Shop, got %v", arr[0]["name"])
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//
//	STAMPS & CARDS TESTS
//
// ═══════════════════════════════════════════════════════════════════════════════

func TestGrantStamp_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, adminBody := registerUser(t, ts, "shopowner", "admin1234", "admin")
	_, userBody := registerUser(t, ts, "customer", "cust1234", "user")

	adminUser := adminBody["user"].(map[string]any)
	_ = adminUser
	customerUser := userBody["user"].(map[string]any)
	customerID := customerUser["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Stamp Shop","rewardDescription":"Free item","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant first stamp
	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["stamps"] != float64(1) {
		t.Errorf("expected 1 stamp, got %v", body["stamps"])
	}
	if body["redeemed"] != false {
		t.Error("expected not redeemed")
	}
}

func TestGrantStamp_MultipleStamps(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")
	_, userBody := registerUser(t, ts, "customer", "cust1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant 3 stamps
	var lastBody map[string]any
	for i := 0; i < 3; i++ {
		_, lastBody = doRequest(t, ts, requestOpts{
			method:  http.MethodPost,
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
			cookies: []*http.Cookie{adminCookie},
		})
	}
	if lastBody["stamps"] != float64(3) {
		t.Errorf("expected 3 stamps, got %v", lastBody["stamps"])
	}
}

func TestGrantStamp_CannotExceedMax(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")
	_, userBody := registerUser(t, ts, "customer", "cust1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":2}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant stamps beyond max
	var lastBody map[string]any
	for i := 0; i < 5; i++ {
		_, lastBody = doRequest(t, ts, requestOpts{
			method:  http.MethodPost,
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
			cookies: []*http.Cookie{adminCookie},
		})
	}
	// Should cap at stampsRequired
	if lastBody["stamps"] != float64(2) {
		t.Errorf("expected stamps capped at 2, got %v", lastBody["stamps"])
	}
}

func TestGrantStamp_NotOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	admin1Cookie, _ := registerUser(t, ts, "owner1", "admin1234", "admin")
	admin2Cookie, _ := registerUser(t, ts, "owner2", "admin5678", "admin")
	_, userBody := registerUser(t, ts, "customer", "cust1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Owner1 Shop","rewardDescription":"Item"}`,
		cookies: []*http.Cookie{admin1Cookie},
	})
	shopID := shopBody["id"].(string)

	// admin2 tries to grant stamp on admin1's shop
	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
		cookies: []*http.Cookie{admin2Cookie},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %v", resp.StatusCode, body)
	}
}

func TestGrantStamp_ShopNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	resp, _ := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops/fake-shop-id/stamps",
		body:    `{"userId":"some-user"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGrantStamp_InvalidBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	resp, _ := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops/" + shopID + "/stamps",
		body:    `{not json}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetMyCards_NoShops(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	userCookie, _ := registerUser(t, ts, "customer", "cust1234", "user")

	resp, arr := doRequestArray(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/users/me/cards",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(arr) != 0 {
		t.Errorf("expected 0 cards with no shops, got %d", len(arr))
	}
}

func TestGetMyCards_WithShopAndStamps(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")
	userCookie, userBody := registerUser(t, ts, "customer", "cust1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant 2 stamps
	for i := 0; i < 2; i++ {
		doRequest(t, ts, requestOpts{
			method:  http.MethodPost,
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
			cookies: []*http.Cookie{adminCookie},
		})
	}

	resp, arr := doRequestArray(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/users/me/cards",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(arr) == 0 {
		t.Fatal("expected at least 1 card")
	}
	// Find the card for this shop
	found := false
	for _, card := range arr {
		if card["shopId"] == shopID {
			found = true
			if card["stamps"] != float64(2) {
				t.Errorf("expected 2 stamps on card, got %v", card["stamps"])
			}
		}
	}
	if !found {
		t.Error("expected a card for the shop")
	}
}

func TestGetMyCards_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequestArray(t, ts, requestOpts{
		method: http.MethodGet,
		path:   "/api/users/me/cards",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetShopCards(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")
	_, userBody := registerUser(t, ts, "customer", "cust1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant a stamp
	doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
		cookies: []*http.Cookie{adminCookie},
	})

	resp, arr := doRequestArray(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/shops/" + shopID + "/cards",
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(arr) == 0 {
		t.Fatal("expected at least 1 card")
	}
	if arr[0]["stamps"] != float64(1) {
		t.Errorf("expected 1 stamp, got %v", arr[0]["stamps"])
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//
//	REDEEM TESTS
//
// ═══════════════════════════════════════════════════════════════════════════════

func TestRedeemCard_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")
	userCookie, userBody := registerUser(t, ts, "customer", "cust1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":2}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Fill the card
	var lastStampBody map[string]any
	for i := 0; i < 2; i++ {
		_, lastStampBody = doRequest(t, ts, requestOpts{
			method:  http.MethodPost,
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
			cookies: []*http.Cookie{adminCookie},
		})
	}
	cardID := lastStampBody["id"].(string)

	// Redeem
	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/cards/" + cardID + "/redeem",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["status"] != "redeemed" {
		t.Errorf("expected status redeemed, got %v", body["status"])
	}
}

func TestRedeemCard_NotEnoughStamps(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")
	userCookie, userBody := registerUser(t, ts, "customer", "cust1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant only 1 stamp
	_, stampBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
		cookies: []*http.Cookie{adminCookie},
	})
	cardID := stampBody["id"].(string)

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/cards/" + cardID + "/redeem",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %v", resp.StatusCode, body)
	}
}

func TestRedeemCard_NotOwnerOfCard(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")
	_, user1Body := registerUser(t, ts, "customer1", "cust1234", "user")
	user2Cookie, _ := registerUser(t, ts, "customer2", "cust5678", "user")
	customer1ID := user1Body["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":2}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Fill customer1's card
	var stampBody map[string]any
	for i := 0; i < 2; i++ {
		_, stampBody = doRequest(t, ts, requestOpts{
			method:  http.MethodPost,
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customer1ID),
			cookies: []*http.Cookie{adminCookie},
		})
	}
	cardID := stampBody["id"].(string)

	// customer2 tries to redeem customer1's card
	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/cards/" + cardID + "/redeem",
		cookies: []*http.Cookie{user2Cookie},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %v", resp.StatusCode, body)
	}
}

func TestRedeemCard_NonExistent(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	userCookie, _ := registerUser(t, ts, "customer", "cust1234", "user")

	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/cards/non-existent-card-id/redeem",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %v", resp.StatusCode, body)
	}
}

func TestRedeemCard_AlreadyRedeemed(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")
	userCookie, userBody := registerUser(t, ts, "customer", "cust1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":2}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	var stampBody map[string]any
	for i := 0; i < 2; i++ {
		_, stampBody = doRequest(t, ts, requestOpts{
			method:  http.MethodPost,
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
			cookies: []*http.Cookie{adminCookie},
		})
	}
	cardID := stampBody["id"].(string)

	// First redeem — should succeed
	resp, _ := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/cards/" + cardID + "/redeem",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected first redeem 200, got %d", resp.StatusCode)
	}

	// Second redeem — card no longer unredeemed
	resp, body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/cards/" + cardID + "/redeem",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for already-redeemed, got %d: %v", resp.StatusCode, body)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//
//	UPDATE STAMP COUNT TESTS (PATCH /api/shops/{id}/stamps)
//
// ═══════════════════════════════════════════════════════════════════════════════

func TestUpdateStampCount_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")
	_, userBody := registerUser(t, ts, "user", "user1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Stamp Shop","rewardDescription":"Reward","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant 1 stamp first
	doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
		cookies: []*http.Cookie{adminCookie},
	})

	// Update stamp count to 3
	resp, body := doRequest(t, ts, requestOpts{
		method:  "PATCH",
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s","stamps":3}`, customerID),
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["stamps"] != float64(3) {
		t.Errorf("expected 3 stamps, got %v", body["stamps"])
	}
}

func TestUpdateStampCount_CreateCardIfNotExists(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")
	_, userBody := registerUser(t, ts, "user", "user1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"New Shop","rewardDescription":"Reward","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Update stamp count without any prior card
	resp, body := doRequest(t, ts, requestOpts{
		method:  "PATCH",
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s","stamps":2}`, customerID),
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["stamps"] != float64(2) {
		t.Errorf("expected 2 stamps, got %v", body["stamps"])
	}
}

func TestUpdateStampCount_ClampNegativeToZero(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")
	_, userBody := registerUser(t, ts, "user", "user1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	resp, body := doRequest(t, ts, requestOpts{
		method:  "PATCH",
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s","stamps":-5}`, customerID),
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["stamps"] != float64(0) {
		t.Errorf("negative stamps should clamp to 0, got %v", body["stamps"])
	}
}

func TestUpdateStampCount_ClampAboveMax(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")
	_, userBody := registerUser(t, ts, "user", "user1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	resp, body := doRequest(t, ts, requestOpts{
		method:  "PATCH",
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s","stamps":99}`, customerID),
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["stamps"] != float64(3) {
		t.Errorf("stamps should clamp to stampsRequired=3, got %v", body["stamps"])
	}
}

func TestUpdateStampCount_MissingUserId(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")
	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	resp, _ := doRequest(t, ts, requestOpts{
		method:  "PATCH",
		path:    "/api/shops/" + shopID + "/stamps",
		body:    `{"stamps":2}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing userId, got %d", resp.StatusCode)
	}
}

func TestUpdateStampCount_InvalidBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")
	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	resp, _ := doRequest(t, ts, requestOpts{
		method:  "PATCH",
		path:    "/api/shops/" + shopID + "/stamps",
		body:    `{invalid json`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", resp.StatusCode)
	}
}

func TestUpdateStampCount_NotOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	admin1Cookie, _ := registerUser(t, ts, "admin1", "admin1234", "admin")
	admin2Cookie, _ := registerUser(t, ts, "admin2", "admin5678", "admin")
	_, userBody := registerUser(t, ts, "user", "user1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{admin1Cookie},
	})
	shopID := shopBody["id"].(string)

	resp, _ := doRequest(t, ts, requestOpts{
		method:  "PATCH",
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s","stamps":2}`, customerID),
		cookies: []*http.Cookie{admin2Cookie},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestUpdateStampCount_ShopNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")

	resp, _ := doRequest(t, ts, requestOpts{
		method:  "PATCH",
		path:    "/api/shops/nonexistent-id/stamps",
		body:    `{"userId":"someone","stamps":2}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//
//	ADDITIONAL EDGE CASES
//
// ═══════════════════════════════════════════════════════════════════════════════

func TestUpdateShop_DuplicateName(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")

	doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"First Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	_, shop2 := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Second Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shop2ID := shop2["id"].(string)

	// Try to rename shop2 to "First Shop" → should conflict
	resp, _ := doRequest(t, ts, requestOpts{
		method:  "PUT",
		path:    "/api/shops/" + shop2ID,
		body:    `{"name":"First Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate name, got %d", resp.StatusCode)
	}
}

func TestUpdateShop_InvalidBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")
	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	resp, _ := doRequest(t, ts, requestOpts{
		method:  "PUT",
		path:    "/api/shops/" + shopID,
		body:    `{not valid json`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", resp.StatusCode)
	}
}

func TestUpdateShop_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: http.MethodPut,
		path:   "/api/shops/some-id",
		body:   `{"name":"Shop"}`,
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetMyShops_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: http.MethodGet,
		path:   "/api/shops/mine",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetMyShops_UserRole_Forbidden(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	userCookie, _ := registerUser(t, ts, "user", "user1234", "user")

	resp, _ := doRequest(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/shops/mine",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestGetShopCards_Empty(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")
	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Empty Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	_, cards := doRequestArray(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/shops/" + shopID + "/cards",
		cookies: []*http.Cookie{adminCookie},
	})
	if len(cards) != 0 {
		t.Errorf("expected 0 cards for new shop, got %d", len(cards))
	}
}

func TestGetShopCards_MissingShopId(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")

	// Path without actual ID — route should not match or return error
	resp, _ := doRequest(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/shops//cards",
		cookies: []*http.Cookie{adminCookie},
	})
	// An empty shop ID should fail (404 or 400)
	if resp.StatusCode == http.StatusOK {
		t.Error("expected non-200 for empty shop ID")
	}
}

func TestGetMe_WithShopId(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")

	// Create a shop (sets shop_id on user)
	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"My Cafe","rewardDescription":"Free drink","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Update user's shop_id in DB
	if err := db.DB.Exec("UPDATE users SET shop_id = ? WHERE username = 'admin'", shopID).Error; err != nil {
		t.Fatalf("failed to set shop_id: %v", err)
	}

	_, meBody := doRequest(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/auth/me",
		cookies: []*http.Cookie{adminCookie},
	})
	if meBody["shopId"] != shopID {
		t.Errorf("expected shopId=%s, got %v", shopID, meBody["shopId"])
	}
}

func TestGetShopCustomers_OnlyJoinedUsers(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, shopID := createShopHelper(t, ts, "admin")

	// Register two users
	user1Cookie, _ := registerUser(t, ts, "customer1", "cust1234", "user")
	registerUser(t, ts, "customer2", "cust5678", "user")

	// Only customer1 joins the shop
	doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    fmt.Sprintf("/api/shops/%s/join", shopID),
		cookies: []*http.Cookie{user1Cookie},
	})

	_, customers := doRequestArray(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    fmt.Sprintf("/api/shops/%s/customers", shopID),
		cookies: []*http.Cookie{adminCookie},
	})
	if len(customers) != 1 {
		t.Fatalf("expected 1 customer (only joined user), got %d", len(customers))
	}
	if customers[0]["username"] != "customer1" {
		t.Errorf("expected customer1, got %v", customers[0]["username"])
	}
	if customers[0]["role"] != "user" {
		t.Errorf("expected role=user, got %v", customers[0]["role"])
	}
}

func TestCreateStampToken_ReplacesExistingToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, shopID := createShopHelper(t, ts, "tokenadmin")
	userCookie, _ := registerUser(t, ts, "claimuser", "pass1234", "user")

	// Create first token
	_, token1Body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
		cookies: []*http.Cookie{adminCookie},
	})
	token1 := token1Body["token"].(string)

	// Create second token (should invalidate first)
	_, token2Body := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
		cookies: []*http.Cookie{adminCookie},
	})
	token2 := token2Body["token"].(string)

	if token1 == token2 {
		t.Error("new token should differ from old token")
	}

	// Old token should no longer work
	resp1, _ := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token1),
		cookies: []*http.Cookie{userCookie},
	})
	if resp1.StatusCode == http.StatusOK {
		t.Error("old token should be invalidated after new token is created")
	}

	// New token should work
	resp2, body2 := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token2),
		cookies: []*http.Cookie{userCookie},
	})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("new token claim: expected 200, got %d: %v", resp2.StatusCode, body2)
	}
}

func TestGetMyCards_MultipleShops(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")
	userCookie, _ := registerUser(t, ts, "user", "user1234", "user")

	// Create 3 shops and collect their IDs
	var shopIDs []string
	for i := range 3 {
		_, shopBody := doRequest(t, ts, requestOpts{
			method:  http.MethodPost,
			path:    "/api/shops",
			body:    fmt.Sprintf(`{"name":"Shop %d","rewardDescription":"Reward","stampsRequired":3}`, i),
			cookies: []*http.Cookie{adminCookie},
		})
		shopIDs = append(shopIDs, shopBody["id"].(string))
	}

	// User joins all 3 shops
	for _, sid := range shopIDs {
		doRequest(t, ts, requestOpts{
			method:  http.MethodPost,
			path:    "/api/shops/" + sid + "/join",
			cookies: []*http.Cookie{userCookie},
		})
	}

	// User fetches cards — should have one per joined shop
	_, cards := doRequestArray(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/users/me/cards",
		cookies: []*http.Cookie{userCookie},
	})
	if len(cards) < 3 {
		t.Errorf("expected at least 3 cards after joining, got %d", len(cards))
	}
}

func TestListShops_MultipleShops(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	admin1Cookie, _ := registerUser(t, ts, "admin1", "admin1234", "admin")
	admin2Cookie, _ := registerUser(t, ts, "admin2", "admin5678", "admin")

	doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Coffee House","rewardDescription":"Free coffee","stampsRequired":5}`,
		cookies: []*http.Cookie{admin1Cookie},
	})
	doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Bakery","rewardDescription":"Free bread","stampsRequired":3}`,
		cookies: []*http.Cookie{admin2Cookie},
	})

	_, shops := doRequestArray(t, ts, requestOpts{
		method: http.MethodGet,
		path:   "/api/shops",
	})
	if len(shops) != 2 {
		t.Errorf("expected 2 shops, got %d", len(shops))
	}
}

func TestRequestLog_SetsRequestID(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: http.MethodGet,
		path:   "/api/shops",
	})
	reqID := resp.Header.Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected X-Request-ID header from RequestLog middleware")
	}
	if len(reqID) != 8 {
		t.Errorf("expected 8-char request ID, got %q (len=%d)", reqID, len(reqID))
	}
}

func TestDocs_OpenAPIEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: http.MethodGet,
		path:   "/api/docs/openapi.yaml",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for docs endpoint, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/yaml" {
		t.Errorf("expected Content-Type application/yaml, got %q", ct)
	}
}

func TestDocs_ScalarUI(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: http.MethodGet,
		path:   "/api/docs",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for scalar UI, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("expected Content-Type text/html, got %q", ct)
	}
}

func TestGrantStamp_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: http.MethodPost,
		path:   "/api/shops/some-id/stamps",
		body:   `{"userId":"someone"}`,
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGrantStamp_UserRole_Forbidden(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	userCookie, _ := registerUser(t, ts, "user", "user1234", "user")

	resp, _ := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops/some-id/stamps",
		body:    `{"userId":"someone"}`,
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCreateShop_DuplicateName(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")

	doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Unique Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})

	resp, _ := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Unique Shop","rewardDescription":"Different Reward","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate shop name, got %d", resp.StatusCode)
	}
}

func TestRedeemCard_AutoCreatesNewCard(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "admin", "admin1234", "admin")
	userCookie, userBody := registerUser(t, ts, "user", "user1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":2}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant enough stamps
	for range 2 {
		doRequest(t, ts, requestOpts{
			method:  http.MethodPost,
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
			cookies: []*http.Cookie{adminCookie},
		})
	}

	// Get the card
	_, cards := doRequestArray(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/users/me/cards",
		cookies: []*http.Cookie{userCookie},
	})
	var cardID string
	for _, c := range cards {
		if c["shopId"] == shopID && c["stamps"] == float64(2) {
			cardID = c["id"].(string)
		}
	}
	if cardID == "" {
		t.Fatal("expected a completed card")
	}

	// Redeem it
	doRequest(t, ts, requestOpts{
		method:  http.MethodPost,
		path:    "/api/cards/" + cardID + "/redeem",
		cookies: []*http.Cookie{userCookie},
	})

	// Fetch cards again — should have a fresh 0-stamp card
	_, cardsAfter := doRequestArray(t, ts, requestOpts{
		method:  http.MethodGet,
		path:    "/api/users/me/cards",
		cookies: []*http.Cookie{userCookie},
	})
	foundFresh := false
	for _, c := range cardsAfter {
		if c["shopId"] == shopID && c["stamps"] == float64(0) && c["redeemed"] == false {
			foundFresh = true
		}
	}
	if !foundFresh {
		t.Error("expected a fresh 0-stamp card after redeem")
	}
}
