package v2_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	apiv2 "github.com/guidewire-oss/fern-platform/internal/api/v2"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

type fakeSavedViewRepo struct {
	mu    sync.Mutex
	items map[uint]*domain.SavedView
	next  uint
	err   error
}

func newFakeSavedViewRepo() *fakeSavedViewRepo {
	return &fakeSavedViewRepo{items: map[uint]*domain.SavedView{}}
}

func (f *fakeSavedViewRepo) Create(_ context.Context, v *domain.SavedView) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	// emulate the unique constraint
	for _, it := range f.items {
		if it.UserID == v.UserID && it.Page == v.Page && it.Name == v.Name {
			return domain.ErrSavedViewConflict
		}
	}
	f.next++
	v.ID = f.next
	cp := *v
	f.items[v.ID] = &cp
	return nil
}

func (f *fakeSavedViewRepo) List(_ context.Context, userID, page string) ([]*domain.SavedView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []*domain.SavedView{}
	for _, it := range f.items {
		if it.UserID != userID {
			continue
		}
		if page != "" && it.Page != page {
			continue
		}
		cp := *it
		out = append(out, &cp)
	}
	return out, nil
}

func (f *fakeSavedViewRepo) Delete(_ context.Context, userID string, id uint) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[id]
	if !ok || it.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

func newSavedViewRouter(t *testing.T, repo apiv2.SavedViewRepo, uid string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	provider := func(_ *gin.Context) string { return uid }
	h := apiv2.NewSavedViewHandler(repo, provider)
	h.Register(r.Group("/api/v2"))
	return r
}

func TestSavedView_CreateThenList(t *testing.T) {
	repo := newFakeSavedViewRepo()
	r := newSavedViewRouter(t, repo, "u1")

	body := strings.NewReader(`{"page":"test-runs","name":"Failures","filter":{"status":["failed"]}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/me/saved-views", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/me/saved-views", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d", w.Code)
	}
	var resp struct {
		Views []struct {
			Name string `json:"name"`
		} `json:"views"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Views) != 1 || resp.Views[0].Name != "Failures" {
		t.Errorf("list returned %+v", resp)
	}
}

func TestSavedView_DuplicateReturns409(t *testing.T) {
	repo := newFakeSavedViewRepo()
	r := newSavedViewRouter(t, repo, "u1")

	post := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		body := bytes.NewReader([]byte(`{"page":"p","name":"n","filter":{}}`))
		req := httptest.NewRequest(http.MethodPost, "/api/v2/me/saved-views", body)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	if w := post(); w.Code != http.StatusCreated {
		t.Fatalf("first create: %d", w.Code)
	}
	if w := post(); w.Code != http.StatusConflict {
		t.Errorf("duplicate create: %d", w.Code)
	}
}

func TestSavedView_UnauthenticatedReturns401(t *testing.T) {
	repo := newFakeSavedViewRepo()
	r := newSavedViewRouter(t, repo, "") // empty user

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/me/saved-views", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d", w.Code)
	}
}

func TestSavedView_DeleteOwnerOnly(t *testing.T) {
	repo := newFakeSavedViewRepo()
	// Seed a row for u1.
	if err := repo.Create(context.Background(), &domain.SavedView{
		UserID: "u1", Page: "p", Name: "n", FilterJSON: []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}

	// As u2: should 404 (we leak no info that the resource exists).
	rOther := newSavedViewRouter(t, repo, "u2")
	w := httptest.NewRecorder()
	rOther.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v2/me/saved-views/1", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("non-owner delete: got %d", w.Code)
	}

	// As u1: 204.
	rOwn := newSavedViewRouter(t, repo, "u1")
	w = httptest.NewRecorder()
	rOwn.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v2/me/saved-views/1", nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("owner delete: got %d body=%s", w.Code, w.Body.String())
	}
}

func TestSavedView_BadIDReturns400(t *testing.T) {
	r := newSavedViewRouter(t, newFakeSavedViewRepo(), "u1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v2/me/saved-views/abc", nil))
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}

func TestSavedView_ListPagination(t *testing.T) {
	repo := newFakeSavedViewRepo()
	// Seed 5 views for u1.
	for i := 0; i < 5; i++ {
		if err := repo.Create(context.Background(), &domain.SavedView{
			UserID: "u1", Page: "p", Name: "n" + string(rune('a'+i)),
			FilterJSON: []byte(`{}`),
		}); err != nil {
			t.Fatal(err)
		}
	}
	r := newSavedViewRouter(t, repo, "u1")

	doList := func(query string) (map[string]any, int) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/me/saved-views?"+query, nil))
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return body, w.Code
	}

	// Default: all 5 returned, totalCount=5.
	body, code := doList("")
	if code != http.StatusOK {
		t.Fatalf("default list: %d", code)
	}
	if int(body["totalCount"].(float64)) != 5 {
		t.Errorf("totalCount = %v want 5", body["totalCount"])
	}
	if got := len(body["views"].([]any)); got != 5 {
		t.Errorf("default list returned %d views want 5", got)
	}

	// limit=2: only 2 returned, totalCount stays 5.
	body, code = doList("limit=2")
	if code != http.StatusOK {
		t.Fatalf("limit=2: %d", code)
	}
	if got := len(body["views"].([]any)); got != 2 {
		t.Errorf("limit=2 returned %d views", got)
	}

	// limit=2&offset=4: only 1 left in the window.
	body, code = doList("limit=2&offset=4")
	if code != http.StatusOK {
		t.Fatalf("offset=4: %d", code)
	}
	if got := len(body["views"].([]any)); got != 1 {
		t.Errorf("offset=4 returned %d views", got)
	}

	// offset past end: empty slice, not error.
	body, code = doList("offset=99")
	if code != http.StatusOK {
		t.Fatalf("offset=99: %d", code)
	}
	if got := len(body["views"].([]any)); got != 0 {
		t.Errorf("offset past end should be empty, got %d", got)
	}

	// Bad limit: 400.
	if _, code = doList("limit=banana"); code != http.StatusBadRequest {
		t.Errorf("limit=banana: %d", code)
	}
	if _, code = doList("limit=-3"); code != http.StatusBadRequest {
		t.Errorf("limit=-3: %d", code)
	}
	if _, code = doList("offset=-1"); code != http.StatusBadRequest {
		t.Errorf("offset=-1: %d", code)
	}
}

func TestSavedView_CreateRequiresPageAndName(t *testing.T) {
	r := newSavedViewRouter(t, newFakeSavedViewRepo(), "u1")
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"page":"","name":"","filter":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/me/saved-views", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d", w.Code)
	}
}
