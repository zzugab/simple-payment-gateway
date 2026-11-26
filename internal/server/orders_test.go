package server

import (
	"encoding/json"
	"testing"
)

func validCreateOrderRequest() createOrderRequest {
	var req createOrderRequest
	req.Customer.Name = "Andika"
	req.Customer.Email = "ANDIKA@example.com "
	req.Customer.Phone = "08123456789"
	req.Amount = 125000
	req.Currency = " idr "
	req.Description = "Test order"
	return req
}

func TestCreateOrderRequestValidateNormalizesValues(t *testing.T) {
	req := validCreateOrderRequest()
	if err := req.validate(); err != nil {
		t.Fatalf("validate() returned error: %v", err)
	}
	if req.Customer.Email != "andika@example.com" {
		t.Fatalf("expected normalized email, got %q", req.Customer.Email)
	}
	if req.Currency != "IDR" {
		t.Fatalf("expected normalized currency IDR, got %q", req.Currency)
	}
}

func TestCreateOrderRequestValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		edit func(*createOrderRequest)
	}{
		{name: "missing customer name", edit: func(r *createOrderRequest) { r.Customer.Name = " " }},
		{name: "invalid email", edit: func(r *createOrderRequest) { r.Customer.Email = "not-an-email" }},
		{name: "zero amount", edit: func(r *createOrderRequest) { r.Amount = 0 }},
		{name: "negative amount", edit: func(r *createOrderRequest) { r.Amount = -1 }},
		{name: "unsupported currency", edit: func(r *createOrderRequest) { r.Currency = "USD" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreateOrderRequest()
			tt.edit(&req)
			if err := req.validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCreateOrderRequestHashStableForNormalizedPayload(t *testing.T) {
	first := validCreateOrderRequest()
	if err := first.validate(); err != nil {
		t.Fatal(err)
	}
	second := first
	hash1, err := first.hash()
	if err != nil {
		t.Fatal(err)
	}
	hash2, err := second.hash()
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Fatalf("expected stable hashes, got %s and %s", hash1, hash2)
	}
	first.Amount++
	hash3, err := first.hash()
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == hash3 {
		t.Fatal("expected changed request to produce a different hash")
	}
}

func TestOrderResponseJSONShape(t *testing.T) {
	response := orderResponse{OrderNumber: "ORD-ABC", Amount: 125000, Currency: "IDR", Status: "pending"}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["orderNumber"] != "ORD-ABC" {
		t.Fatalf("unexpected orderNumber: %v", decoded["orderNumber"])
	}
}
