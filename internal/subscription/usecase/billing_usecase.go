package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/webhook"

	"github.com/nemesis-project/api-nemesis/internal/subscription/domain"
)

var (
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrInvalidPriceID       = errors.New("invalid price id")
)

type billingUseCase struct {
	repo              domain.SubscriptionRepository
	stripeKey         string
	webhookSecret     string
	pricePro          string
	priceFamiliar     string
}

func NewBillingUseCase(
	repo domain.SubscriptionRepository,
	stripeKey, webhookSecret, pricePro, priceFamiliar string,
) domain.BillingUseCase {
	return &billingUseCase{
		repo:          repo,
		stripeKey:     stripeKey,
		webhookSecret: webhookSecret,
		pricePro:      pricePro,
		priceFamiliar: priceFamiliar,
	}
}

func (uc *billingUseCase) CreateCheckoutSession(ctx context.Context, userID string, req domain.CheckoutRequest) (*domain.CheckoutResponse, error) {
	stripe.Key = uc.stripeKey

	var tier domain.Tier
	switch req.PriceID {
	case uc.pricePro:
		tier = domain.TierPro
	case uc.priceFamiliar:
		tier = domain.TierFamiliar
	default:
		return nil, ErrInvalidPriceID
	}

	sub, err := uc.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find subscription: %w", err)
	}

	var stripeCustomerID string
	if sub != nil && sub.StripeCustomerID != "" {
		stripeCustomerID = sub.StripeCustomerID
	} else {
		custParams := &stripe.CustomerParams{}
		custParams.AddMetadata("user_id", userID)
		c, err := customer.New(custParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create stripe customer: %w", err)
		}
		stripeCustomerID = c.ID
	}

	sessionParams := &stripe.CheckoutSessionParams{
		Mode:             stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Customer:         &stripeCustomerID,
		ClientReferenceID: &userID,
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: &req.PriceID, Quantity: stripe.Int64(1)},
		},
		SuccessURL: &req.SuccessURL,
		CancelURL:  &req.CancelURL,
		Metadata:   map[string]string{"tier": string(tier), "user_id": userID},
	}

	sess, err := session.New(sessionParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create checkout session: %w", err)
	}

	return &domain.CheckoutResponse{
		SessionURL: sess.URL,
		SessionID:  sess.ID,
	}, nil
}

func (uc *billingUseCase) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	stripe.Key = uc.stripeKey

	event, err := webhook.ConstructEvent(payload, signature, uc.webhookSecret)
	if err != nil {
		return fmt.Errorf("webhook signature verification failed: %w", err)
	}

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return fmt.Errorf("failed to parse checkout session: %w", err)
		}
		if err := uc.handleCheckoutCompleted(ctx, &sess); err != nil {
			return fmt.Errorf("handle checkout completed: %w", err)
		}

	case "customer.subscription.deleted":
		var subs stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &subs); err != nil {
			return fmt.Errorf("failed to parse subscription: %w", err)
		}
		if err := uc.handleSubscriptionDeleted(ctx, &subs); err != nil {
			return fmt.Errorf("handle subscription deleted: %w", err)
		}
	}

	return nil
}

func (uc *billingUseCase) handleCheckoutCompleted(ctx context.Context, sess *stripe.CheckoutSession) error {
	tier := domain.TierPro
	if sess.Metadata != nil {
		if t, ok := sess.Metadata["tier"]; ok {
			tier = domain.Tier(t)
		}
	}

	now := time.Now().UTC()
	var periodEnd *time.Time
	if sess.Subscription != nil {
		sub := sess.Subscription
		t := time.Unix(sub.CurrentPeriodEnd, 0).UTC()
		periodEnd = &t
	}

	sub := &domain.Subscription{
		ID:               fmt.Sprintf("sub_%s", uuid.New().String()),
		UserID:           sess.Metadata["user_id"],
		Tier:             tier,
		StripeCustomerID: sess.Customer.ID,
		Status:           "active",
		CurrentPeriodEnd: periodEnd,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if sess.Subscription != nil {
		sub.StripeSubsID = sess.Subscription.ID
	}

	if sub.UserID == "" && sess.ClientReferenceID != "" {
		sub.UserID = sess.ClientReferenceID
	}

	return uc.repo.Upsert(ctx, sub)
}

func (uc *billingUseCase) handleSubscriptionDeleted(ctx context.Context, subs *stripe.Subscription) error {
	existing, err := uc.repo.FindByStripeCustomerID(ctx, subs.Customer.ID)
	if err != nil || existing == nil {
		return err
	}

	now := time.Now().UTC()
	existing.Status = "canceled"
	existing.Tier = domain.TierFree
	existing.UpdatedAt = now

	return uc.repo.Upsert(ctx, existing)
}

func (uc *billingUseCase) GetSubscription(ctx context.Context, userID string) (*domain.Subscription, error) {
	sub, err := uc.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscription: %w", err)
	}
	if sub == nil {
		return &domain.Subscription{
			UserID:    userID,
			Tier:      domain.TierFree,
			Status:    "active",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}, nil
	}
	return sub, nil
}