package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStaticHandlerServesExportedRoutes(t *testing.T) {
	handler := StaticHandler(fstest.MapFS{
		"index.html":      {Data: []byte("<html>overview</html>")},
		"mods/index.html": {Data: []byte("<html>mods</html>")},
	})
	for _, route := range []string{"/", "/mods/"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s returned %d, want 200", route, recorder.Code)
		}
	}
}

func TestStaticHandlerReportsMissingBuild(t *testing.T) {
	handler := StaticHandler(fstest.MapFS{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", recorder.Code)
	}
}
