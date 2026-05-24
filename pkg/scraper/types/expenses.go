package types

import "net/http"

type RawExpenses struct {
	Contract *http.Response
	Hospitality *http.Response
	Travel *http.Response
}