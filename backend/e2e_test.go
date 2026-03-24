package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/middleware"

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
	tmpFile.Close()
	t.Cleanup(func() { db.Close(); os.Remove(tmpFile.Name()) })

	auth.Init("test-secret-key-for-e2e")
	db.Init(tmpFile.Name())

	// Middleware wrappers matching main.go's Auth / AdminOnly chains.
	withAuth := func(fn http.HandlerFunc) http.Handler {
		return middleware.Auth(http.HandlerFunc(fn))
	}
	withAdmin := func(fn http.HandlerFunc) http.Handler {
		return middleware.Auth(middleware.AdminOnly(http.HandlerFunc(fn)))
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

	// Admin routes
	mux.Handle("POST /api/shops", withAdmin(handlers.CreateShop))
	mux.Handle("PUT /api/shops/{id}", withAdmin(handlers.UpdateShop))
	mux.Handle("GET /api/shops/mine", withAdmin(handlers.GetMyShops))
	mux.Handle("GET /api/shops/{id}/cards", withAdmin(handlers.GetShopCards))
	mux.Handle("POST /api/shops/{id}/stamps", withAdmin(handlers.GrantStamp))
	mux.Handle("PATCH /api/shops/{id}/stamps", withAdmin(handlers.UpdateStampCount))
	mux.Handle("POST /api/shops/{id}/stamp-token", withAdmin(handlers.CreateStampToken))
	mux.Handle("GET /api/users/customers", withAdmin(handlers.ListCustomers))

	handler := middleware.CORS(mux)
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
	resp.Body.Close()

	var result map[string]any
	json.Unmarshal(raw, &result) // may fail for arrays; caller handles
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
	resp.Body.Close()

	var result []map[string]any
	json.Unmarshal(raw, &result)
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
		method:  "POST",
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

// ═══════════════════════════════════════════════════════════════════════════════
//
//	AUTH TESTS
//
// ═══════════════════════════════════════════════════════════════════════════════

func TestRegister_HappyPath_User(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
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
		method:  "POST",
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
		method: "POST",
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
		method:  "POST",
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
		method:  "POST",
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
		method:  "POST",
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
		method:  "POST",
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
		method:  "POST",
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
		method:  "POST",
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
		method:  "POST",
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
		method:  "POST",
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
		method:  "POST",
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
		method: "POST",
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
		method: "POST",
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
		method:  "GET",
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
		method: "GET",
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
		method: "GET",
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
		method:  "GET",
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
		method: "GET",
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
		method:  "POST",
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
		method:  "POST",
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
		method:  "POST",
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
		method: "POST",
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
		method:  "POST",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"rewardDescription":"Free item"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d: %v", resp.StatusCode, body)
	}

	// Missing rewardDescription
	resp, body = doRequest(t, ts, requestOpts{
		method:  "POST",
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
		method:  "POST",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Shop 1","rewardDescription":"Free item"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp1.StatusCode, body1)
	}

	// Create second shop — should succeed
	resp2, body2 := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Shop 2","rewardDescription":"Another free item"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp2.StatusCode, body2)
	}

	// Verify both shops appear in /api/shops/mine
	resp3, arr := doRequestArray(t, ts, requestOpts{
		method:  "GET",
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
		method:  "POST",
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
		method:  "POST",
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
		method:  "GET",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"My Shop","rewardDescription":"Free thing"}`,
		cookies: []*http.Cookie{adminCookie},
	})

	resp, arr := doRequestArray(t, ts, requestOpts{
		method:  "GET",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Public Shop","rewardDescription":"Free item"}`,
		cookies: []*http.Cookie{adminCookie},
	})

	resp, arr := doRequestArray(t, ts, requestOpts{
		method: "GET",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Stamp Shop","rewardDescription":"Free item","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant first stamp
	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":3}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant 3 stamps
	var lastBody map[string]any
	for i := 0; i < 3; i++ {
		_, lastBody = doRequest(t, ts, requestOpts{
			method:  "POST",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":2}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant stamps beyond max
	var lastBody map[string]any
	for i := 0; i < 5; i++ {
		_, lastBody = doRequest(t, ts, requestOpts{
			method:  "POST",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Owner1 Shop","rewardDescription":"Item"}`,
		cookies: []*http.Cookie{admin1Cookie},
	})
	shopID := shopBody["id"].(string)

	// admin2 tries to grant stamp on admin1's shop
	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
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
		method:  "POST",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	resp, _ := doRequest(t, ts, requestOpts{
		method:  "POST",
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
		method:  "GET",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant 2 stamps
	for i := 0; i < 2; i++ {
		doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
			cookies: []*http.Cookie{adminCookie},
		})
	}

	resp, arr := doRequestArray(t, ts, requestOpts{
		method:  "GET",
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
		method: "GET",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant a stamp
	doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
		cookies: []*http.Cookie{adminCookie},
	})

	resp, arr := doRequestArray(t, ts, requestOpts{
		method:  "GET",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":2}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Fill the card
	var lastStampBody map[string]any
	for i := 0; i < 2; i++ {
		_, lastStampBody = doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
			cookies: []*http.Cookie{adminCookie},
		})
	}
	cardID := lastStampBody["id"].(string)

	// Redeem
	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":5}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Grant only 1 stamp
	_, stampBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/shops/" + shopID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
		cookies: []*http.Cookie{adminCookie},
	})
	cardID := stampBody["id"].(string)

	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":2}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// Fill customer1's card
	var stampBody map[string]any
	for i := 0; i < 2; i++ {
		_, stampBody = doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customer1ID),
			cookies: []*http.Cookie{adminCookie},
		})
	}
	cardID := stampBody["id"].(string)

	// customer2 tries to redeem customer1's card
	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
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
		method:  "POST",
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
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Shop","rewardDescription":"Reward","stampsRequired":2}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	var stampBody map[string]any
	for i := 0; i < 2; i++ {
		_, stampBody = doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
			cookies: []*http.Cookie{adminCookie},
		})
	}
	cardID := stampBody["id"].(string)

	// First redeem — should succeed
	resp, _ := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/cards/" + cardID + "/redeem",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected first redeem 200, got %d", resp.StatusCode)
	}

	// Second redeem — card no longer unredeemed
	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/cards/" + cardID + "/redeem",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for already-redeemed, got %d: %v", resp.StatusCode, body)
	}
}

