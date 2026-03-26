package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/docs"
	"land-of-stamp-backend/gen/pb"
	"land-of-stamp-backend/gen/pb/pbconnect"
	"land-of-stamp-backend/interceptor"
	"land-of-stamp-backend/middleware"
	"land-of-stamp-backend/service"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	t.Cleanup(func() { db.Close(context.Background()) })

	auth.Init("test-secret-key-for-e2e")
	db.Init(context.Background(), ":memory:")

	mux := http.NewServeMux()
	opts := connect.WithInterceptors(interceptor.NewAuthInterceptor())
	p, h := pbconnect.NewAuthServiceHandler(&service.AuthService{}, opts)
	mux.Handle(p, h)
	p, h = pbconnect.NewShopServiceHandler(&service.ShopService{}, opts)
	mux.Handle(p, h)
	p, h = pbconnect.NewStampServiceHandler(&service.StampService{}, opts)
	mux.Handle(p, h)
	p, h = pbconnect.NewDocsServiceHandler(&docs.Service{}, opts)
	mux.Handle(p, h)
	return httptest.NewServer(middleware.RequestLog(middleware.CORS(mux)))
}

type clients struct {
	auth  pbconnect.AuthServiceClient
	shop  pbconnect.ShopServiceClient
	stamp pbconnect.StampServiceClient
	docs  pbconnect.DocsServiceClient
}

func newClients(url string) clients {
	return clients{
		auth:  pbconnect.NewAuthServiceClient(http.DefaultClient, url),
		shop:  pbconnect.NewShopServiceClient(http.DefaultClient, url),
		stamp: pbconnect.NewStampServiceClient(http.DefaultClient, url),
		docs:  pbconnect.NewDocsServiceClient(http.DefaultClient, url),
	}
}

// ck creates a connect.Request with the given cookie attached.
func ck[T any](msg *T, c *http.Cookie) *connect.Request[T] {
	r := connect.NewRequest(msg)
	if c != nil {
		r.Header().Set("Cookie", c.Name+"="+c.Value)
	}
	return r
}

// tokenCookie extracts the __token cookie from response headers.
func tokenCookie(h http.Header) *http.Cookie {
	for _, c := range (&http.Response{Header: h}).Cookies() {
		if c.Name == "__token" {
			return c
		}
	}
	return nil
}

func wantCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	if connect.CodeOf(err) != code {
		t.Fatalf("expected %v, got %v (%v)", code, connect.CodeOf(err), err)
	}
}

func noErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// uuidFromName generates a deterministic UUID v5 from a name (for reproducible tests).
func uuidFromName(name string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("land-of-stamp-test:"+name))
}

// regUser creates a user directly in the DB and returns an auth cookie + user proto.
// Since password-based registration has been removed, test setup inserts users directly.
func regUser(t *testing.T, c clients, user, pass, role string) (*http.Cookie, *pb.User) {
	t.Helper()
	u := db.User{
		UUID:     uuidFromName(user),
		Username: user,
		Role:     role,
	}
	if err := db.DB.Create(&u).Error; err != nil {
		t.Fatalf("regUser %s: create: %v", user, err)
	}
	token, err := auth.GenerateToken(u.UUID.String(), user, role)
	if err != nil {
		t.Fatalf("regUser %s: token: %v", user, err)
	}
	cookie := &http.Cookie{Name: "__token", Value: token}
	return cookie, &pb.User{Id: u.UUID.String(), Username: user, Role: role}
}

// mkShop registers an admin, creates a shop with stampsRequired=3, returns cookie + shop ID.
func mkShop(t *testing.T, c clients, admin string) (*http.Cookie, string) {
	t.Helper()
	tk, _ := regUser(t, c, admin, "admin1234", "admin")
	resp, err := c.shop.CreateShop(context.Background(), ck(&pb.CreateShopRequest{
		Name: admin + " Shop", RewardDescription: "Free item", StampsRequired: 3,
	}, tk))
	noErr(t, err)
	return tk, resp.Msg.Id
}

// ctx is a shorthand for context.Background().
var ctx = context.Background()

