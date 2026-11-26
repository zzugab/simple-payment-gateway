package server

import (
	"crypto/sha512"
	"encoding/hex"
	"testing"
)

func TestNoticeSignature(t *testing.T) {
	n := midtransNotice{OrderID: "ORD-1", StatusCode: "200", GrossAmount: "125000.00"}
	sum := sha512.Sum512([]byte("ORD-1200125000.00secret"))
	n.Signature = hex.EncodeToString(sum[:])
	if !validNotice(n, "secret") || validNotice(n, "wrong") || validNotice(n, "") {
		t.Fatal("unexpected signature validation")
	}
}

func TestRupiah(t *testing.T) {
	if got, err := rupiah("125000.00"); err != nil || got != 125000 {
		t.Fatalf("unexpected amount: %d %v", got, err)
	}
	if _, err := rupiah("125000.01"); err == nil {
		t.Fatal("fractional rupiah accepted")
	}
}

func TestPaymentState(t *testing.T) {
	if next, order := nextPayment("pending", midtransNotice{Status: "settlement"}); next != "settlement" || order != "paid" {
		t.Fatal("settlement must mark order paid")
	}
	if next, _ := nextPayment("settlement", midtransNotice{Status: "pending"}); next != "settlement" {
		t.Fatal("paid payment regressed")
	}
	if _, order := nextPayment("pending", midtransNotice{Status: "capture", Fraud: "challenge"}); order != "" {
		t.Fatal("challenged capture marked paid")
	}
}