func TestRedeemCard_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: "POST",
		path:   "/api/cards/some-card-id/redeem",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//
//	CUSTOMERS TESTS
//
// ═══════════════════════════════════════════════════════════════════════════════

func TestListCustomers_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")
	registerUser(t, ts, "user1", "pass1234", "user")
	registerUser(t, ts, "user2", "pass5678", "user")

	resp, arr := doRequestArray(t, ts, requestOpts{
		method:  "GET",
		path:    "/api/users/customers",
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 customers, got %d", len(arr))
	}
	// Admin should NOT appear in customer list
	for _, c := range arr {
		if c["role"] == "admin" {
			t.Error("admin should not appear in customer list")
		}
	}
}

func TestListCustomers_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequestArray(t, ts, requestOpts{
		method: "GET",
		path:   "/api/users/customers",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListCustomers_UserRole_Forbidden(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	userCookie, _ := registerUser(t, ts, "regularuser", "pass1234", "user")

	resp, _ := doRequestArray(t, ts, requestOpts{
		method:  "GET",
		path:    "/api/users/customers",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestListCustomers_Empty(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "shopowner", "admin1234", "admin")

	resp, arr := doRequestArray(t, ts, requestOpts{
		method:  "GET",
		path:    "/api/users/customers",
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(arr) != 0 {
		t.Errorf("expected 0 customers, got %d", len(arr))
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//
//	CORS TESTS
//
// ═══════════════════════════════════════════════════════════════════════════════

func TestCORS_PreflightRequest(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/shops", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if v := resp.Header.Get("Access-Control-Allow-Origin"); v != "http://localhost:5173" {
		t.Errorf("expected origin http://localhost:5173, got %v", v)
	}
	if v := resp.Header.Get("Access-Control-Allow-Credentials"); v != "true" {
		t.Errorf("expected credentials true, got %v", v)
	}
}

func TestCORS_DefaultOrigin(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// No Origin header → falls back to http://localhost:5173
	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/shops", nil)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	if v := resp.Header.Get("Access-Control-Allow-Origin"); v != "http://localhost:5173" {
		t.Errorf("expected default origin, got %v", v)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//
//	FULL E2E SCENARIO: COMPLETE USER JOURNEY
//
// ═══════════════════════════════════════════════════════════════════════════════

func TestE2E_FullUserJourney(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// 1. Admin registers and creates a shop
	adminCookie, _ := registerUser(t, ts, "coffeeadmin", "admin1234", "admin")

	_, shopBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Java Beans","description":"Premium coffee","rewardDescription":"1 free latte","stampsRequired":3,"color":"#6366f1"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)

	// 2. Verify shop appears in public listing
	_, shopList := doRequestArray(t, ts, requestOpts{
		method: "GET",
		path:   "/api/shops",
	})
	if len(shopList) != 1 {
		t.Fatalf("expected 1 shop in listing, got %d", len(shopList))
	}

	// 3. User registers
	userCookie, userBody := registerUser(t, ts, "coffeeuser", "user1234", "user")
	customerID := userBody["user"].(map[string]any)["id"].(string)

	// 4. User checks their profile
	_, meBody := doRequest(t, ts, requestOpts{
		method:  "GET",
		path:    "/api/auth/me",
		cookies: []*http.Cookie{userCookie},
	})
	if meBody["username"] != "coffeeuser" {
		t.Errorf("expected coffeeuser, got %v", meBody["username"])
	}

	// 5. User views their cards (should auto-create card for the shop)
	_, cardsArr := doRequestArray(t, ts, requestOpts{
		method:  "GET",
		path:    "/api/users/me/cards",
		cookies: []*http.Cookie{userCookie},
	})
	if len(cardsArr) == 0 {
		t.Fatal("expected at least 1 auto-created card")
	}

	// 6. Admin sees customer in customer list
	_, custArr := doRequestArray(t, ts, requestOpts{
		method:  "GET",
		path:    "/api/users/customers",
		cookies: []*http.Cookie{adminCookie},
	})
	found := false
	for _, c := range custArr {
		if c["id"] == customerID {
			found = true
		}
	}
	if !found {
		t.Error("expected customer to appear in admin's customer list")
	}

	// 7. Admin grants 3 stamps (card complete)
	var lastStamp map[string]any
	for i := 0; i < 3; i++ {
		_, lastStamp = doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    "/api/shops/" + shopID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, customerID),
			cookies: []*http.Cookie{adminCookie},
		})
	}
	if lastStamp["stamps"] != float64(3) {
		t.Errorf("expected 3 stamps, got %v", lastStamp["stamps"])
	}
	cardID := lastStamp["id"].(string)

	// 8. User redeems the completed card
	resp, redeemBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/cards/" + cardID + "/redeem",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("redeem failed: %d %v", resp.StatusCode, redeemBody)
	}
	if redeemBody["status"] != "redeemed" {
		t.Errorf("expected redeemed, got %v", redeemBody["status"])
	}

	// 9. User cannot redeem the same card again
	resp, _ = doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/cards/" + cardID + "/redeem",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode == http.StatusOK {
		t.Error("should not be able to redeem same card twice")
	}

	// 10. Admin updates the shop
	resp, updatedShop := doRequest(t, ts, requestOpts{
		method:  "PUT",
		path:    "/api/shops/" + shopID,
		body:    `{"name":"Java Beans Premium","description":"Updated desc","rewardDescription":"2 free lattes","stampsRequired":5,"color":"#ef4444"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update failed: %d %v", resp.StatusCode, updatedShop)
	}
	if updatedShop["name"] != "Java Beans Premium" {
		t.Errorf("expected updated name, got %v", updatedShop["name"])
	}

	// 11. Admin checks shop cards
	_, shopCardsArr := doRequestArray(t, ts, requestOpts{
		method:  "GET",
		path:    "/api/shops/" + shopID + "/cards",
		cookies: []*http.Cookie{adminCookie},
	})
	if len(shopCardsArr) == 0 {
		t.Error("expected shop cards to be returned")
	}

	// 12. User logs out and logs back in
	doRequest(t, ts, requestOpts{
		method: "POST",
		path:   "/api/auth/logout",
	})

	loginResp, loginBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/auth/login",
		headers: map[string]string{"Authorization": basicAuth("coffeeuser", "user1234")},
	})
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: %d %v", loginResp.StatusCode, loginBody)
	}
	newCookie := extractCookie(loginResp, "__token")
	if newCookie == nil {
		t.Fatal("expected new cookie after login")
	}

	// 13. Verify user still has their data
	_, meBodyAfter := doRequest(t, ts, requestOpts{
		method:  "GET",
		path:    "/api/auth/me",
		cookies: []*http.Cookie{newCookie},
	})
	if meBodyAfter["username"] != "coffeeuser" {
		t.Errorf("expected coffeeuser after re-login, got %v", meBodyAfter["username"])
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//
//	MULTI-SHOP / MULTI-USER SCENARIO
//
// ═══════════════════════════════════════════════════════════════════════════════

func TestE2E_MultipleShopsMultipleUsers(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create 2 admins with shops
	admin1Cookie, _ := registerUser(t, ts, "admin1", "admin1234", "admin")
	admin2Cookie, _ := registerUser(t, ts, "admin2", "admin5678", "admin")

	_, shop1Body := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Coffee Place","rewardDescription":"Free coffee","stampsRequired":3}`,
		cookies: []*http.Cookie{admin1Cookie},
	})
	shop1ID := shop1Body["id"].(string)

	_, shop2Body := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"Bakery","rewardDescription":"Free bread","stampsRequired":2}`,
		cookies: []*http.Cookie{admin2Cookie},
	})
	shop2ID := shop2Body["id"].(string)

	// Create 2 users
	user1Cookie, user1Body := registerUser(t, ts, "user1", "user1234", "user")
	user2Cookie, user2Body := registerUser(t, ts, "user2", "user5678", "user")
	user1ID := user1Body["user"].(map[string]any)["id"].(string)
	user2ID := user2Body["user"].(map[string]any)["id"].(string)

	// Admin1 grants stamps to both users at shop1
	for i := 0; i < 3; i++ {
		doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    "/api/shops/" + shop1ID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, user1ID),
			cookies: []*http.Cookie{admin1Cookie},
		})
	}
	doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/shops/" + shop1ID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s"}`, user2ID),
		cookies: []*http.Cookie{admin1Cookie},
	})

	// Admin2 grants stamps to user1 at shop2
	for i := 0; i < 2; i++ {
		doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    "/api/shops/" + shop2ID + "/stamps",
			body:    fmt.Sprintf(`{"userId":"%s"}`, user1ID),
			cookies: []*http.Cookie{admin2Cookie},
		})
	}

	// Verify public shop listing has 2 shops
	_, shopList := doRequestArray(t, ts, requestOpts{
		method: "GET",
		path:   "/api/shops",
	})
	if len(shopList) != 2 {
		t.Fatalf("expected 2 shops, got %d", len(shopList))
	}

	// User1 should have cards for both shops
	_, user1Cards := doRequestArray(t, ts, requestOpts{
		method:  "GET",
		path:    "/api/users/me/cards",
		cookies: []*http.Cookie{user1Cookie},
	})
	shop1Card := ""
	shop2Card := ""
	for _, card := range user1Cards {
		if card["shopId"] == shop1ID && card["stamps"] == float64(3) {
			shop1Card = card["id"].(string)
		}
		if card["shopId"] == shop2ID && card["stamps"] == float64(2) {
			shop2Card = card["id"].(string)
		}
	}
	if shop1Card == "" {
		t.Error("user1 should have a complete card at shop1")
	}
	if shop2Card == "" {
		t.Error("user1 should have a complete card at shop2")
	}

	// User1 redeems both cards
	resp1, _ := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/cards/" + shop1Card + "/redeem",
		cookies: []*http.Cookie{user1Cookie},
	})
	if resp1.StatusCode != http.StatusOK {
		t.Errorf("expected shop1 redeem 200, got %d", resp1.StatusCode)
	}
	resp2, _ := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/cards/" + shop2Card + "/redeem",
		cookies: []*http.Cookie{user1Cookie},
	})
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected shop2 redeem 200, got %d", resp2.StatusCode)
	}

	// User2 should have 1 stamp at shop1, cannot redeem
	_, user2Cards := doRequestArray(t, ts, requestOpts{
		method:  "GET",
		path:    "/api/users/me/cards",
		cookies: []*http.Cookie{user2Cookie},
	})
	for _, card := range user2Cards {
		if card["shopId"] == shop1ID && card["stamps"] == float64(1) {
			cardID := card["id"].(string)
			resp, body := doRequest(t, ts, requestOpts{
				method:  "POST",
				path:    "/api/cards/" + cardID + "/redeem",
				cookies: []*http.Cookie{user2Cookie},
			})
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("user2 should not redeem with 1/3 stamps: %d %v", resp.StatusCode, body)
			}
		}
	}

	// Admin1 cannot grant stamps to admin2's shop
	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/shops/" + shop2ID + "/stamps",
		body:    fmt.Sprintf(`{"userId":"%s"}`, user1ID),
		cookies: []*http.Cookie{admin1Cookie},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 cross-shop, got %d: %v", resp.StatusCode, body)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//
