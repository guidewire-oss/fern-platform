package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/pkg/middleware"
)

func TestDeprecationOnPrefix_OnlyMatchesPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.DeprecationOnPrefix("/api/v1", middleware.DeprecationOptions{
		Enabled: true,
		Sunset:  time.Date(2027, 5, 14, 0, 0, 0, 0, time.UTC),
		Link:    "https://docs.fern/migrate-v2",
	}))
	r.GET("/api/v1/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v2/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/graphql", func(c *gin.Context) { c.Status(http.StatusOK) })

	cases := []struct {
		path        string
		wantHeader  bool
	}{
		{"/api/v1/x", true},
		{"/api/v2/x", false},
		{"/graphql", false},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		got := w.Header().Get("Deprecation")
		hasHeader := got != ""
		if hasHeader != tc.wantHeader {
			t.Errorf("%s: got Deprecation=%q (header=%v), want header=%v",
				tc.path, got, hasHeader, tc.wantHeader)
		}
	}
}
