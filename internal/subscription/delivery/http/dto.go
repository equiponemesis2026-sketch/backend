package http

type checkoutRequest struct {
	PriceID    string `json:"price_id"`
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

type checkoutResponse struct {
	SessionURL string `json:"session_url"`
	SessionID  string `json:"session_id"`
}

type subscriptionResponse struct {
	Tier            string  `json:"tier"`
	Status          string  `json:"status"`
	CurrentPeriodEnd *string `json:"current_period_end,omitempty"`
}