//	QR STAMP TOKEN TESTS
//
// ═══════════════════════════════════════════════════════════════════════════════

// createShopHelper registers an admin, creates a shop, and returns the cookie + shopID.
func createShopHelper(t *testing.T, ts *httptest.Server, username string) (*http.Cookie, string) {
	t.Helper()
	adminCookie, _ := registerUser(t, ts, username, "admin1234", "admin")
	_, shopBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/shops",
		body:    `{"name":"QR Test Shop","rewardDescription":"Free item","stampsRequired":3,"color":"#6366f1"}`,
		cookies: []*http.Cookie{adminCookie},
	})
	shopID := shopBody["id"].(string)
	return adminCookie, shopID
}

func TestCreateStampToken_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, shopID := createShopHelper(t, ts, "tokenadmin")

	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	if body["token"] == nil || body["token"] == "" {
		t.Error("expected non-empty token")
	}
	if body["expiresAt"] == nil || body["expiresAt"] == "" {
		t.Error("expected non-empty expiresAt")
	}
	if body["shopId"] != shopID {
		t.Errorf("expected shopId %s, got %v", shopID, body["shopId"])
	}
}

func TestCreateStampToken_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: "POST",
		path:   "/api/shops/nonexistent/stamp-token",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateStampToken_UserRole_Forbidden(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	userCookie, _ := registerUser(t, ts, "regularuser", "pass1234", "user")

	resp, _ := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/shops/someid/stamp-token",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCreateStampToken_NotOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	_, shopID := createShopHelper(t, ts, "owner1")
	otherAdmin, _ := registerUser(t, ts, "owner2", "admin1234", "admin")

	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
		cookies: []*http.Cookie{otherAdmin},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %v", resp.StatusCode, body)
	}
}

