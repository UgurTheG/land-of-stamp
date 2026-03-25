package db

import (
	"time"

	"land-of-stamp-backend/gen/pb"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// ── Entities ───────────────────────────────────────────

// User represents an application user stored in the database.
type User struct {
	gorm.Model
	Username      string    `gorm:"uniqueIndex;type:text;not null"`
	PasswordHash  string    `gorm:"type:text;not null;default:''"`
	Role          string    `gorm:"type:text;not null;check:role IN ('user','admin')"`
	OAuthProvider string    `gorm:"column:oauth_provider;type:text;not null;default:''"` // "google" | "github" | ""
	OAuthID       string    `gorm:"column:oauth_id;type:text;not null;default:''"`       // provider's user ID
	UUID          uuid.UUID `gorm:"type:text;uniqueIndex;not null"`
}

// Shop represents a stamp-card shop stored in the database.
type Shop struct {
	gorm.Model
	Name              string    `gorm:"uniqueIndex;type:text;not null"`
	Description       string    `gorm:"type:text;not null;default:''"`
	RewardDescription string    `gorm:"column:reward_description;type:text;not null;default:''"`
	Color             string    `gorm:"type:text;not null;default:'#6366f1'"`
	OwnerID           string    `gorm:"column:owner_id;type:text;not null;index"` // stores User.UUID as string
	UUID              uuid.UUID `gorm:"type:text;uniqueIndex;not null"`
	StampsRequired    int32     `gorm:"column:stamps_required;type:integer;not null;default:8"`
}

// StampCard represents a user's stamp card for a specific shop.
type StampCard struct {
	gorm.Model
	UserID   string    `gorm:"column:user_id;type:text;not null;index:idx_user_shop,composite:user_shop"`       // User.UUID
	ShopID   string    `gorm:"column:shop_id;type:text;not null;index:idx_user_shop,composite:user_shop;index"` // Shop.UUID
	UUID     uuid.UUID `gorm:"type:text;uniqueIndex;not null"`
	Stamps   int32     `gorm:"type:integer;not null;default:0;check:stamps >= 0"`
	Redeemed bool      `gorm:"type:boolean;not null;default:false"`
}

// StampToken represents a short-lived QR token used to grant stamps.
type StampToken struct {
	gorm.Model
	ExpiresAt time.Time `gorm:"column:expires_at;type:datetime;not null"`
	ShopID    string    `gorm:"column:shop_id;type:text;not null;index"` // Shop.UUID
	Token     string    `gorm:"uniqueIndex;type:text;not null"`
	UUID      uuid.UUID `gorm:"type:text;uniqueIndex;not null"`
}

// StampTokenClaim records that a user claimed a particular stamp token.
type StampTokenClaim struct {
	gorm.Model
	TokenID string `gorm:"column:token_id;type:text;not null;uniqueIndex:idx_token_user;index"` // StampToken.UUID
	UserID  string `gorm:"column:user_id;type:text;not null;uniqueIndex:idx_token_user"`        // User.UUID
}

// ── Table names (match existing DB schema) ─────────────

// TableName returns the database table name for User.
func (*User) TableName() string { return "users" }

// TableName returns the database table name for Shop.
func (*Shop) TableName() string { return "shops" }

// TableName returns the database table name for StampCard.
func (*StampCard) TableName() string { return "stamp_cards" }

// TableName returns the database table name for StampToken.
func (*StampToken) TableName() string { return "stamp_tokens" }

// TableName returns the database table name for StampTokenClaim.
func (*StampTokenClaim) TableName() string { return "stamp_token_claims" }

// ── Proto conversion helpers ───────────────────────────

// ToProto converts a User to its protobuf representation.
func (u *User) ToProto() *pb.User {
	return &pb.User{
		Id:       u.UUID.String(),
		Username: u.Username,
		Role:     u.Role,
	}
}

// ToProto converts a Shop to its protobuf representation.
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

// ToProto converts a StampCard to its protobuf representation.
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

// ToProtoToken converts a StampToken to its protobuf representation.
func (t *StampToken) ToProtoToken() *pb.StampToken {
	return &pb.StampToken{
		Token:     t.Token,
		ExpiresAt: t.ExpiresAt.Format(time.RFC3339),
		ShopId:    t.ShopID,
	}
}

// ── Slice-to-proto helpers ─────────────────────────────

// UsersToProto converts a slice of Users to a slice of proto messages.
func UsersToProto(users []User) []proto.Message {
	msgs := make([]proto.Message, len(users))
	for i := range users {
		msgs[i] = users[i].ToProto()
	}
	return msgs
}

// ShopsToProto converts a slice of Shops to a slice of proto messages.
func ShopsToProto(shops []Shop) []proto.Message {
	msgs := make([]proto.Message, len(shops))
	for i := range shops {
		msgs[i] = shops[i].ToProto()
	}
	return msgs
}

// CardsToProto converts a slice of StampCards to a slice of proto messages.
func CardsToProto(cards []StampCard) []proto.Message {
	msgs := make([]proto.Message, len(cards))
	for i := range cards {
		msgs[i] = cards[i].ToProto()
	}
	return msgs
}
