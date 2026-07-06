package WebUI

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/free5gc/openapi/models"
	"github.com/free5gc/util/mongoapi"
	"github.com/free5gc/webconsole/backend/factory"
	"github.com/free5gc/webconsole/backend/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	idAllocatorColl = "webui.idAllocator"

	imsiPrefix    = "imsi-"
	imsiDigitSize = 15

	subscriberBulkTimeout = 60 * time.Second
)

var errSubscriberIMSIExhausted = errors.New("subscriber IMSI range exhausted")

type subscriberIMSIAllocation struct {
	Start int64
	Next  int64
}

type bulkSubscriberDocuments struct {
	authWeb  []interface{}
	auth     []interface{}
	am       []interface{}
	sm       []interface{}
	smfSel   []interface{}
	amPolicy []interface{}
	smPolicy []interface{}
	flow     []interface{}
	qos      []interface{}
	charging []interface{}
	identity []interface{}
}

func EnsureSubscriberIndexes() error {
	db, err := webuiMongoDatabase()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	indexes := []struct {
		collName string
		models   []mongo.IndexModel
	}{
		{
			collName: amDataColl,
			models: []mongo.IndexModel{
				{
					Keys: bson.D{
						{Key: "servingPlmnId", Value: 1},
						{Key: "ueId", Value: -1},
					},
					Options: options.Index().SetName("servingPlmnId_1_ueId_-1"),
				},
			},
		},
		{
			collName: authWebSubsDataColl,
			models: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "ueId", Value: 1}},
					Options: options.Index().SetName("ueId_1"),
				},
			},
		},
		{
			collName: identityDataColl,
			models: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "gpsi", Value: 1}},
					Options: options.Index().SetName("gpsi_1"),
				},
			},
		},
	}

	for _, index := range indexes {
		if _, err := db.Collection(index.collName).Indexes().CreateMany(ctx, index.models); err != nil {
			return fmt.Errorf("create %s indexes: %w", index.collName, err)
		}
	}

	return nil
}