func TestCreateStampToken_ShopNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, _ := registerUser(t, ts, "tokenadmin", "admin1234", "admin")

	resp, _ := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/shops/nonexistent-id/stamp-token",
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestClaimStamp_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, shopID := createShopHelper(t, ts, "claimadmin")
	userCookie, _ := registerUser(t, ts, "claimuser", "pass1234", "user")

	// Admin creates a token
	_, tokenBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
		cookies: []*http.Cookie{adminCookie},
	})
	token := tokenBody["token"].(string)

	// User claims it
	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token),
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["shopName"] != "QR Test Shop" {
		t.Errorf("expected shop name 'QR Test Shop', got %v", body["shopName"])
	}
	if body["stamps"] != float64(1) {
		t.Errorf("expected 1 stamp, got %v", body["stamps"])
	}
	if body["stampsRequired"] != float64(3) {
		t.Errorf("expected 3 stampsRequired, got %v", body["stampsRequired"])
	}
	msg := body["message"].(string)
	if !strings.Contains(msg, "Stamp collected") {
		t.Errorf("expected success message, got %q", msg)
	}
}

func TestClaimStamp_DoubleScan_SameUser(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, shopID := createShopHelper(t, ts, "dsadmin")
	userCookie, _ := registerUser(t, ts, "dsuser", "pass1234", "user")

	// Create token
	_, tokenBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
		cookies: []*http.Cookie{adminCookie},
	})
	token := tokenBody["token"].(string)

	// First claim — should succeed
	resp1, body1 := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token),
		cookies: []*http.Cookie{userCookie},
	})
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first claim: expected 200, got %d: %v", resp1.StatusCode, body1)
	}
	if body1["stamps"] != float64(1) {
		t.Errorf("first claim: expected 1 stamp, got %v", body1["stamps"])
	}

	// Second claim — same user, same token — should return friendly message, NOT grant second stamp
	resp2, body2 := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token),
		cookies: []*http.Cookie{userCookie},
	})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second claim: expected 200, got %d: %v", resp2.StatusCode, body2)
	}
	if body2["stamps"] != float64(1) {
		t.Errorf("second claim: stamps should still be 1, got %v", body2["stamps"])
	}
	msg := body2["message"].(string)
	if !strings.Contains(msg, "already scanned") {
		t.Errorf("expected 'already scanned' message, got %q", msg)
	}
}

