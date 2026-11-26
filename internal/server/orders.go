package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxOrderBodyBytes = 16 << 10
)

type createOrderRequest struct {
	Customer struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
	} `json:"customer"`
	Amount      int64       `json:"amount"`
	Currency    string      `json:"currency"`
	Description string      `json:"description"`
	Items       []orderItem `json:"items,omitempty"`
}

type orderResponse struct {
	OrderNumber string    `json:"orderNumber"`
	Amount      int64     `json:"amount"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type apiError struct {
	Error apiErrorDetail `json:"error"`
}

type apiErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (req *createOrderRequest) validate() error {
	req.Customer.Name = strings.TrimSpace(req.Customer.Name)
	req.Customer.Email = strings.ToLower(strings.TrimSpace(req.Customer.Email))
	req.Customer.Phone = strings.TrimSpace(req.Customer.Phone)
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	req.Description = strings.TrimSpace(req.Description)

	if req.Customer.Name == "" || len(req.Customer.Name) > 120 {
		return errors.New("customer.name is required and must be at most 120 characters")
	}
	if req.Customer.Email == "" || len(req.Customer.Email) > 254 {
		return errors.New("customer.email is required and must be at most 254 characters")
	}
	address, err := mail.ParseAddress(req.Customer.Email)
	if err != nil || address.Address != req.Customer.Email {
		return errors.New("customer.email must be a valid email address")
	}
	if len(req.Customer.Phone) > 32 {
		return errors.New("customer.phone must be at most 32 characters")
	}
	if err := validateItems(req.Items, req.Amount); err != nil {
		return err
	}
	if req.Currency == "" {
		req.Currency = "IDR"
	}
	if req.Currency != "IDR" {
		return errors.New("only IDR currency is currently supported")
	}
	if len(req.Description) > 255 {
		return errors.New("description must be at most 255 characters")
	}
	return nil
}

func (req *createOrderRequest) hash() (string, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func createOrderHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
			writeAPIError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
			return
		}

		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if len(key) < 8 || len(key) > 255 {
			writeAPIError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key header must contain 8 to 255 characters")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxOrderBodyBytes)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var req createOrderRequest
		if err := decoder.Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON")
			return
		}
		if err := ensureJSONEOF(decoder); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_json", "Request body must contain exactly one JSON object")
			return
		}
		if err := req.validate(); err != nil {
			writeAPIError(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
			return
		}
		requestHash, err := req.hash()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "Unable to process request")
			return
		}

		response, status, err := persistOrder(r.Context(), pool, key, requestHash, req)
		if errors.Is(err, errIdempotencyConflict) {
			writeAPIError(w, http.StatusConflict, "idempotency_conflict", "Idempotency-Key was already used with a different request")
			return
		}
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "Unable to create order")
			return
		}
		w.Header().Set("Idempotency-Replayed", fmt.Sprintf("%t", status == http.StatusOK))
		writeJSON(w, status, response)
	}
}

func persistOrder(ctx context.Context, pool *pgxpool.Pool, key, requestHash string, req createOrderRequest) (orderResponse, int, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return orderResponse{}, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var idempotencyID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO idempotency_keys (scope, idempotency_key, request_hash, expires_at)
		VALUES ('create_order', $1, $2, NOW() + INTERVAL '24 hours')
		ON CONFLICT (scope, idempotency_key) DO UPDATE
		SET request_hash = EXCLUDED.request_hash,
		    response_status = NULL,
		    response_body = NULL,
		    resource_id = NULL,
		    created_at = NOW(),
		    expires_at = EXCLUDED.expires_at
		WHERE idempotency_keys.expires_at <= NOW()
		RETURNING id
	`, key, requestHash).Scan(&idempotencyID)

	if errors.Is(err, pgx.ErrNoRows) {
		var existingHash string
		var responseStatus int
		var responseBody []byte
		err = tx.QueryRow(ctx, `
			SELECT request_hash, response_status, response_body
			FROM idempotency_keys
			WHERE scope = 'create_order' AND idempotency_key = $1
			FOR UPDATE
		`, key).Scan(&existingHash, &responseStatus, &responseBody)
		if err != nil {
			return orderResponse{}, 0, err
		}
		if existingHash != requestHash {
			return orderResponse{}, 0, errIdempotencyConflict
		}
		if len(responseBody) == 0 || responseStatus == 0 {
			return orderResponse{}, 0, errors.New("idempotency record is incomplete")
		}
		var response orderResponse
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return orderResponse{}, 0, err
		}
		if err := tx.Commit(ctx); err != nil {
			return orderResponse{}, 0, err
		}
		return response, http.StatusOK, nil
	}
	if err != nil {
		return orderResponse{}, 0, err
	}

	var customerID string
	err = tx.QueryRow(ctx, `
		INSERT INTO customers (full_name, email, phone)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (LOWER(email)) DO UPDATE
		SET full_name = EXCLUDED.full_name,
		    phone = COALESCE(EXCLUDED.phone, customers.phone),
		    updated_at = NOW()
		RETURNING id
	`, req.Customer.Name, req.Customer.Email, req.Customer.Phone).Scan(&customerID)
	if err != nil {
		return orderResponse{}, 0, err
	}

	orderNumber, err := newOrderNumber()
	if err != nil {
		return orderResponse{}, 0, err
	}
	snapshot, err := json.Marshal(req.Customer)
	if err != nil {
		return orderResponse{}, 0, err
	}
	var orderID string
	var response orderResponse
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (order_number, customer_id, amount, currency, status, description, customer_snapshot)
		VALUES ($1, $2, $3, $4, 'pending', NULLIF($5, ''), $6)
		RETURNING id, order_number, amount, currency, status, COALESCE(description, ''), created_at
	`, orderNumber, customerID, req.Amount, req.Currency, req.Description, snapshot).Scan(
		&orderID, &response.OrderNumber, &response.Amount, &response.Currency,
		&response.Status, &response.Description, &response.CreatedAt,
	)
	if err != nil {
		return orderResponse{}, 0, err
	}

	for i, item := range req.Items {
		_, err = tx.Exec(ctx, "INSERT INTO order_items(order_id,line_number,item_id,name,price,quantity) VALUES($1,$2,$3,$4,$5,$6)", orderID, i+1, item.ID, item.Name, item.Price, item.Quantity)
		if err != nil {
			return orderResponse{}, 0, err
		}
	}

	responseBody, err := json.Marshal(response)
	if err != nil {
		return orderResponse{}, 0, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_status = $2, response_body = $3, resource_id = $4
		WHERE id = $1
	`, idempotencyID, http.StatusCreated, responseBody, orderID)
	if err != nil {
		return orderResponse{}, 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return orderResponse{}, 0, err
	}
	return response, http.StatusCreated, nil
}

var errIdempotencyConflict = errors.New("idempotency key was already used with a different request")

func getOrderHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderNumber := chi.URLParam(r, "orderNumber")
		var response orderResponse
		err := pool.QueryRow(r.Context(), `
			SELECT order_number, amount, currency, status, COALESCE(description, ''), created_at
			FROM orders
			WHERE order_number = $1
		`, orderNumber).Scan(
			&response.OrderNumber, &response.Amount, &response.Currency,
			&response.Status, &response.Description, &response.CreatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, "order_not_found", "Order was not found")
			return
		}
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "Unable to retrieve order")
			return
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func newOrderNumber() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return "ORD-" + strings.ToUpper(hex.EncodeToString(random[:])), nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiError{Error: apiErrorDetail{Code: code, Message: message}})
}
