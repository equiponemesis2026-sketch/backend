package domain

import (
	"context"
	"time"
)

type Tier string

const (
	TierFree     Tier = "free"
	TierPro      Tier = "pro"
	TierFamiliar Tier = "familiar"
)

type Subscription struct {
	ID               string     `json:"subscription_id" bson:"_id"`
	UserID           string     `json:"user_id" bson:"user_id"`
	Tier             Tier       `json:"tier" bson:"tier"`
	StripeCustomerID string     `json:"stripe_customer_id,omitempty" bson:"stripe_customer_id,omitempty"`
	StripeSubsID     string     `json:"stripe_subscription_id,omitempty" bson:"stripe_subscription_id,omitempty"`
	Status           string     `json:"status" bson:"status"`
	CurrentPeriodEnd *time.Time `json:"current_period_end,omitempty" bson:"current_period_end,omitempty"`
	CreatedAt        time.Time  `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" bson:"updated_at"`
}

type CheckoutRequest struct {
	PriceID    string `json:"price_id"`
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

type CheckoutResponse struct {
	SessionURL string `json:"session_url"`
	SessionID  string `json:"session_id"`
}

type SubscriptionRepository interface {
	Upsert(ctx context.Context, sub *Subscription) error
	FindByUserID(ctx context.Context, userID string) (*Subscription, error)
	FindByStripeCustomerID(ctx context.Context, customerID string) (*Subscription, error)
}

type BillingUseCase interface {
	CreateCheckoutSession(ctx context.Context, userID string, req CheckoutRequest) (*CheckoutResponse, error)
	HandleWebhook(ctx context.Context, payload []byte, signature string) error
	GetSubscription(ctx context.Context, userID string) (*Subscription, error)
}