package server

import (
	"bytes"
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type snapClient struct {
	key string
	http *http.Client
}

type snapResult struct {
	Token string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

func (s snapClient) request(ctx context.Context, method, endpoint string, body any, result any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, &buf)
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.key, "")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("provider HTTP status %d", res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(result)
}

type paymentResponse struct {
	PaymentID string `json:"paymentId"`
	OrderNumber string `json:"orderNumber"`
	Status string `json:"status"`
	SnapToken string `json:"snapToken,omitempty"`
	RedirectURL string `json:"redirectUrl,omitempty"`
}

func registerPaymentRoutes(r chi.Router, pool *pgxpool.Pool) {
	s := snapClient{key: os.Getenv("MIDTRANS_SERVER_KEY"), http: &http.Client{Timeout: 8 * time.Second}}
	r.Post("/orders/{orderNumber}/payments", func(w http.ResponseWriter, r *http.Request) {
		if s.key == "" {
			writeAPIError(w, 503, "provider_disabled", "Midtrans Sandbox key is not configured")
			return
		}
		ctx := r.Context()
		number := chi.URLParam(r, "orderNumber")
		tx, err := pool.Begin(ctx)
		if err != nil {
			writeAPIError(w, 500, "database_error", "Unable to start payment")
			return
		}
		defer tx.Rollback(ctx)
		var orderID, state string
		var amount int64
		err = tx.QueryRow(ctx, "SELECT id, status, amount FROM orders WHERE order_number=$1 FOR UPDATE", number).Scan(&orderID, &state, &amount)
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, 404, "order_not_found", "Order was not found")
			return
		}
		if err != nil {
			writeAPIError(w, 500, "database_error", "Unable to read order")
			return
		}
		if state != "pending" {
			writeAPIError(w, 409, "order_not_payable", "Order is not pending")
			return
		}
		p := paymentResponse{OrderNumber: number}
		err = tx.QueryRow(ctx, "SELECT id,status,COALESCE(snap_token,''),COALESCE(redirect_url,'') FROM payments WHERE order_id=$1 ORDER BY created_at DESC LIMIT 1", orderID).Scan(&p.PaymentID,&p.Status,&p.SnapToken,&p.RedirectURL)
		if err == nil {
			if p.SnapToken == "" {
				writeAPIError(w, 409, "payment_unresolved", "Payment already reserved; reconcile provider result before retrying")
				return
			}
			writeJSON(w, 200, p)
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, 500, "database_error", "Unable to read payment")
			return
		}
		err = tx.QueryRow(ctx, "INSERT INTO payments(order_id,provider_order_id,amount) VALUES($1,$2,$3) RETURNING id,status",orderID,number,amount).Scan(&p.PaymentID,&p.Status)
		if err == nil {
			_,err = tx.Exec(ctx,"INSERT INTO payment_attempts(payment_id,attempt_number) VALUES($1,1)",p.PaymentID)
		}
		if err != nil || tx.Commit(ctx) != nil {
			writeAPIError(w,500,"database_error","Unable to reserve payment")
			return
		}
		var result snapResult
		err = s.request(ctx,"POST","https://app.sandbox.midtrans.com/snap/v1/transactions",map[string]any{"transaction_details":map[string]any{"order_id":number,"gross_amount":amount}},&result)
		saveCtx,cancel := context.WithTimeout(context.Background(),4*time.Second)
		defer cancel()
		if err != nil || result.Token == "" || result.RedirectURL == "" {
			_,_ = pool.Exec(saveCtx,"UPDATE payment_attempts SET status='uncertain',error_message='Provider result not confirmed' WHERE payment_id=$1",p.PaymentID)
			writeAPIError(w,502,"provider_uncertain","Payment reserved; provider result needs reconciliation")
			return
		}
		saveTx,err := pool.Begin(saveCtx)
		if err != nil {
			writeAPIError(w,500,"database_error","Unable to save provider result")
			return
		}
		defer saveTx.Rollback(saveCtx)
		err = saveTx.QueryRow(saveCtx,"UPDATE payments SET snap_token=$2,redirect_url=$3,updated_at=NOW() WHERE id=$1 RETURNING status",p.PaymentID,result.Token,result.RedirectURL).Scan(&p.Status)
		if err == nil {
			_,err = saveTx.Exec(saveCtx,"UPDATE payment_attempts SET status='succeeded' WHERE payment_id=$1",p.PaymentID)
		}
		if err != nil || saveTx.Commit(saveCtx) != nil {
			writeAPIError(w,500,"database_error","Unable to save provider result")
			return
		}
		p.SnapToken,p.RedirectURL=result.Token,result.RedirectURL
		writeJSON(w,201,p)
	})
	r.Get("/orders/{orderNumber}/payments",func(w http.ResponseWriter,r *http.Request){
		p:=paymentResponse{OrderNumber:chi.URLParam(r,"orderNumber")}
		err:=pool.QueryRow(r.Context(),"SELECT p.id,p.status FROM payments p JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1 ORDER BY p.created_at DESC LIMIT 1",p.OrderNumber).Scan(&p.PaymentID,&p.Status)
		if errors.Is(err,pgx.ErrNoRows){writeAPIError(w,404,"payment_not_found","Payment was not found");return}
		if err!=nil{writeAPIError(w,500,"database_error","Unable to read payment");return}
		writeJSON(w,200,p)
	})
	r.Post("/webhooks/midtrans",func(w http.ResponseWriter,r *http.Request){s.webhook(w,r,pool)})
}

