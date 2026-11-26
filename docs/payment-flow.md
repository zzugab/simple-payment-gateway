# Payment lifecycle (planned)

This is the intended first Midtrans Snap flow. The implementation will be added in the payment milestone.

1. Client asks the API to create an order.
2. API validates the request and persists the order.
3. API creates a payment attempt and requests a Snap transaction token from Midtrans.
4. API returns the Snap token or redirect URL to the client.
5. Customer completes or abandons the payment in the Midtrans checkout.
6. Midtrans sends a server-to-server notification to the webhook endpoint.
7. API verifies the notification and retrieves transaction status from Midtrans when required.
8. API applies an allowed payment state transition in a database transaction.
9. API stores a durable event for downstream fulfillment.

Important rules:

- A frontend redirect or browser callback is not proof of payment.
- Duplicate notifications must be safe to process.
- Notifications may be delayed or arrive out of order.
- Payment state changes must be validated and persisted atomically.
- Credentials must never be sent to the browser or committed to Git.