// ═══════════════════════════════════════════════════════════════════════════════
//  AUTH TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestLogout(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	resp, err := c.auth.Logout(ctx, connect.NewRequest(&pb.LogoutRequest{}))
	noErr(t, err)
	if resp.Msg.Status != "logged out" {
		t.Errorf("expected 'logged out', got %v", resp.Msg.Status)
	}
	ck := tokenCookie(resp.Header())
	if ck == nil || ck.MaxAge >= 0 {
		t.Error("expected __token cookie cleared (MaxAge < 0)")
	}
}

func TestGetMe_Authenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "alice", "secret1234", "user")

	resp, err := c.auth.GetMe(ctx, ck(&pb.GetMeRequest{}, tk))
	noErr(t, err)
	if resp.Msg.Username != "alice" {
		t.Errorf("expected alice, got %v", resp.Msg.Username)
	}
	if resp.Msg.Role != "user" {
		t.Errorf("expected role user, got %v", resp.Msg.Role)
	}
}

func TestGetMe_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	_, err := c.auth.GetMe(ctx, connect.NewRequest(&pb.GetMeRequest{}))
	wantCode(t, err, connect.CodeUnauthenticated)
}

func TestGetMe_InvalidToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	_, err := c.auth.GetMe(ctx, ck(&pb.GetMeRequest{}, &http.Cookie{Name: "__token", Value: "totally-bogus-jwt"}))
	wantCode(t, err, connect.CodeUnauthenticated)
}

func TestGetMe_BearerToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "alice", "secret1234", "user")

	req := connect.NewRequest(&pb.GetMeRequest{})
	req.Header().Set("Authorization", "Bearer "+tk.Value)
	resp, err := c.auth.GetMe(ctx, req)
	noErr(t, err)
	if resp.Msg.Username != "alice" {
		t.Errorf("expected alice, got %v", resp.Msg.Username)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//  SHOP TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestListShops_Empty(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	resp, err := c.shop.ListShops(ctx, connect.NewRequest(&pb.ListShopsRequest{}))
	noErr(t, err)
	if len(resp.Msg.Shops) != 0 {
		t.Errorf("expected 0 shops, got %d", len(resp.Msg.Shops))
	}
}

func TestCreateShop_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "shopowner", "admin1234", "admin")

	resp, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Coffee House", Description: "Best coffee in town",
		RewardDescription: "1 free coffee", StampsRequired: 5, Color: "#ef4444",
	}, tk))
	noErr(t, err)
	if resp.Msg.Name != "Coffee House" {
		t.Errorf("expected Coffee House, got %v", resp.Msg.Name)
	}
	if resp.Msg.StampsRequired != 5 {
		t.Errorf("expected 5 stamps, got %v", resp.Msg.StampsRequired)
	}
	if resp.Msg.Color != "#ef4444" {
		t.Errorf("expected #ef4444, got %v", resp.Msg.Color)
	}
	if resp.Msg.Id == "" {
		t.Error("expected a non-empty shop ID")
	}
}

func TestCreateShop_DefaultStampsAndColor(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "shopowner", "admin1234", "admin")

	resp, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Bakery", RewardDescription: "Free bread", StampsRequired: 0,
	}, tk))
	noErr(t, err)
	if resp.Msg.StampsRequired != 8 {
		t.Errorf("expected default 8 stamps, got %v", resp.Msg.StampsRequired)
	}
	if resp.Msg.Color != "#6366f1" {
		t.Errorf("expected default #6366f1, got %v", resp.Msg.Color)
	}
}

func TestCreateShop_StampsRequiredOutOfRange(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "shopowner", "admin1234", "admin")

	resp, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Pizza Place", RewardDescription: "Free pizza", StampsRequired: 50,
	}, tk))
	noErr(t, err)
	if resp.Msg.StampsRequired != 8 {
		t.Errorf("expected default 8, got %v", resp.Msg.StampsRequired)
	}
}

func TestCreateShop_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	_, err := c.shop.CreateShop(ctx, connect.NewRequest(&pb.CreateShopRequest{
		Name: "No Auth Shop", RewardDescription: "test",
	}))
	wantCode(t, err, connect.CodeUnauthenticated)
}

func TestCreateShop_AnyUserCanCreate(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "regularuser", "pass1234", "user")

	resp, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Sneaky Shop", RewardDescription: "test",
	}, tk))
	noErr(t, err)
	if resp.Msg.Name != "Sneaky Shop" {
		t.Errorf("expected shop name 'Sneaky Shop', got %q", resp.Msg.Name)
	}
}

