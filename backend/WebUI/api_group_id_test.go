package WebUI_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/free5gc/openapi/models"
	"github.com/free5gc/webconsole/backend/WebUI"
)

func writeSubscriber(
	t *testing.T, router *gin.Engine, token, method string, groupIds []string,
) *httptest.ResponseRecorder {
	t.Helper()
	subsData := WebUI.SubsData{
		PlmnID: "20893",
		UeId:   "imsi-208930000000001",
		SessionManagementSubscriptionData: []models.Udm_SDM_SessionManagementSubscriptionData{
			{
				SingleNssai:      &models.Snssai{Sst: 1, Sd: "010203"},
				InternalGroupIds: groupIds,
			},
		},
	}
	body, err := json.Marshal(subsData)
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), method,
		"/api/subscriber/imsi-208930000000001/20893", bytes.NewReader(body))
	req.Header.Set("Token", token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestSubscriberWriteRejectsInvalidDualSteerGroupIds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, WebUI.InitJwtKey())
	router := WebUI.NewRouter()
	token := WebUI.JWT("admin", "user", "tenant")

	testcases := []struct {
		Name     string
		GroupIds []string
	}{
		{Name: "missing local group ID", GroupIds: []string{"00004453-208-93"}},
		{Name: "two-digit MCC", GroupIds: []string{"00004453-20-93-01"}},
		{Name: "odd-length local group ID", GroupIds: []string{"00004453-208-93-0"}},
		{Name: "local group ID longer than 10 octets", GroupIds: []string{"00004453-208-93-0102030405060708090a0b"}},
		{Name: "no dash after group service identifier", GroupIds: []string{"000044532-208-93-01"}},
		{Name: "two DualSteer group IDs on one S-NSSAI", GroupIds: []string{"00004453-208-93-01", "00004453-208-93-02"}},
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch} {
		for _, tc := range testcases {
			t.Run(method+"/"+tc.Name, func(t *testing.T) {
				rec := writeSubscriber(t, router, token, method, tc.GroupIds)
				require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				require.Contains(t, rec.Body.String(), "internal group ID")
			})
		}
	}
}
