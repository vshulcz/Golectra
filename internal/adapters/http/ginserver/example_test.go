package ginserver_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/vshulcz/Golectra/internal/adapters/http/ginserver"
	"github.com/vshulcz/Golectra/internal/adapters/repository/memory"
	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/services/metrics"
)

func newExampleRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	repo := memory.New()
	svc := metrics.New(repo, nil, nil)
	handler := ginserver.NewHandler(svc)
	return ginserver.NewRouter(handler, zap.NewNop())
}

func ExampleNewRouter_plainTextEndpoints() {
	router := newExampleRouter()

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Temp/42.5", nil)
	router.ServeHTTP(resp, req)
	fmt.Println(resp.Code, strings.TrimSpace(resp.Body.String()))

	resp = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/value/gauge/Temp", nil)
	router.ServeHTTP(resp, req)
	fmt.Println(resp.Code, strings.TrimSpace(resp.Body.String()))

	// Output:
	// 200 ok
	// 200 42.5
}

func ExampleNewRouter_jsonEndpoints() {
	router := newExampleRouter()

	updateBody := bytes.NewBufferString(`{"id":"PollCount","type":"counter","delta":3}`)
	req := httptest.NewRequest(http.MethodPost, "/update", updateBody)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	fmt.Println(resp.Code)

	valueBody := bytes.NewBufferString(`{"id":"PollCount","type":"counter"}`)
	req = httptest.NewRequest(http.MethodPost, "/value", valueBody)
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	var m domain.Metrics
	_ = json.NewDecoder(resp.Body).Decode(&m)
	fmt.Printf("%d %s %s %d\n", resp.Code, m.MType, m.ID, *m.Delta)

	// Output:
	// 200
	// 200 counter PollCount 3
}