func TestCreateShop_MissingRequiredFields(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "shopowner", "admin1234", "admin")

	// Missing name
	_, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		RewardDescription: "Free item",
	}, tk))
	wantCode(t, err, connect.CodeInvalidArgument)

	// Missing rewardDescription
	_, err = c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop Without Reward",
	}, tk))
	wantCode(t, err, connect.CodeInvalidArgument)
}

func TestCreateShop_MultipleShops(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "shopowner", "admin1234", "admin")

	_, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop 1", RewardDescription: "Free item",
	}, tk))
	noErr(t, err)
	_, err = c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop 2", RewardDescription: "Another free item",
	}, tk))
	noErr(t, err)

	resp, err := c.shop.GetMyShops(ctx, ck(&pb.GetMyShopsRequest{}, tk))
	noErr(t, err)
	if len(resp.Msg.Shops) != 2 {
		t.Fatalf("expected 2 shops, got %d", len(resp.Msg.Shops))
	}
}

func TestUpdateShop_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	cr, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Old Name", RewardDescription: "Old reward", StampsRequired: 5,
	}, tk))
	noErr(t, err)
	shopID := cr.Msg.Id

	resp, err := c.shop.UpdateShop(ctx, ck(&pb.UpdateShopRequest{
		Id: shopID, Name: "New Name", Description: "Updated desc",
		RewardDescription: "New reward", StampsRequired: 10, Color: "#10b981",
	}, tk))
	noErr(t, err)
	if resp.Msg.Name != "New Name" {
		t.Errorf("expected New Name, got %v", resp.Msg.Name)
	}
	if resp.Msg.StampsRequired != 10 {
		t.Errorf("expected 10, got %v", resp.Msg.StampsRequired)
	}
}

func TestUpdateShop_NotOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk1, _ := regUser(t, c, "owner1", "admin1234", "admin")
	tk2, _ := regUser(t, c, "owner2", "admin5678", "admin")

	cr, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Owner1 Shop", RewardDescription: "Free item",
	}, tk1))
	noErr(t, err)

	_, err = c.shop.UpdateShop(ctx, ck(&pb.UpdateShopRequest{
		Id: cr.Msg.Id, Name: "Hijacked", RewardDescription: "Stolen",
	}, tk2))
	wantCode(t, err, connect.CodePermissionDenied)
}

func TestUpdateShop_NonExistent(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "shopowner", "admin1234", "admin")

	_, err := c.shop.UpdateShop(ctx, ck(&pb.UpdateShopRequest{
		Id: "non-existent-id", Name: "Ghost", RewardDescription: "Boo",
	}, tk))
	wantCode(t, err, connect.CodeNotFound)
}

func TestGetMyShops_NoShop(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "newadmin", "admin1234", "admin")

	resp, err := c.shop.GetMyShops(ctx, ck(&pb.GetMyShopsRequest{}, tk))
	noErr(t, err)
	if len(resp.Msg.Shops) != 0 {
		t.Fatalf("expected 0 shops, got %d", len(resp.Msg.Shops))
	}
}

func TestGetMyShops_WithShop(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	_, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "My Shop", RewardDescription: "Free thing",
	}, tk))
	noErr(t, err)

	resp, err := c.shop.GetMyShops(ctx, ck(&pb.GetMyShopsRequest{}, tk))
	noErr(t, err)
	if len(resp.Msg.Shops) != 1 {
		t.Fatalf("expected 1 shop, got %d", len(resp.Msg.Shops))
	}
	if resp.Msg.Shops[0].Name != "My Shop" {
		t.Errorf("expected My Shop, got %v", resp.Msg.Shops[0].Name)
	}
}