func postBulkSubscriberByID(c *gin.Context, subsData *SubsData, claims jwt.MapClaims) {
	servingPlmnID := c.Param("servingPlmnId")
	userNumber, err := strconv.Atoi(c.Param("userNumber"))
	if err != nil || userNumber <= 0 {
		logger.ProcLog.Errorf("PostBulkSubscriberByID userNumber err: %+v", err)
		c.JSON(http.StatusBadRequest, gin.H{"cause": "userNumber format incorrect"})
		return
	}

	seedIMSI, err := parseIMSI(c.Param("ueId"))
	if err != nil {
		logger.ProcLog.Errorf("PostBulkSubscriberByID ueId err: %+v", err)
		c.JSON(http.StatusBadRequest, gin.H{"cause": "ueId format incorrect"})
		return
	}
	if err := validateIMSIForPLMN(seedIMSI, servingPlmnID); err != nil {
		logger.ProcLog.Errorf("PostBulkSubscriberByID PLMN err: %+v", err)
		c.JSON(http.StatusBadRequest, gin.H{"cause": err.Error()})
		return
	}

	db, err := webuiMongoDatabase()
	if err != nil {
		logger.ProcLog.Errorf("PostBulkSubscriberByID mongo err: %+v", err)
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), subscriberBulkTimeout)
	defer cancel()

	existingMax, err := findMaxSubscriberIMSI(ctx, db, servingPlmnID)
	if err != nil {
		logger.ProcLog.Errorf("PostBulkSubscriberByID max IMSI err: %+v", err)
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	floor := subscriberAllocatorFloor(seedIMSI, existingMax+1)
	if err := validateIMSIRangeForPLMN(floor, userNumber, servingPlmnID); err != nil {
		logger.ProcLog.Errorf("PostBulkSubscriberByID range err: %+v", err)
		c.JSON(http.StatusBadRequest, gin.H{"cause": err.Error()})
		return
	}

	allocation, err := reserveSubscriberIMSIRange(ctx, db, servingPlmnID, floor, userNumber)
	if err != nil {
		logger.ProcLog.Errorf("PostBulkSubscriberByID allocator err: %+v", err)
		status := http.StatusInternalServerError
		cause := ""
		if errors.Is(err, errSubscriberIMSIExhausted) {
			status = http.StatusConflict
			cause = err.Error()
		}
		if cause == "" {
			c.JSON(status, gin.H{})
		} else {
			c.JSON(status, gin.H{"cause": cause})
		}
		return
	}

	if err := validateIMSIRangeForPLMN(allocation.Start, userNumber, servingPlmnID); err != nil {
		logger.ProcLog.Errorf("PostBulkSubscriberByID allocated range err: %+v", err)
		c.JSON(http.StatusConflict, gin.H{"cause": err.Error()})
		return
	}

	ueIDs := buildIMSIRange(allocation.Start, userNumber)
	gpsis, err := buildSubscriberGPSIRange(subsData.AccessAndMobilitySubscriptionData.Gpsis, userNumber)
	if err != nil {
		logger.ProcLog.Errorf("PostBulkSubscriberByID gpsi err: %+v", err)
		c.JSON(http.StatusBadRequest, gin.H{"cause": err.Error()})
		return
	}

	if cause, err := findBulkSubscriberConflict(ctx, db, servingPlmnID, ueIDs[0], ueIDs[len(ueIDs)-1], gpsis); err != nil {
		logger.ProcLog.Errorf("PostBulkSubscriberByID conflict check err: %+v", err)
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	} else if cause != "" {
		c.JSON(http.StatusConflict, gin.H{"cause": cause})
		return
	}

	docs, err := buildBulkSubscriberDocuments(subsData, servingPlmnID, ueIDs, gpsis, claims)
	if err != nil {
		logger.ProcLog.Errorf("PostBulkSubscriberByID document err: %+v", err)
		c.JSON(http.StatusBadRequest, gin.H{"cause": err.Error()})
		return
	}

	if err := insertBulkSubscriberDocuments(ctx, db, docs); err != nil {
		logger.ProcLog.Errorf("PostBulkSubscriberByID insert err: %+v", err)
		cleanupBulkSubscriberDocuments(db, servingPlmnID, ueIDs[0], ueIDs[len(ueIDs)-1])
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"created":   userNumber,
		"startUeId": ueIDs[0],
		"endUeId":   ueIDs[len(ueIDs)-1],
	})
}

func webuiMongoDatabase() (*mongo.Database, error) {
	if mongoapi.Client == nil {
		return nil, fmt.Errorf("MongoDB client is not initialized")
	}
	if factory.WebuiConfig == nil ||
		factory.WebuiConfig.Configuration == nil ||
		factory.WebuiConfig.Configuration.Mongodb == nil ||
		factory.WebuiConfig.Configuration.Mongodb.Name == "" {
		return nil, fmt.Errorf("MongoDB name is not configured")
	}

	return mongoapi.Client.Database(factory.WebuiConfig.Configuration.Mongodb.Name), nil
}

func parseIMSI(supi string) (int64, error) {
	if !strings.HasPrefix(supi, imsiPrefix) {
		return 0, fmt.Errorf("SUPI must start with %s", imsiPrefix)
	}

	digits := strings.TrimPrefix(supi, imsiPrefix)
	if len(digits) != imsiDigitSize {
		return 0, fmt.Errorf("IMSI must contain %d digits", imsiDigitSize)
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return 0, fmt.Errorf("IMSI must contain digits only")
		}
	}

	value, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse IMSI: %w", err)
	}
	return value, nil
}

func formatIMSI(value int64) string {
	return fmt.Sprintf("%s%015d", imsiPrefix, value)
}

func buildIMSIRange(start int64, count int) []string {
	ueIDs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		ueIDs = append(ueIDs, formatIMSI(start+int64(i)))
	}
	return ueIDs
}

