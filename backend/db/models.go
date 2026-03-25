package db

import (
	"time"

	"land-of-stamp-backend/gen/pb"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// ── Entities ───────────────────────────────────────────

type User struct {
	gorm.Model
	UUID          uuid.UUID `gorm:"type:text;uniqueIndex;not null"`
	Username      string    `gorm:"uniqueIndex;type:text;not null"`
	PasswordHash  string    `gorm:"type:text;not null;default:''"`
	Role          string    `gorm:"type:text;not null;check:role IN ('user','admin')"`
	OAuthProvider string    `gorm:"column:oauth_provider;type:text;not null;default:''"` // "google" | "github" | ""
	OAuthID       string    `gorm:"column:oauth_id;type:text;not null;default:''"`       // provider's user ID
}

type Shop struct {
	gorm.Model
	UUID              uuid.UUID `gorm:"type:text;uniqueIndex;not null"`
	Name              string    `gorm:"uniqueIndex;type:text;not null"`
	Description       string    `gorm:"type:text;not null;default:''"`
	RewardDescription string    `gorm:"column:reward_description;type:text;not null;default:''"`
	StampsRequired    int32     `gorm:"column:stamps_required;type:integer;not null;default:8"`
	Color             string    `gorm:"type:text;not null;default:'#6366f1'"`
	OwnerID           string    `gorm:"column:owner_id;type:text;not null;index"` // stores User.UUID as string
}

type StampCard struct {
	gorm.Model
	UUID     uuid.UUID `gorm:"type:text;uniqueIndex;not null"`
	UserID   string    `gorm:"column:user_id;type:text;not null;index:idx_user_shop,composite:user_shop"`       // User.UUID
	ShopID   string    `gorm:"column:shop_id;type:text;not null;index:idx_user_shop,composite:user_shop;index"` // Shop.UUID
	Stamps   int32     `gorm:"type:integer;not null;default:0;check:stamps >= 0"`
	Redeemed bool      `gorm:"type:boolean;not null;default:false"`
}

type StampToken struct {
	gorm.Model
	UUID      uuid.UUID `gorm:"type:text;uniqueIndex;not null"`
	ShopID    string    `gorm:"column:shop_id;type:text;not null;index"` // Shop.UUID
	Token     string    `gorm:"uniqueIndex;type:text;not null"`
	ExpiresAt time.Time `gorm:"column:expires_at;type:datetime;not null"`
}

type StampTokenClaim struct {
	gorm.Model
	TokenID string `gorm:"column:token_id;type:text;not null;uniqueIndex:idx_token_user;index"` // StampToken.UUID
	UserID  string `gorm:"column:user_id;type:text;not null;uniqueIndex:idx_token_user"`        // User.UUID
}

// ── Table names (match existing DB schema) ─────────────

func (User) TableName() string            { return "users" }
func (Shop) TableName() string            { return "shops" }
func (StampCard) TableName() string       { return "stamp_cards" }
func (StampToken) TableName() string      { return "stamp_tokens" }
func (StampTokenClaim) TableName() string { return "stamp_token_claims" }

// ── Proto conversion helpers ───────────────────────────

func (u *User) ToProto() *pb.User {
	return &pb.User{
		Id:       u.UUID.String(),
		Username: u.Username,
		Role:     u.Role,
	}
}

func (s *Shop) ToProto() *pb.Shop {
	return &pb.Shop{
		Id:                s.UUID.String(),
		Name:              s.Name,
		Description:       s.Description,
		RewardDescription: s.RewardDescription,
		StampsRequired:    s.StampsRequired,
		Color:             s.Color,
		OwnerId:           s.OwnerID,
	}
}

func (c *StampCard) ToProto() *pb.StampCard {
	return &pb.StampCard{
		Id:        c.UUID.String(),
		UserId:    c.UserID,
		ShopId:    c.ShopID,
		Stamps:    c.Stamps,
		Redeemed:  c.Redeemed,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
	}
}

func (t *StampToken) ToProtoToken() *pb.StampToken {
	return &pb.StampToken{
		Token:     t.Token,
		ExpiresAt: t.ExpiresAt.Format(time.RFC3339),
		ShopId:    t.ShopID,
	}
}

// ── Slice-to-proto helpers ─────────────────────────────

func UsersToProto(users []User) []proto.Message {
	msgs := make([]proto.Message, len(users))
	for i := range users {
		msgs[i] = users[i].ToProto()
	}
	return msgs
}

func ShopsToProto(shops []Shop) []proto.Message {
	msgs := make([]proto.Message, len(shops))
	for i := range shops {
		msgs[i] = shops[i].ToProto()
	}
	return msgs
}

func CardsToProto(cards []StampCard) []proto.Message {
	msgs := make([]proto.Message, len(cards))
	for i := range cards {
		msgs[i] = cards[i].ToProto()
	}
	return msgs
}
