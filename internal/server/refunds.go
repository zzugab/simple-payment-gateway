package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type refundRequest struct {
	Amount int64  `json:"amount"`
	Reason string `json:"reason"`
}

type refundResult struct {
	StatusCode string `json:"status_code"`
	OrderID    string `json:"order_id"`
	RefundKey  string `json:"refund_key"`
}

func refundAmount(requested, remaining int64) (int64, bool) {
	if requested == 0 {
		requested = remaining
	}
	return requested, requested > 0 && requested <= remaining
}

func (s snapClient) refund(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	ctx := r.Context()
	number := chi.URLParam(r, "orderNumber")
	var req refundRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || ensureJSONEOF(decoder) != nil || req.Amount < 0 || len(req.Reason) > 255 || strings.TrimSpace(req.Reason) == "" {
		writeAPIError(w, 400, "invalid_refund", "Provide a nonnegative amount and reason")
		return
	}
	if s.key == "" {
		writeAPIError(w, 503, "provider_disabled", "Midtrans Sandbox key is not configured")
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		writeAPIError(w, 500, "database_error", "Unable to reserve refund")
		return
	}
	defer tx.Rollback(ctx)
	var pid, providerID, state string
	var paid, reserved int64
	var unresolved bool
	err = tx.QueryRow(ctx, "SELECT p.id,p.provider_order_id,p.status,p.amount FROM payments p JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1 AND p.provider='midtrans' ORDER BY p.created_at DESC LIMIT 1 FOR UPDATE OF p", number).Scan(&pid, &providerID, &state, &paid)
	if errors.Is(err, pgx.ErrNoRows) {
		writeAPIError(w, 404, "payment_not_found", "Payment was not found")
		return
	}
	if err != nil {
		writeAPIError(w, 500, "database_error", "Unable to read payment")
		return
	}
	err = tx.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0),COALESCE(BOOL_OR(status='uncertain'),false) FROM refunds WHERE payment_id=$1", pid).Scan(&reserved, &unresolved)
	if err != nil {
		writeAPIError(w, 500, "database_error", "Unable to read refunds")
		return
	}
	if unresolved || (state != "settlement" && state != "partial_refund") {
		writeAPIError(w, 409, "refund_blocked", "Refresh payment status or reconcile an unresolved refund first")
		return
	}
	var n midtransNotice
	endpoint := "https://api.sandbox.midtrans.com/v2/" + url.PathEscape(providerID)
	if err := s.request(ctx, "GET", endpoint+"/status", nil, &n); err != nil || n.OrderID != providerID || n.TransactionID == "" || n.Status != state {
		writeAPIError(w, 409, "provider_unverified", "Provider status differs or cannot be verified; refresh first")
		return
	}
	gross, err := rupiah(n.GrossAmount)
	if err != nil || gross != paid {
		writeAPIError(w, 409, "amount_mismatch", "Provider amount differs")
		return
	}
	// Do not infer an externally issued partial refund from local records.
	if state == "partial_refund" {
		writeAPIError(w, 409, "refund_reconciliation_required", "Further refunds after a partial refund require provider refund history reconciliation")
		return
	}
	amount, ok := refundAmount(req.Amount, paid-reserved)
	if !ok || reserved != 0 {
		writeAPIError(w, 422, "invalid_refund_amount", "Refund exceeds available amount or prior refund requires reconciliation")
		return
	}
	key, err := newOrderNumber()
	if err != nil {
		writeAPIError(w, 500, "internal_error", "Unable to generate refund key")
		return
	}
	_, err = tx.Exec(ctx, "INSERT INTO refunds(refund_key,payment_id,amount,reason,status) VALUES($1,$2,$3,$4,'uncertain')", key, pid, amount, req.Reason)
	if err != nil || tx.Commit(ctx) != nil {
		writeAPIError(w, 500, "database_error", "Unable to reserve refund")
		return
	}
	var result refundResult
	err = s.request(ctx, "POST", endpoint+"/refund", map[string]any{"refund_key": key, "amount": amount, "reason": req.Reason}, &result)
	saveCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err != nil || result.StatusCode != "200" || result.OrderID != providerID || result.RefundKey != key {
		reason := "Refund response not confirmed (status " + result.StatusCode + ")"
		if err != nil {
			reason = providerDiagnostic(err)
		}
		_, _ = pool.Exec(saveCtx, "UPDATE refunds SET error_message=$2,updated_at=NOW() WHERE refund_key=$1", key, reason)
		writeAPIError(w, 502, "refund_uncertain", reason+"; do not retry until reconciled")
		return
	}
	_, err = pool.Exec(saveCtx, "UPDATE refunds SET status='confirmed',updated_at=NOW() WHERE refund_key=$1", key)
	if err != nil {
		writeAPIError(w, 500, "refund_uncertain", "Provider acknowledged refund; local record needs reconciliation")
		return
	}
	writeJSON(w, 200, map[string]any{"refundKey": key, "amount": amount, "status": "confirmed", "message": "Refresh payment status to verify final state"})
}

func listRefunds(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(), "SELECT f.refund_key,f.amount,f.status,COALESCE(f.error_message,'') FROM refunds f JOIN payments p ON p.id=f.payment_id JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1 ORDER BY f.created_at DESC", chi.URLParam(r, "orderNumber"))
		if err != nil {
			writeAPIError(w, 500, "database_error", "Unable to read refund history")
			return
		}
		defer rows.Close()
		result := make([]map[string]any, 0)
		for rows.Next() {
			var key, status, message string
			var amount int64
			if err := rows.Scan(&key, &amount, &status, &message); err != nil {
				writeAPIError(w, 500, "database_error", "Unable to read refund history")
				return
			}
			result = append(result, map[string]any{"refundKey": key, "amount": amount, "status": status, "error": message})
		}
		if rows.Err() != nil {
			writeAPIError(w, 500, "database_error", "Unable to read refund history")
			return
		}
		writeJSON(w, 200, result)
	}
}
