package api

import (
	"context"
	"net/http"
	"net/http/httptest"
)

func reqWithCtx() *http.Request {
	return httptest.NewRequest(http.MethodPost, "/x", nil).WithContext(context.Background())
}
