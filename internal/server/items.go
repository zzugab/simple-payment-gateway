package server

import (
	"errors"
	"strings"
	"unicode/utf8"
)

type orderItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Quantity int64  `json:"quantity"`
}

const maxLabAmount int64 = 100000000

func validateItems(items []orderItem, amount int64) error {
	if amount <= 0 || amount > maxLabAmount {
		return errors.New("amount must be between 1 and 100000000 IDR")
	}
	if len(items) > 50 {
		return errors.New("at most 50 items are allowed")
	}
	if len(items) == 0 {
		return nil
	}
	var total int64
	for i := range items {
		item := &items[i]
		item.ID = strings.TrimSpace(item.ID)
		item.Name = strings.TrimSpace(item.Name)
		if item.ID == "" || item.Name == "" || utf8.RuneCountInString(item.ID) > 50 || utf8.RuneCountInString(item.Name) > 50 {
			return errors.New("item id and name must contain 1 to 50 characters")
		}
		if item.Price <= 0 || item.Price > maxLabAmount || item.Quantity <= 0 || item.Quantity > 10000 || item.Price > (maxLabAmount-total)/item.Quantity {
			return errors.New("invalid item price, quantity or total")
		}
		total += item.Price * item.Quantity
	}
	if total != amount {
		return errors.New("item total must equal order amount")
	}
	return nil
}

func limitName(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}

func snapPayload(number string, amount int64, name, email, phone, description string, items []orderItem) map[string]any {
	if len(items) == 0 {
		if description == "" {
			description = "Order payment"
		}
		items = []orderItem{{ID: number, Name: limitName(description, 50), Price: amount, Quantity: 1}}
	}
	customer := map[string]string{"first_name": limitName(name, 50)}
	if email != "" {
		customer["email"] = email
	}
	if phone != "" {
		customer["phone"] = phone
	}
	return map[string]any{
		"transaction_details": map[string]any{"order_id": number, "gross_amount": amount},
		"customer_details":    customer,
		"item_details":        items,
	}
}