type midtransNotice struct {
	OrderID string `json:"order_id"`
	StatusCode string `json:"status_code"`
	GrossAmount string `json:"gross_amount"`
	Signature string `json:"signature_key"`
	Status string `json:"transaction_status"`
	Fraud string `json:"fraud_status"`
	TransactionID string `json:"transaction_id"`
}

func validNotice(n midtransNotice,key string) bool {
	sum:=sha512.Sum512([]byte(n.OrderID+n.StatusCode+n.GrossAmount+key))
	return key!="" && subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])),[]byte(n.Signature))==1
}

func rupiah(v string)(int64,error){
	parts:=strings.Split(v,".")
	if len(parts)>2 || (len(parts)==2 && parts[1]!="00"){return 0,errors.New("invalid amount")}
	return strconv.ParseInt(parts[0],10,64)
}

func nextPayment(old string,n midtransNotice)(string,string){
	if n.Status=="settlement" || (n.Status=="capture" && n.Fraud=="accept"){
		if old=="refund" || old=="partial_refund" || old=="chargeback"{return old,""}
		return n.Status,"paid"
	}
	if old=="settlement" || old=="capture" || old=="refund" || old=="partial_refund" || old=="chargeback"{return old,""}
	if old!="pending" && old!="authorize"{return old,""}
	switch n.Status{
	case "deny","cancel","expire","failure":
		return n.Status,""
	case "authorize":
		return "authorize",""
	}
	return old,""
}

func (s snapClient) webhook(w http.ResponseWriter,r *http.Request,pool *pgxpool.Pool){
	raw,err:=io.ReadAll(http.MaxBytesReader(w,r.Body,64<<10))
	var incoming midtransNotice
	if err!=nil || json.Unmarshal(raw,&incoming)!=nil || !validNotice(incoming,s.key){
		writeAPIError(w,401,"invalid_notification","Invalid notification")
		return
	}
	var n midtransNotice
	if err=s.request(r.Context(),"GET","https://api.sandbox.midtrans.com/v2/"+url.PathEscape(incoming.OrderID)+"/status",nil,&n);err!=nil || n.OrderID!=incoming.OrderID || n.TransactionID==""{
		writeAPIError(w,502,"provider_unverified","Unable to verify provider status")
		return
	}
	amount,err:=rupiah(n.GrossAmount)
	if err!=nil{writeAPIError(w,400,"invalid_amount","Invalid amount");return}
	ctx:=r.Context()
	tx,err:=pool.Begin(ctx)
	if err!=nil{writeAPIError(w,500,"database_error","Unable to process notification");return}
	defer tx.Rollback(ctx)
	var pid,oid,old string
	var expected int64
	err=tx.QueryRow(ctx,"SELECT id,order_id,status,amount FROM payments WHERE provider='midtrans' AND provider_order_id=$1 FOR UPDATE",n.OrderID).Scan(&pid,&oid,&old,&expected)
	if errors.Is(err,pgx.ErrNoRows){writeAPIError(w,404,"payment_not_found","Payment was not found");return}
	if err!=nil{writeAPIError(w,500,"database_error","Unable to read payment");return}
	if amount!=expected{writeAPIError(w,409,"amount_mismatch","Amount does not match");return}
	eventHash:=sha512.Sum512(raw)
	tag,err:=tx.Exec(ctx,"INSERT INTO webhook_events(provider,event_key,provider_order_id,signature_valid,payload,processing_status,processed_at) VALUES('midtrans',$1,$2,true,$3,'processed',NOW()) ON CONFLICT(provider,event_key) DO NOTHING",hex.EncodeToString(eventHash[:]),n.OrderID,string(raw))
	if err!=nil{writeAPIError(w,500,"database_error","Unable to record notification");return}
	if tag.RowsAffected()==0{writeJSON(w,200,map[string]string{"status":"duplicate"});return}
	next,orderState:=nextPayment(old,n)
	if next!=old{
		_,err=tx.Exec(ctx,"UPDATE payments SET status=$2,provider_transaction_id=$3,paid_at=CASE WHEN $4='paid' THEN COALESCE(paid_at,NOW()) ELSE paid_at END,updated_at=NOW() WHERE id=$1",pid,next,n.TransactionID,orderState)
		if err==nil{_,err=tx.Exec(ctx,"INSERT INTO payment_events(payment_id,event_type,previous_status,new_status,source) VALUES($1,'status_changed',$2,$3,'webhook')",pid,old,next)}
		if err==nil && orderState=="paid"{
			_,err=tx.Exec(ctx,"UPDATE orders SET status='paid',updated_at=NOW() WHERE id=$1 AND status='pending'",oid)
			if err==nil{_,err=tx.Exec(ctx,"INSERT INTO outbox_events(aggregate_type,aggregate_id,event_type,payload) VALUES('payment',$1,'payment.paid',jsonb_build_object('orderId',$2::text))",pid,oid)}
		}
	}
	if err!=nil || tx.Commit(ctx)!=nil{writeAPIError(w,500,"database_error","Unable to commit notification");return}
	writeJSON(w,200,map[string]string{"status":"ok"})
}