func TestListShops_AfterCreation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	_, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Public Shop", RewardDescription: "Free item",
	}, tk))
	noErr(t, err)

	resp, err := c.shop.ListShops(ctx, connect.NewRequest(&pb.ListShopsRequest{}))
	noErr(t, err)
	if len(resp.Msg.Shops) != 1 {
		t.Fatalf("expected 1 shop, got %d", len(resp.Msg.Shops))
	}
	if resp.Msg.Shops[0].Name != "Public Shop" {
		t.Errorf("expected Public Shop, got %v", resp.Msg.Shops[0].Name)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//  STAMPS & CARDS TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestGrantStamp_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	_, cust := regUser(t, c, "customer", "cust1234", "user")

	cr, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Stamp Shop", RewardDescription: "Free item", StampsRequired: 3,
	}, aTk))
	noErr(t, err)
	shopID := cr.Msg.Id

	resp, err := c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{
		ShopId: shopID, UserId: cust.Id,
	}, aTk))
	noErr(t, err)
	if resp.Msg.Stamps != 1 {
		t.Errorf("expected 1 stamp, got %v", resp.Msg.Stamps)
	}
	if resp.Msg.Redeemed {
		t.Error("expected not redeemed")
	}
}

func TestGrantStamp_MultipleStamps(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	_, cust := regUser(t, c, "customer", "cust1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 3,
	}, aTk))
	shopID := cr.Msg.Id

	var last *pb.StampCard
	for range 3 {
		resp, err := c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: shopID, UserId: cust.Id}, aTk))
		noErr(t, err)
		last = resp.Msg
	}
	if last.Stamps != 3 {
		t.Errorf("expected 3 stamps, got %v", last.Stamps)
	}
}

func TestGrantStamp_CannotExceedMax(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	_, cust := regUser(t, c, "customer", "cust1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 2,
	}, aTk))
	shopID := cr.Msg.Id

	var last *pb.StampCard
	for range 5 {
		resp, _ := c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: shopID, UserId: cust.Id}, aTk))
		last = resp.Msg
	}
	if last.Stamps != 2 {
		t.Errorf("expected stamps capped at 2, got %v", last.Stamps)
	}
}

func TestGrantStamp_NotOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk1, _ := regUser(t, c, "owner1", "admin1234", "admin")
	tk2, _ := regUser(t, c, "owner2", "admin5678", "admin")
	_, cust := regUser(t, c, "customer", "cust1234", "user")

	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Owner1 Shop", RewardDescription: "Item",
	}, tk1))

	_, err := c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: cr.Msg.Id, UserId: cust.Id}, tk2))
	wantCode(t, err, connect.CodePermissionDenied)
}

func TestGrantStamp_ShopNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "shopowner", "admin1234", "admin")

	_, err := c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: "fake-shop-id", UserId: "some-user"}, aTk))
	wantCode(t, err, connect.CodeNotFound)
}

func TestGetMyCards_NoShops(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	uTk, _ := regUser(t, c, "customer", "cust1234", "user")

	resp, err := c.stamp.GetMyCards(ctx, ck(&pb.GetMyCardsRequest{}, uTk))
	noErr(t, err)
	if len(resp.Msg.Cards) != 0 {
		t.Errorf("expected 0 cards, got %d", len(resp.Msg.Cards))
	}
}

func TestGetMyCards_WithShopAndStamps(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	uTk, cust := regUser(t, c, "customer", "cust1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 5,
	}, aTk))
	shopID := cr.Msg.Id

	for range 2 {
		c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: shopID, UserId: cust.Id}, aTk))
	}

	resp, err := c.stamp.GetMyCards(ctx, ck(&pb.GetMyCardsRequest{}, uTk))
	noErr(t, err)
	found := false
	for _, card := range resp.Msg.Cards {
		if card.ShopId == shopID {
			found = true
			if card.Stamps != 2 {
				t.Errorf("expected 2 stamps, got %v", card.Stamps)
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
	c := newClients(ts.URL)

	_, err := c.stamp.GetMyCards(ctx, connect.NewRequest(&pb.GetMyCardsRequest{}))
	wantCode(t, err, connect.CodeUnauthenticated)
}

func TestGetShopCards(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	_, cust := regUser(t, c, "customer", "cust1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 5,
	}, aTk))
	shopID := cr.Msg.Id

	c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: shopID, UserId: cust.Id}, aTk))

	resp, err := c.stamp.GetShopCards(ctx, ck(&pb.GetShopCardsRequest{ShopId: shopID}, aTk))
	noErr(t, err)
	if len(resp.Msg.Cards) == 0 {
		t.Fatal("expected at least 1 card")
	}
	if resp.Msg.Cards[0].Stamps != 1 {
		t.Errorf("expected 1 stamp, got %v", resp.Msg.Cards[0].Stamps)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//  REDEEM TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestRedeemCard_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	uTk, cust := regUser(t, c, "customer", "cust1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 2,
	}, aTk))
	shopID := cr.Msg.Id

	var lastCard *pb.StampCard
	for range 2 {
		resp, _ := c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: shopID, UserId: cust.Id}, aTk))
		lastCard = resp.Msg
	}

	resp, err := c.stamp.RedeemCard(ctx, ck(&pb.RedeemCardRequest{CardId: lastCard.Id}, uTk))
	noErr(t, err)
	if resp.Msg.Status != "redeemed" {
		t.Errorf("expected redeemed, got %v", resp.Msg.Status)
	}
}