func TestClaimStamp_MultipleUsers_SameToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, shopID := createShopHelper(t, ts, "muadmin")
	user1Cookie, _ := registerUser(t, ts, "muuser1", "pass1234", "user")
	user2Cookie, _ := registerUser(t, ts, "muuser2", "pass1234", "user")

	// Create ONE token
	_, tokenBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
		cookies: []*http.Cookie{adminCookie},
	})
	token := tokenBody["token"].(string)

	// User 1 claims
	resp1, body1 := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token),
		cookies: []*http.Cookie{user1Cookie},
	})
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("user1 claim: expected 200, got %d", resp1.StatusCode)
	}
	if body1["stamps"] != float64(1) {
		t.Errorf("user1: expected 1 stamp, got %v", body1["stamps"])
	}

	// User 2 claims the SAME token — should also succeed
	resp2, body2 := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token),
		cookies: []*http.Cookie{user2Cookie},
	})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("user2 claim: expected 200, got %d", resp2.StatusCode)
	}
	if body2["stamps"] != float64(1) {
		t.Errorf("user2: expected 1 stamp, got %v", body2["stamps"])
	}
}

func TestClaimStamp_InvalidToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	userCookie, _ := registerUser(t, ts, "invtokenuser", "pass1234", "user")

	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    `{"token":"totally-bogus-token"}`,
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %v", resp.StatusCode, body)
	}
}

