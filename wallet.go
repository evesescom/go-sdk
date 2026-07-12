package eveses

import (
	"context"
	"encoding/json"
)

// WalletBalance is the snapshot of total / held / available balance and
// currency returned by /api/v1/wallet.
type WalletBalance struct {
	Balance          int    `json:"balance"`
	HeldBalance      int    `json:"held_balance"`
	AvailableBalance int    `json:"available_balance"`
	Currency         string `json:"currency"`
}

// WalletService wraps /api/v1/wallet.
type WalletService struct {
	client *Client
}

// Balance returns the wallet's current balance snapshot. Currency defaults
// to "USD" when not explicitly returned by the API.
func (s *WalletService) Balance(ctx context.Context) (*WalletBalance, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/wallet",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var bal WalletBalance
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &bal); err != nil {
			return nil, &Error{Message: "decode wallet balance: " + err.Error()}
		}
	}
	if bal.Currency == "" {
		bal.Currency = "USD"
	}
	return &bal, nil
}