func TestRedeemCard_NotEnoughStamps(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	uTk, cust := regUser(t, c, "customer", "cust1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 5,
	}, aTk))

	resp, _ := c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: cr.Msg.Id, UserId: cust.Id}, aTk))

	_, err := c.stamp.RedeemCard(ctx, ck(&pb.RedeemCardRequest{CardId: resp.Msg.Id}, uTk))
	wantCode(t, err, connect.CodeFailedPrecondition)
}

func TestRedeemCard_NotOwnerOfCard(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	_, cust1 := regUser(t, c, "customer1", "cust1234", "user")
	uTk2, _ := regUser(t, c, "customer2", "cust5678", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 2,
	}, aTk))

	var lastCard *pb.StampCard
	for range 2 {
		resp, _ := c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: cr.Msg.Id, UserId: cust1.Id}, aTk))
		lastCard = resp.Msg
	}

	_, err := c.stamp.RedeemCard(ctx, ck(&pb.RedeemCardRequest{CardId: lastCard.Id}, uTk2))
	wantCode(t, err, connect.CodePermissionDenied)
}

func TestRedeemCard_NonExistent(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	uTk, _ := regUser(t, c, "customer", "cust1234", "user")

	_, err := c.stamp.RedeemCard(ctx, ck(&pb.RedeemCardRequest{CardId: "non-existent"}, uTk))
	wantCode(t, err, connect.CodeNotFound)
}

func TestRedeemCard_AlreadyRedeemed(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "shopowner", "admin1234", "admin")
	uTk, cust := regUser(t, c, "customer", "cust1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 2,
	}, aTk))

	var lastCard *pb.StampCard
	for range 2 {
		resp, _ := c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: cr.Msg.Id, UserId: cust.Id}, aTk))
		lastCard = resp.Msg
	}

	_, err := c.stamp.RedeemCard(ctx, ck(&pb.RedeemCardRequest{CardId: lastCard.Id}, uTk))
	noErr(t, err)

	_, err = c.stamp.RedeemCard(ctx, ck(&pb.RedeemCardRequest{CardId: lastCard.Id}, uTk))
	wantCode(t, err, connect.CodeNotFound)
}

// ═══════════════════════════════════════════════════════════════════════════════
//  UPDATE STAMP COUNT TESTS
// ═══════════════════════════════════════════════════════════════════════════════

func TestUpdateStampCount_HappyPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")
	_, cust := regUser(t, c, "user", "user1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Stamp Shop", RewardDescription: "Reward", StampsRequired: 5,
	}, aTk))
	shopID := cr.Msg.Id

	c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: shopID, UserId: cust.Id}, aTk))

	resp, err := c.stamp.UpdateStampCount(ctx, ck(&pb.UpdateStampCountRequest{
		ShopId: shopID, UserId: cust.Id, Stamps: 3,
	}, aTk))
	noErr(t, err)
	if resp.Msg.Stamps != 3 {
		t.Errorf("expected 3 stamps, got %v", resp.Msg.Stamps)
	}
}

func TestUpdateStampCount_CreateCardIfNotExists(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")
	_, cust := regUser(t, c, "user", "user1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "New Shop", RewardDescription: "Reward", StampsRequired: 5,
	}, aTk))

	resp, err := c.stamp.UpdateStampCount(ctx, ck(&pb.UpdateStampCountRequest{
		ShopId: cr.Msg.Id, UserId: cust.Id, Stamps: 2,
	}, aTk))
	noErr(t, err)
	if resp.Msg.Stamps != 2 {
		t.Errorf("expected 2 stamps, got %v", resp.Msg.Stamps)
	}
}