func TestClaimStamp_EmptyToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	userCookie, _ := registerUser(t, ts, "emptytkuser", "pass1234", "user")

	resp, _ := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    `{"token":""}`,
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestClaimStamp_NoBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	userCookie, _ := registerUser(t, ts, "nobody", "pass1234", "user")

	resp, _ := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestClaimStamp_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, _ := doRequest(t, ts, requestOpts{
		method: "POST",
		path:   "/api/stamps/claim",
		body:   `{"token":"abc"}`,
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestClaimStamp_AdminRole_Forbidden(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, shopID := createShopHelper(t, ts, "claimadminrole")

	// Create token
	_, tokenBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
		cookies: []*http.Cookie{adminCookie},
	})
	token := tokenBody["token"].(string)

	// Admin tries to claim — should be forbidden
	resp, _ := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token),
		cookies: []*http.Cookie{adminCookie},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestClaimStamp_ExpiredToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, shopID := createShopHelper(t, ts, "expadmin")
	userCookie, _ := registerUser(t, ts, "expuser", "pass1234", "user")

	// Insert an already-expired token directly into the DB
	expiredTime := time.Now().UTC().Add(-10 * time.Second).Format(time.RFC3339)
	db.DB.Exec(
		"INSERT INTO stamp_tokens (id, shop_id, token, expires_at) VALUES (?, ?, ?, ?)",
		"expired-id", shopID, "expired-token-value", expiredTime,
	)
	_ = adminCookie

	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    `{"token":"expired-token-value"}`,
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("expected 410 Gone, got %d: %v", resp.StatusCode, body)
	}
}

func TestClaimStamp_CardFull(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, shopID := createShopHelper(t, ts, "fulladmin")
	userCookie, _ := registerUser(t, ts, "fulluser", "pass1234", "user")

	// Grant 3 stamps via QR (stampsRequired=3), filling the card
	for i := 0; i < 3; i++ {
		_, tokenBody := doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
			cookies: []*http.Cookie{adminCookie},
		})
		token := tokenBody["token"].(string)
		resp, body := doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    "/api/stamps/claim",
			body:    fmt.Sprintf(`{"token":"%s"}`, token),
			cookies: []*http.Cookie{userCookie},
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("stamp %d: expected 200, got %d: %v", i+1, resp.StatusCode, body)
		}
	}

	// Try one more — card is full
	_, tokenBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
		cookies: []*http.Cookie{adminCookie},
	})
	token := tokenBody["token"].(string)
	resp, body := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token),
		cookies: []*http.Cookie{userCookie},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	msg := body["message"].(string)
	if !strings.Contains(msg, "already full") {
		t.Errorf("expected 'already full' message, got %q", msg)
	}
	if body["stamps"] != float64(3) {
		t.Errorf("stamps should still be 3, got %v", body["stamps"])
	}
}