func subscriberAllocatorFloor(seedIMSI, existingNextIMSI int64) int64 {
	if seedIMSI > existingNextIMSI {
		return seedIMSI
	}
	return existingNextIMSI
}

func validateIMSIForPLMN(imsi int64, plmnID string) error {
	minValue, maxValue, err := imsiBoundsForPLMN(plmnID)
	if err != nil {
		return err
	}
	if imsi < minValue || imsi > maxValue {
		return fmt.Errorf("SUPI Prefix must be same as PLMN")
	}
	return nil
}

func validateIMSIRangeForPLMN(start int64, count int, plmnID string) error {
	if count <= 0 {
		return fmt.Errorf("userNumber must be positive")
	}

	minValue, maxValue, err := imsiBoundsForPLMN(plmnID)
	if err != nil {
		return err
	}
	end := start + int64(count) - 1
	if start < minValue || end > maxValue || end < start {
		return fmt.Errorf("allocated IMSI range exceeds PLMN %s", plmnID)
	}
	return nil
}

func maxIMSIStartForPLMN(plmnID string, count int) (int64, error) {
	_, maxValue, err := imsiBoundsForPLMN(plmnID)
	if err != nil {
		return 0, err
	}
	if count <= 0 {
		return 0, fmt.Errorf("userNumber must be positive")
	}
	return maxValue - int64(count) + 1, nil
}

func imsiBoundsForPLMN(plmnID string) (int64, int64, error) {
	if len(plmnID) < 5 || len(plmnID) > 6 {
		return 0, 0, fmt.Errorf("servingPlmnId format incorrect")
	}
	for _, digit := range plmnID {
		if digit < '0' || digit > '9' {
			return 0, 0, fmt.Errorf("servingPlmnId format incorrect")
		}
	}

	prefix, err := strconv.ParseInt(plmnID, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse servingPlmnId: %w", err)
	}

	multiplier := int64(1)
	for i := 0; i < imsiDigitSize-len(plmnID); i++ {
		multiplier *= 10
	}

	minValue := prefix * multiplier
	return minValue, minValue + multiplier - 1, nil
}

func findMaxSubscriberIMSI(ctx context.Context, db *mongo.Database, servingPlmnID string) (int64, error) {
	var result struct {
		UeID string `bson:"ueId"`
	}

	opts := options.FindOne().
		SetProjection(bson.M{"ueId": 1}).
		SetSort(bson.D{{Key: "ueId", Value: -1}})
	err := db.Collection(amDataColl).
		FindOne(ctx, bson.M{"servingPlmnId": servingPlmnID}, opts).
		Decode(&result)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	imsi, err := parseIMSI(result.UeID)
	if err != nil {
		return 0, err
	}
	return imsi, validateIMSIForPLMN(imsi, servingPlmnID)
}

func reserveSubscriberIMSIRange(
	ctx context.Context,
	db *mongo.Database,
	servingPlmnID string,
	floor int64,
	count int,
) (subscriberIMSIAllocation, error) {
	maxStart, err := maxIMSIStartForPLMN(servingPlmnID, count)
	if err != nil {
		return subscriberIMSIAllocation{}, err
	}
	if floor > maxStart {
		return subscriberIMSIAllocation{}, errSubscriberIMSIExhausted
	}

	count64 := int64(count)
	filter := bson.M{
		"_id": subscriberAllocatorID(servingPlmnID),
		"$or": bson.A{
			bson.M{"nextImsi": bson.M{"$exists": false}},
			bson.M{"nextImsi": bson.M{"$lte": maxStart}},
		},
	}
	update := mongo.Pipeline{
		bson.D{
			{
				Key: "$set",
				Value: bson.D{
					{
						Key: "nextImsi",
						Value: bson.D{
							{
								Key: "$add",
								Value: bson.A{
									bson.D{
										{
											Key: "$max",
											Value: bson.A{
												bson.D{
													{
														Key:   "$ifNull",
														Value: bson.A{"$nextImsi", floor},
													},
												},
												floor,
											},
										},
									},
									count64,
								},
							},
						},
					},
				},
			},
		},
	}

	var result struct {
		NextIMSI int64 `bson:"nextImsi"`
	}
	opts := options.FindOneAndUpdate().
		SetProjection(bson.M{"nextImsi": 1}).
		SetReturnDocument(options.After).
		SetUpsert(true)
	err = db.Collection(idAllocatorColl).FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if errors.Is(err, mongo.ErrNoDocuments) || mongo.IsDuplicateKeyError(err) {
		return subscriberIMSIAllocation{}, errSubscriberIMSIExhausted
	}
	if err != nil {
		return subscriberIMSIAllocation{}, err
	}

	start := result.NextIMSI - count64
	if start > maxStart {
		return subscriberIMSIAllocation{}, errSubscriberIMSIExhausted
	}
	return subscriberIMSIAllocation{Start: start, Next: result.NextIMSI}, nil
}

