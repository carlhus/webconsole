package WebUI

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAndFormatIMSI(t *testing.T) {
	value, err := parseIMSI("imsi-208930000000001")
	require.NoError(t, err)
	require.Equal(t, int64(208930000000001), value)
	require.Equal(t, "imsi-208930000000001", formatIMSI(value))

	_, err = parseIMSI("supi-208930000000001")
	require.Error(t, err)

	_, err = parseIMSI("imsi-20893000000001")
	require.Error(t, err)

	_, err = parseIMSI("imsi-20893000000000x")
	require.Error(t, err)
}

func TestIMSIRangeForPLMN(t *testing.T) {
	minValue, maxValue, err := imsiBoundsForPLMN("20893")
	require.NoError(t, err)
	require.Equal(t, int64(208930000000000), minValue)
	require.Equal(t, int64(208939999999999), maxValue)

	require.NoError(t, validateIMSIForPLMN(208930000000001, "20893"))
	require.Error(t, validateIMSIForPLMN(208940000000000, "20893"))

	require.NoError(t, validateIMSIRangeForPLMN(208939999999998, 2, "20893"))
	require.Error(t, validateIMSIRangeForPLMN(208939999999998, 3, "20893"))
}

func TestSubscriberAllocatorFloor(t *testing.T) {
	require.Equal(t, int64(208930000000100), subscriberAllocatorFloor(
		208930000000001,
		208930000000100,
	))
	require.Equal(t, int64(208930000000200), subscriberAllocatorFloor(
		208930000000200,
		208930000000100,
	))
}

func TestBuildSubscriberGPSIRange(t *testing.T) {
	gpsis, err := buildSubscriberGPSIRange([]string{"msisdn-1000"}, 3)
	require.NoError(t, err)
	require.Equal(t, []string{"msisdn-1000", "msisdn-1001", "msisdn-1002"}, gpsis)

	gpsis, err = buildSubscriberGPSIRange([]string{"msisdn-"}, 3)
	require.NoError(t, err)
	require.Nil(t, gpsis)

	gpsis, err = buildSubscriberGPSIRange(nil, 3)
	require.NoError(t, err)
	require.Nil(t, gpsis)

	_, err = buildSubscriberGPSIRange([]string{"msisdn-abc"}, 3)
	require.Error(t, err)
}

func TestReplaceMSISDN(t *testing.T) {
	require.Equal(t,
		[]string{"msisdn-2000"},
		replaceMSISDN([]string{"msisdn-1000"}, "msisdn-2000"),
	)
	require.Equal(t,
		[]string{"nai-user", "msisdn-2000"},
		replaceMSISDN([]string{"nai-user", "msisdn-1000"}, "msisdn-2000"),
	)
	require.Equal(t,
		[]string{"msisdn-2000", "nai-user"},
		replaceMSISDN([]string{"nai-user"}, "msisdn-2000"),
	)
}