func TestClaimStamp_CardComplete_Message(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	adminCookie, shopID := createShopHelper(t, ts, "compadmin")
	userCookie, _ := registerUser(t, ts, "compuser", "pass1234", "user")

	// Stamps required = 3. Grant 2 via QR, then the 3rd should trigger "complete"
	for i := 0; i < 2; i++ {
		_, tokenBody := doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
			cookies: []*http.Cookie{adminCookie},
		})
		doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    "/api/stamps/claim",
			body:    fmt.Sprintf(`{"token":"%s"}`, tokenBody["token"].(string)),
			cookies: []*http.Cookie{userCookie},
		})
	}

	// 3rd stamp → completes the card
	_, tokenBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
		cookies: []*http.Cookie{adminCookie},
	})
	_, body := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, tokenBody["token"].(string)),
		cookies: []*http.Cookie{userCookie},
	})
	if body["stamps"] != float64(3) {
		t.Errorf("expected 3 stamps, got %v", body["stamps"])
	}
	msg := body["message"].(string)
	if !strings.Contains(msg, "Card complete") {
		t.Errorf("expected completion message, got %q", msg)
	}
}

func TestE2E_QRStampFullJourney(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// 1. Admin creates shop (stampsRequired = 3)
	adminCookie, shopID := createShopHelper(t, ts, "journeyadmin")

	// 2. Two users register
	user1Cookie, _ := registerUser(t, ts, "journeyuser1", "pass1234", "user")
	user2Cookie, _ := registerUser(t, ts, "journeyuser2", "pass1234", "user")

	// 3. Admin generates a token
	_, tokenBody := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
		cookies: []*http.Cookie{adminCookie},
	})
	token := tokenBody["token"].(string)

	// 4. Both users claim the same token
	_, body1 := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token),
		cookies: []*http.Cookie{user1Cookie},
	})
	if body1["stamps"] != float64(1) {
		t.Errorf("user1: expected 1 stamp, got %v", body1["stamps"])
	}

	_, body2 := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token),
		cookies: []*http.Cookie{user2Cookie},
	})
	if body2["stamps"] != float64(1) {
		t.Errorf("user2: expected 1 stamp, got %v", body2["stamps"])
	}

	// 5. User1 double-scans — should not get a 2nd stamp
	_, body1dup := doRequest(t, ts, requestOpts{
		method:  "POST",
		path:    "/api/stamps/claim",
		body:    fmt.Sprintf(`{"token":"%s"}`, token),
		cookies: []*http.Cookie{user1Cookie},
	})
	if body1dup["stamps"] != float64(1) {
		t.Errorf("user1 double-scan: expected 1 stamp, got %v", body1dup["stamps"])
	}

	// 6. Admin generates 2 more tokens, user1 fills card
	for i := 0; i < 2; i++ {
		_, tb := doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    fmt.Sprintf("/api/shops/%s/stamp-token", shopID),
			cookies: []*http.Cookie{adminCookie},
		})
		doRequest(t, ts, requestOpts{
			method:  "POST",
			path:    "/api/stamps/claim",
			body:    fmt.Sprintf(`{"token":"%s"}`, tb["token"].(string)),
			cookies: []*http.Cookie{user1Cookie},
		})
	}

	// 7. Verify user1's cards show 3 stamps
	_, cards := doRequestArray(t, ts, requestOpts{
		method:  "GET",
		path:    "/api/users/me/cards",
		cookies: []*http.Cookie{user1Cookie},
	})
	found := false
	for _, c := range cards {
		if c["shopId"] == shopID && c["stamps"] == float64(3) {
			found = true
		}
	}
	if !found {
		t.Errorf("user1 should have a card with 3 stamps for shop %s", shopID)
	}

	// 8. User1 redeems the card
	for _, c := range cards {
		if c["shopId"] == shopID && c["stamps"] == float64(3) {
			resp, _ := doRequest(t, ts, requestOpts{
				method:  "POST",
				path:    fmt.Sprintf("/api/cards/%s/redeem", c["id"].(string)),
				cookies: []*http.Cookie{user1Cookie},
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("redeem: expected 200, got %d", resp.StatusCode)
			}
		}
	}
}