func subscriberAllocatorID(servingPlmnID string) string {
	return "subscriber:" + servingPlmnID
}

func buildSubscriberGPSIRange(gpsis []string, count int) ([]string, error) {
	baseGPSI := firstMSISDN(gpsis)
	if baseGPSI == "" || baseGPSI == "msisdn-" {
		return nil, nil
	}
	if !strings.HasPrefix(baseGPSI, "msisdn-") {
		return nil, fmt.Errorf("gpsi format incorrect (now only support MSISDN format)")
	}

	start, err := strconv.ParseInt(strings.TrimPrefix(baseGPSI, "msisdn-"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("gpsi format incorrect (now only support MSISDN format)")
	}

	gpsiRange := make([]string, 0, count)
	for i := 0; i < count; i++ {
		gpsiRange = append(gpsiRange, fmt.Sprintf("msisdn-%d", start+int64(i)))
	}
	return gpsiRange, nil
}

func firstMSISDN(gpsis []string) string {
	for _, gpsi := range gpsis {
		if strings.HasPrefix(gpsi, "msisdn-") {
			return gpsi
		}
	}
	return ""
}

func findBulkSubscriberConflict(
	ctx context.Context,
	db *mongo.Database,
	servingPlmnID string,
	startUeID string,
	endUeID string,
	gpsis []string,
) (string, error) {
	ueRange := bson.M{"$gte": startUeID, "$lte": endUeID}
	authCount, err := db.Collection(authWebSubsDataColl).CountDocuments(ctx, bson.M{"ueId": ueRange})
	if err != nil {
		return "", err
	}
	if authCount > 0 {
		return "allocated UE range conflicts with existing authentication data", nil
	}

	amCount, err := db.Collection(amDataColl).CountDocuments(ctx, bson.M{
		"servingPlmnId": servingPlmnID,
		"ueId":          ueRange,
	})
	if err != nil {
		return "", err
	}
	if amCount > 0 {
		return "allocated UE range conflicts with existing subscription data", nil
	}

	if len(gpsis) > 0 {
		gpsiCount, err := db.Collection(identityDataColl).CountDocuments(ctx, bson.M{
			"gpsi": bson.M{"$in": gpsis},
		})
		if err != nil {
			return "", err
		}
		if gpsiCount > 0 {
			return "allocated GPSI range conflicts with existing identity data", nil
		}
	}

	return "", nil
}

func buildBulkSubscriberDocuments(
	subsData *SubsData,
	servingPlmnID string,
	ueIDs []string,
	gpsis []string,
	claims jwt.MapClaims,
) (bulkSubscriberDocuments, error) {
	authSubs, err := WebAuthSubToModels(subsData.WebAuthenticationSubscription)
	if err != nil {
		return bulkSubscriberDocuments{}, err
	}

	webAuthBase := toBsonM(subsData.WebAuthenticationSubscription)
	authBase := toBsonM(authSubs)
	amBase := toBsonM(subsData.AccessAndMobilitySubscriptionData)
	smfSelBase := toBsonM(subsData.SmfSelectionSubscriptionData)
	amPolicyBase := toBsonM(subsData.AmPolicyData)
	smPolicyBase := toBsonM(cloneSmPolicyDataWithEscapedDNN(subsData.SmPolicyData))
	tenantID, hasTenantID := tenantIDFromClaims(claims)

	docs := bulkSubscriberDocuments{}
	for i, ueID := range ueIDs {
		webAuthDoc := cloneBsonM(webAuthBase)
		webAuthDoc["ueId"] = ueID
		authDoc := cloneBsonM(authBase)
		authDoc["ueId"] = ueID
		if hasTenantID {
			webAuthDoc["tenantId"] = tenantID
			authDoc["tenantId"] = tenantID
		}
		docs.authWeb = append(docs.authWeb, webAuthDoc)
		docs.auth = append(docs.auth, authDoc)

		amDoc := cloneBsonM(amBase)
		if len(gpsis) > 0 {
			amDoc["gpsis"] = replaceMSISDN(subsData.AccessAndMobilitySubscriptionData.Gpsis, gpsis[i])
		}
		amDoc["ueId"] = ueID
		amDoc["servingPlmnId"] = servingPlmnID
		if hasTenantID {
			amDoc["tenantId"] = tenantID
		}
		docs.am = append(docs.am, amDoc)

		for _, data := range subsData.SessionManagementSubscriptionData {
			smDoc := toBsonM(data)
			smDoc["ueId"] = ueID
			smDoc["servingPlmnId"] = servingPlmnID
			docs.sm = append(docs.sm, smDoc)
		}

		smfSelDoc := cloneBsonM(smfSelBase)
		smfSelDoc["ueId"] = ueID
		smfSelDoc["servingPlmnId"] = servingPlmnID
		docs.smfSel = append(docs.smfSel, smfSelDoc)

		amPolicyDoc := cloneBsonM(amPolicyBase)
		amPolicyDoc["ueId"] = ueID
		docs.amPolicy = append(docs.amPolicy, amPolicyDoc)

		smPolicyDoc := cloneBsonM(smPolicyBase)
		smPolicyDoc["ueId"] = ueID
		docs.smPolicy = append(docs.smPolicy, smPolicyDoc)

		for _, flowRule := range subsData.FlowRules {
			flowDoc := toBsonM(flowRule)
			flowDoc["ueId"] = ueID
			flowDoc["servingPlmnId"] = servingPlmnID
			docs.flow = append(docs.flow, flowDoc)
		}

		for _, qosFlow := range subsData.QosFlows {
			qosDoc := toBsonM(qosFlow)
			qosDoc["ueId"] = ueID
			qosDoc["servingPlmnId"] = servingPlmnID
			docs.qos = append(docs.qos, qosDoc)
		}

		for _, chargingData := range subsData.ChargingDatas {
			chargingDoc := toBsonM(chargingData)
			if chargingData.ChargingMethod == ChargingOffline {
				chargingDoc["quota"] = "0"
			}
			if chargingData.Dnn == "" || chargingData.Filter == "" {
				chargingDoc["dnn"] = ""
				chargingDoc["filter"] = ""
			}
			chargingDoc["ueId"] = ueID
			chargingDoc["servingPlmnId"] = servingPlmnID
			docs.charging = append(docs.charging, chargingDoc)
		}

		if len(gpsis) > 0 {
			docs.identity = append(docs.identity, bson.M{
				"ueId": ueID,
				"gpsi": gpsis[i],
			})
		}
	}

	return docs, nil
}

func cloneSmPolicyDataWithEscapedDNN(smPolicyData models.SmPolicyData) models.SmPolicyData {
	cloned := smPolicyData
	if smPolicyData.SmPolicySnssaiData == nil {
		return cloned
	}

	cloned.SmPolicySnssaiData = make(map[string]models.SmPolicySnssaiData, len(smPolicyData.SmPolicySnssaiData))
	for key, snssaiData := range smPolicyData.SmPolicySnssaiData {
		if snssaiData.SmPolicyDnnData != nil {
			escapedDnnData := make(map[string]models.SmPolicyDnnData, len(snssaiData.SmPolicyDnnData))
			for dnnKey, dnnData := range snssaiData.SmPolicyDnnData {
				escapedDnnData[EscapeDnn(dnnKey)] = dnnData
			}
			snssaiData.SmPolicyDnnData = escapedDnnData
		}
		cloned.SmPolicySnssaiData[key] = snssaiData
	}
	return cloned
}

func tenantIDFromClaims(claims jwt.MapClaims) (string, bool) {
	if claims == nil {
		return "", false
	}
	tenantID, ok := claims["tenantId"].(string)
	return tenantID, ok && tenantID != ""
}

func cloneBsonM(src map[string]interface{}) bson.M {
	dst := make(bson.M, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func replaceMSISDN(existing []string, generated string) []string {
	if len(existing) == 0 {
		return []string{generated}
	}

	replaced := false
	gpsis := append([]string(nil), existing...)
	for i, gpsi := range gpsis {
		if strings.HasPrefix(gpsi, "msisdn-") {
			gpsis[i] = generated
			replaced = true
			break
		}
	}
	if !replaced {
		gpsis = append([]string{generated}, gpsis...)
	}
	return gpsis
}

func insertBulkSubscriberDocuments(ctx context.Context, db *mongo.Database, docs bulkSubscriberDocuments) error {
	batches := []struct {
		collName string
		docs     []interface{}
	}{
		{collName: authWebSubsDataColl, docs: docs.authWeb},
		{collName: authSubsDataColl, docs: docs.auth},
		{collName: amDataColl, docs: docs.am},
		{collName: smDataColl, docs: docs.sm},
		{collName: flowRuleDataColl, docs: docs.flow},
		{collName: smfSelDataColl, docs: docs.smfSel},
		{collName: amPolicyDataColl, docs: docs.amPolicy},
		{collName: smPolicyDataColl, docs: docs.smPolicy},
		{collName: qosFlowDataColl, docs: docs.qos},
		{collName: chargingDataColl, docs: docs.charging},
		{collName: identityDataColl, docs: docs.identity},
	}

	insertOpts := options.InsertMany().SetOrdered(false)
	for _, batch := range batches {
		if len(batch.docs) == 0 {
			continue
		}
		if _, err := db.Collection(batch.collName).InsertMany(ctx, batch.docs, insertOpts); err != nil {
			return fmt.Errorf("insert %s: %w", batch.collName, err)
		}
	}
	return nil
}

func cleanupBulkSubscriberDocuments(db *mongo.Database, servingPlmnID, startUeID, endUeID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ueRange := bson.M{"$gte": startUeID, "$lte": endUeID}
	ueIDOnlyFilter := bson.M{"ueId": ueRange}
	scopedFilter := bson.M{
		"servingPlmnId": servingPlmnID,
		"ueId":          ueRange,
	}
	deletes := []struct {
		collName string
		filter   bson.M
	}{
		{collName: authSubsDataColl, filter: ueIDOnlyFilter},
		{collName: authWebSubsDataColl, filter: ueIDOnlyFilter},
		{collName: amDataColl, filter: scopedFilter},
		{collName: smDataColl, filter: scopedFilter},
		{collName: flowRuleDataColl, filter: scopedFilter},
		{collName: smfSelDataColl, filter: scopedFilter},
		{collName: amPolicyDataColl, filter: ueIDOnlyFilter},
		{collName: smPolicyDataColl, filter: ueIDOnlyFilter},
		{collName: qosFlowDataColl, filter: scopedFilter},
		{collName: chargingDataColl, filter: scopedFilter},
		{collName: identityDataColl, filter: ueIDOnlyFilter},
	}

	for _, deleteOp := range deletes {
		if _, err := db.Collection(deleteOp.collName).DeleteMany(ctx, deleteOp.filter); err != nil {
			logger.ProcLog.Errorf("cleanup %s err: %+v", deleteOp.collName, err)
		}
	}
}
