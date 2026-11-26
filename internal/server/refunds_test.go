package server

import "testing"

func TestRefundAmount(t *testing.T) {
	for _, tt := range []struct {
		requested int64
		want      int64
		ok        bool
	}{
		{0, 130000, true},
		{30000, 30000, true},
		{130001, 130001, false},
		{-1, -1, false},
	} {
		got, ok := refundAmount(tt.requested, 130000)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("refundAmount(%d) = %d, %t", tt.requested, got, ok)
		}
	}
}

func TestRefundTransitions(t *testing.T) {
	if p, o := nextPayment("settlement", midtransNotice{Status: "partial_refund"}); p != "partial_refund" || o != "partially_refunded" {
		t.Fatal("partial refund not applied")
	}
	if p, o := nextPayment("partial_refund", midtransNotice{Status: "refund"}); p != "refund" || o != "refunded" {
		t.Fatal("full refund not applied")
	}
	if p, _ := nextPayment("refund", midtransNotice{Status: "settlement"}); p != "refund" {
		t.Fatal("refund regressed")
	}
}
