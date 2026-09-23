package ops

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/metrics"
)

func TestEndpoints(t *testing.T) {
	handler := New(func() error { return errors.New("not ready") }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) }), metrics.New([]string{"adshield"}).Registry())
	tests := []struct {
		path   string
		status int
	}{{"/healthz", 200}, {"/readyz", 503}, {"/metrics", 200}, {"/install", 201}, {"/missing", 404}}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != test.status {
			t.Fatalf("%s got %d", test.path, recorder.Code)
		}
	}
}
