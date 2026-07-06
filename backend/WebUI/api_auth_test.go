package WebUI_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/free5gc/webconsole/backend/WebUI"
)

func setupAuthMiddlewareRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	require.NoError(t, WebUI.InitJwtKey())

	router := gin.New()
	group := router.Group("/api")
	group.Use(WebUI.AuthMiddleware())

	group.POST("/login", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	group.OPTIONS("/subscriber", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	group.GET("/subscriber/:ueId/:servingPlmnId", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	group.PUT("/subscriber/:ueId/:servingPlmnId", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	group.PATCH("/subscriber/:ueId/:servingPlmnId", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	group.DELETE("/subscriber/:ueId/:servingPlmnId", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	group.DELETE("/subscriber", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	group.GET("/protected", func(c *gin.Context) {
		tenantId, err := WebUI.GetTenantId(c)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"tenantId": tenantId,
			"isAdmin":  WebUI.CheckAuth(c),
		})
	})

	return router
}

func TestAuthMiddlewareRejectsSubscriberRequestsWithoutToken(t *testing.T) {
	router := setupAuthMiddlewareRouter(t)

	testcases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "get subscriber",
			method: http.MethodGet,
			path:   "/api/subscriber/imsi-208930000000100/20893",
		},
		{
			name:   "put subscriber",
			method: http.MethodPut,
			path:   "/api/subscriber/imsi-208930000000100/20893",
			body:   "{}",
		},
		{
			name:   "patch subscriber",
			method: http.MethodPatch,
			path:   "/api/subscriber/imsi-208930000000100/20893",
			body:   "{}",
		},
		{
			name:   "delete subscriber",
			method: http.MethodDelete,
			path:   "/api/subscriber/imsi-208930000000100/20893",
		},
		{
			name:   "delete multiple subscribers",
			method: http.MethodDelete,
			path:   "/api/subscriber",
			body:   "[]",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestAuthMiddlewareRejectsInvalidToken(t *testing.T) {
	router := setupAuthMiddlewareRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/subscriber/imsi-208930000000100/20893", nil)
	req.Header.Set("Token", "not-a-valid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddlewareAllowsLoginAndOptionsWithoutToken(t *testing.T) {
	router := setupAuthMiddlewareRouter(t)

	testcases := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "login",
			method: http.MethodPost,
			path:   "/api/login",
		},
		{
			name:   "options",
			method: http.MethodOptions,
			path:   "/api/subscriber",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusNoContent, w.Code)
		})
	}
}

func TestAuthMiddlewareStoresClaimsForAuthenticatedRequest(t *testing.T) {
	router := setupAuthMiddlewareRouter(t)
	token := WebUI.JWT("user@example.com", "user-id", "tenant-a")

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("Token", token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		TenantId string `json:"tenantId"`
		IsAdmin  bool   `json:"isAdmin"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "tenant-a", response.TenantId)
	require.False(t, response.IsAdmin)
}

func TestAuthMiddlewareStoresAdminFlag(t *testing.T) {
	router := setupAuthMiddlewareRouter(t)
	token := WebUI.JWT("admin", "admin-id", "admin-tenant")

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("Token", token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		TenantId string `json:"tenantId"`
		IsAdmin  bool   `json:"isAdmin"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "admin-tenant", response.TenantId)
	require.True(t, response.IsAdmin)
}