func TestUpdateStampCount_ClampNegativeToZero(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")
	_, cust := regUser(t, c, "user", "user1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 5,
	}, aTk))

	resp, err := c.stamp.UpdateStampCount(ctx, ck(&pb.UpdateStampCountRequest{
		ShopId: cr.Msg.Id, UserId: cust.Id, Stamps: -5,
	}, aTk))
	noErr(t, err)
	if resp.Msg.Stamps != 0 {
		t.Errorf("negative should clamp to 0, got %v", resp.Msg.Stamps)
	}
}

func TestUpdateStampCount_ClampAboveMax(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")
	_, cust := regUser(t, c, "user", "user1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 3,
	}, aTk))

	resp, err := c.stamp.UpdateStampCount(ctx, ck(&pb.UpdateStampCountRequest{
		ShopId: cr.Msg.Id, UserId: cust.Id, Stamps: 99,
	}, aTk))
	noErr(t, err)
	if resp.Msg.Stamps != 3 {
		t.Errorf("should clamp to 3, got %v", resp.Msg.Stamps)
	}
}

func TestUpdateStampCount_MissingUserId(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 3,
	}, aTk))

	_, err := c.stamp.UpdateStampCount(ctx, ck(&pb.UpdateStampCountRequest{
		ShopId: cr.Msg.Id, Stamps: 2,
	}, aTk))
	wantCode(t, err, connect.CodeInvalidArgument)
}

func TestUpdateStampCount_NotOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk1, _ := regUser(t, c, "admin1", "admin1234", "admin")
	tk2, _ := regUser(t, c, "admin2", "admin5678", "admin")
	_, cust := regUser(t, c, "user", "user1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 3,
	}, tk1))

	_, err := c.stamp.UpdateStampCount(ctx, ck(&pb.UpdateStampCountRequest{
		ShopId: cr.Msg.Id, UserId: cust.Id, Stamps: 2,
	}, tk2))
	wantCode(t, err, connect.CodePermissionDenied)
}

func TestUpdateStampCount_ShopNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")

	_, err := c.stamp.UpdateStampCount(ctx, ck(&pb.UpdateStampCountRequest{
		ShopId: "nonexistent-id", UserId: "someone", Stamps: 2,
	}, aTk))
	wantCode(t, err, connect.CodeNotFound)
}

// ═══════════════════════════════════════════════════════════════════════════════
//  ADDITIONAL EDGE CASES
// ═══════════════════════════════════════════════════════════════════════════════

func TestUpdateShop_DuplicateName(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")
	c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "First Shop", RewardDescription: "Reward", StampsRequired: 3,
	}, aTk))
	s2, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Second Shop", RewardDescription: "Reward", StampsRequired: 3,
	}, aTk))

	_, err := c.shop.UpdateShop(ctx, ck(&pb.UpdateShopRequest{
		Id: s2.Msg.Id, Name: "First Shop", RewardDescription: "Reward", StampsRequired: 3,
	}, aTk))
	wantCode(t, err, connect.CodeAlreadyExists)
}

func TestUpdateShop_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	_, err := c.shop.UpdateShop(ctx, connect.NewRequest(&pb.UpdateShopRequest{
		Id: "some-id", Name: "Shop",
	}))
	wantCode(t, err, connect.CodeUnauthenticated)
}

func TestGetMyShops_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	_, err := c.shop.GetMyShops(ctx, connect.NewRequest(&pb.GetMyShopsRequest{}))
	wantCode(t, err, connect.CodeUnauthenticated)
}

func TestGetMyShops_AnyUserCanCall(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	uTk, _ := regUser(t, c, "user", "user1234", "user")

	resp, err := c.shop.GetMyShops(ctx, ck(&pb.GetMyShopsRequest{}, uTk))
	noErr(t, err)
	if len(resp.Msg.Shops) != 0 {
		t.Errorf("expected 0 shops for user with no shops, got %d", len(resp.Msg.Shops))
	}
}

