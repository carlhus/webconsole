package WebUI

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/free5gc/openapi/models"
	"github.com/free5gc/webconsole/backend/logger"
)

// dualSteerGroupServiceId is the group service identifier ("DS" in ASCII)
// reserved for DualSteer group IDs.
const dualSteerGroupServiceId = "00004453"

var errInvalidGroupId = errors.New("invalid internal group ID")

// dualSteerGroupIdPattern is the TS 29.571 GroupId pattern
// (<group service identifier>-<MCC>-<MNC>-<local group ID>) with the DualSteer
// group service identifier. Mirrors dualSteerGroupIdSchema in the frontend.
var dualSteerGroupIdPattern = regexp.MustCompile(
	`^` + dualSteerGroupServiceId + `-[0-9]{3}-[0-9]{2,3}-([A-Fa-f0-9]{2}){1,10}$`)

// validateSubscriberInternalGroupIds checks the DualSteer group IDs among the
// internal group IDs: each must be well-formed, and each S-NSSAI carries at most
// one. Other internal group IDs pass through untouched.
func validateSubscriberInternalGroupIds(smData []models.Udm_SDM_SessionManagementSubscriptionData) error {
	for _, sessionData := range smData {
		dualSteerGroupIds := 0
		for _, groupId := range sessionData.InternalGroupIds {
			if !strings.HasPrefix(groupId, dualSteerGroupServiceId) {
				continue
			}
			if !dualSteerGroupIdPattern.MatchString(groupId) {
				return fmt.Errorf("%w %q: DualSteer group ID must be %s-<MCC>-<MNC>-<local group ID hex>",
					errInvalidGroupId, groupId, dualSteerGroupServiceId)
			}
			dualSteerGroupIds++
		}
		if dualSteerGroupIds > 1 {
			var snssai models.Snssai
			if sessionData.SingleNssai != nil {
				snssai = *sessionData.SingleNssai
			}
			return fmt.Errorf("%w for S-NSSAI %+v: at most one DualSteer group ID is allowed",
				errInvalidGroupId, snssai)
		}
	}

	return nil
}

func validateSubscriberInternalGroupIdsForWrite(c *gin.Context, subsData *SubsData) bool {
	err := validateSubscriberInternalGroupIds(subsData.SessionManagementSubscriptionData)
	if err == nil {
		return true
	}

	logger.ProcLog.Warnf("Internal group ID validation failed: %v", err)
	c.JSON(http.StatusBadRequest, gin.H{responseCause: err.Error()})
	return false
}
