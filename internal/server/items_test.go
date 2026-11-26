package server

import "testing"

func TestValidateItems(t *testing.T) {
	items := []orderItem{{ID: "NASI", Name: "Nasi box", Price: 25000, Quantity: 4}, {ID: "SNACK", Name: "Snack box", Price: 15000, Quantity: 2}}
	if err := validateItems(items, 130000); err != nil {
		t.Fatal(err)
	}
	if err := validateItems(items, 129000); err == nil {
		t.Fatal("mismatched total accepted")
	}
	items[0].Quantity = 1 << 62
	if err := validateItems(items, 130000); err == nil {
		t.Fatal("oversized quantity accepted")
	}
}

func TestSnapPayload(t *testing.T) {
	p := snapPayload("ORD-1", 99000, "Customer Test", "test@example.com", "08123456789", "KPI subscription", nil)
	customer := p["customer_details"].(map[string]string)
	if customer["first_name"] != "Customer Test" || customer["email"] != "test@example.com" || customer["phone"] != "08123456789" {
		t.Fatal("customer mapping incorrect")
	}
	items := p["item_details"].([]orderItem)
	if len(items) != 1 || items[0].Price != 99000 || items[0].Quantity != 1 {
		t.Fatal("legacy order fallback incorrect")
	}
}