func TestGetShopCards_Empty(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Empty Shop", RewardDescription: "Reward", StampsRequired: 3,
	}, aTk))

	resp, err := c.stamp.GetShopCards(ctx, ck(&pb.GetShopCardsRequest{ShopId: cr.Msg.Id}, aTk))
	noErr(t, err)
	if len(resp.Msg.Cards) != 0 {
		t.Errorf("expected 0 cards, got %d", len(resp.Msg.Cards))
	}
}

func TestGetShopCards_MissingShopId(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")

	_, err := c.stamp.GetShopCards(ctx, ck(&pb.GetShopCardsRequest{}, aTk))
	wantCode(t, err, connect.CodeInvalidArgument)
}

func TestGetShopCustomers_OnlyJoinedUsers(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, shopID := mkShop(t, c, "admin")
	u1Tk, _ := regUser(t, c, "customer1", "cust1234", "user")
	regUser(t, c, "customer2", "cust5678", "user")

	// Only customer1 joins
	c.stamp.JoinShop(ctx, ck(&pb.JoinShopRequest{ShopId: shopID}, u1Tk))

	resp, err := c.stamp.GetShopCustomers(ctx, ck(&pb.GetShopCustomersRequest{ShopId: shopID}, aTk))
	noErr(t, err)
	if len(resp.Msg.Users) != 1 {
		t.Fatalf("expected 1 customer, got %d", len(resp.Msg.Users))
	}
	if resp.Msg.Users[0].Username != "customer1" {
		t.Errorf("expected customer1, got %v", resp.Msg.Users[0].Username)
	}
}

func TestCreateStampToken_ReplacesExistingToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, shopID := mkShop(t, c, "tokenadmin")
	uTk, _ := regUser(t, c, "claimuser", "pass1234", "user")

	t1, err := c.stamp.CreateStampToken(ctx, ck(&pb.CreateStampTokenRequest{ShopId: shopID}, aTk))
	noErr(t, err)
	t2, err := c.stamp.CreateStampToken(ctx, ck(&pb.CreateStampTokenRequest{ShopId: shopID}, aTk))
	noErr(t, err)

	if t1.Msg.Token == t2.Msg.Token {
		t.Error("new token should differ from old")
	}

	// Old token should fail
	_, err = c.stamp.ClaimStamp(ctx, ck(&pb.ClaimStampRequest{Token: t1.Msg.Token}, uTk))
	if err == nil {
		t.Error("old token should be invalidated")
	}

	// New token should work
	_, err = c.stamp.ClaimStamp(ctx, ck(&pb.ClaimStampRequest{Token: t2.Msg.Token}, uTk))
	noErr(t, err)
}

func TestGetMyCards_MultipleShops(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")
	uTk, _ := regUser(t, c, "user", "user1234", "user")

	shopIDs := make([]string, 0, 3)
	for i := range 3 {
		cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
			Name: fmt.Sprintf("Shop %d", i), RewardDescription: "Reward", StampsRequired: 3,
		}, aTk))
		shopIDs = append(shopIDs, cr.Msg.Id)
	}

	for _, sid := range shopIDs {
		c.stamp.JoinShop(ctx, ck(&pb.JoinShopRequest{ShopId: sid}, uTk))
	}

	resp, err := c.stamp.GetMyCards(ctx, ck(&pb.GetMyCardsRequest{}, uTk))
	noErr(t, err)
	if len(resp.Msg.Cards) < 3 {
		t.Errorf("expected at least 3 cards, got %d", len(resp.Msg.Cards))
	}
}

func TestListShops_MultipleShops(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	tk1, _ := regUser(t, c, "admin1", "admin1234", "admin")
	tk2, _ := regUser(t, c, "admin2", "admin5678", "admin")

	c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Coffee House", RewardDescription: "Free coffee", StampsRequired: 5,
	}, tk1))
	c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Bakery", RewardDescription: "Free bread", StampsRequired: 3,
	}, tk2))

	resp, err := c.shop.ListShops(ctx, connect.NewRequest(&pb.ListShopsRequest{}))
	noErr(t, err)
	if len(resp.Msg.Shops) != 2 {
		t.Errorf("expected 2 shops, got %d", len(resp.Msg.Shops))
	}
}

func TestGrantStamp_Unauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	_, err := c.stamp.GrantStamp(ctx, connect.NewRequest(&pb.GrantStampRequest{ShopId: "some-id", UserId: "someone"}))
	wantCode(t, err, connect.CodeUnauthenticated)
}

func TestGrantStamp_UserRole_NonOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	uTk, _ := regUser(t, c, "user", "user1234", "user")

	_, err := c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: "some-id", UserId: "someone"}, uTk))
	wantCode(t, err, connect.CodeNotFound)
}

func TestCreateShop_DuplicateName(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")

	c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Unique Shop", RewardDescription: "Reward", StampsRequired: 3,
	}, aTk))
	_, err := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Unique Shop", RewardDescription: "Different", StampsRequired: 5,
	}, aTk))
	wantCode(t, err, connect.CodeAlreadyExists)
}

func TestRedeemCard_AutoCreatesNewCard(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	aTk, _ := regUser(t, c, "admin", "admin1234", "admin")
	uTk, cust := regUser(t, c, "user", "user1234", "user")
	cr, _ := c.shop.CreateShop(ctx, ck(&pb.CreateShopRequest{
		Name: "Shop", RewardDescription: "Reward", StampsRequired: 2,
	}, aTk))
	shopID := cr.Msg.Id

	for range 2 {
		c.stamp.GrantStamp(ctx, ck(&pb.GrantStampRequest{ShopId: shopID, UserId: cust.Id}, aTk))
	}

	// Find the completed card
	cards, _ := c.stamp.GetMyCards(ctx, ck(&pb.GetMyCardsRequest{}, uTk))
	var cardID string
	for _, card := range cards.Msg.Cards {
		if card.ShopId == shopID && card.Stamps == 2 {
			cardID = card.Id
		}
	}
	if cardID == "" {
		t.Fatal("expected a completed card")
	}

	c.stamp.RedeemCard(ctx, ck(&pb.RedeemCardRequest{CardId: cardID}, uTk))

	// Should have a fresh 0-stamp card
	cardsAfter, _ := c.stamp.GetMyCards(ctx, ck(&pb.GetMyCardsRequest{}, uTk))
	foundFresh := false
	for _, card := range cardsAfter.Msg.Cards {
		if card.ShopId == shopID && card.Stamps == 0 && !card.Redeemed {
			foundFresh = true
		}
	}
	if !foundFresh {
		t.Error("expected a fresh 0-stamp card after redeem")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//  INFRASTRUCTURE TESTS (docs, request log)
// ═══════════════════════════════════════════════════════════════════════════════

func doHTTPGet(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func TestRequestLog_SetsRequestID(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// ListShops is a Connect endpoint that goes through the RequestLog middleware.
	resp := doHTTPGet(t, ts, "/landofstamp.v1.ShopService/ListShops")
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	reqID := resp.Header.Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected X-Request-ID header")
	}
	if len(reqID) != 8 {
		t.Errorf("expected 8-char ID, got %q", reqID)
	}
}

func TestDocs_GetOpenAPISpec(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	resp, err := c.docs.GetOpenAPISpec(context.Background(), connect.NewRequest(&pb.GetOpenAPISpecRequest{}))
	if err != nil {
		t.Fatalf("GetOpenAPISpec: %v", err)
	}
	if resp.Msg.ContentType != "application/yaml" {
		t.Errorf("expected content_type application/yaml, got %q", resp.Msg.ContentType)
	}
	if !strings.Contains(resp.Msg.Content, "openapi:") {
		t.Error("expected spec content to contain 'openapi:' key")
	}
	if !strings.Contains(resp.Msg.Content, "Länd of Stamp") {
		t.Error("expected spec to contain API title")
	}
}

func TestDocs_GetDocsPage(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	c := newClients(ts.URL)

	resp, err := c.docs.GetDocsPage(context.Background(), connect.NewRequest(&pb.GetDocsPageRequest{}))
	if err != nil {
		t.Fatalf("GetDocsPage: %v", err)
	}
	html := resp.Msg.Html
	if !strings.Contains(html, "<!doctype html>") {
		t.Error("expected HTML doctype")
	}
	if !strings.Contains(html, "scalar") {
		t.Error("expected Scalar script reference")
	}
	if !strings.Contains(html, "openapi") {
		t.Error("expected inlined spec content in HTML")
	}
}
