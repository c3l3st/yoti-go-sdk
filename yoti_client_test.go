package yoti

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/getyoti/yoti-go-sdk/v3/aml"
	"github.com/getyoti/yoti-go-sdk/v3/attribute"
	"github.com/getyoti/yoti-go-sdk/v3/consts"
	"github.com/getyoti/yoti-go-sdk/v3/cryptoutil"
	"github.com/getyoti/yoti-go-sdk/v3/dynamic_sharing_service"
	"github.com/getyoti/yoti-go-sdk/v3/profile"
	"github.com/getyoti/yoti-go-sdk/v3/test"
	"github.com/getyoti/yoti-go-sdk/v3/yotiprotoattr"
	"github.com/getyoti/yoti-go-sdk/v3/yotiprotocom"
	"github.com/getyoti/yoti-go-sdk/v3/yotiprotoshare"
	"github.com/golang/protobuf/proto"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type mockHTTPClient struct {
	do func(*http.Request) (*http.Response, error)
}

func (mock *mockHTTPClient) Do(request *http.Request) (*http.Response, error) {
	if mock.do != nil {
		return mock.do(request)
	}
	return nil, nil
}

func createExtraDataContent(t *testing.T, pemBytes []byte, protoExtraData *yotiprotoshare.ExtraData, wrappedReceiptKey string) string {
	outBytes, err := proto.Marshal(protoExtraData)
	assert.NilError(t, err)

	key, err := getValidKey()
	assert.NilError(t, err)

	cipherBytes, err := base64.StdEncoding.DecodeString(test.WrappedReceiptKey)
	assert.NilError(t, err)
	unwrappedKey, err := rsa.DecryptPKCS1v15(rand.Reader, key, cipherBytes)
	assert.NilError(t, err)
	cipherBlock, err := aes.NewCipher(unwrappedKey)
	assert.NilError(t, err)

	padLength := cipherBlock.BlockSize() - len(outBytes)%cipherBlock.BlockSize()
	outBytes = append(outBytes, bytes.Repeat([]byte{byte(padLength)}, padLength)...)

	iv := make([]byte, cipherBlock.BlockSize())
	encrypter := cipher.NewCBCEncrypter(cipherBlock, iv)
	encrypter.CryptBlocks(outBytes, outBytes)

	outProto := &yotiprotocom.EncryptedData{
		CipherText: outBytes,
		Iv:         iv,
	}
	outBytes, err = proto.Marshal(outProto)
	assert.NilError(t, err)

	return base64.StdEncoding.EncodeToString(outBytes)
}

func TestYotiClient_KeyLoad_Failure(t *testing.T) {
	key, _ := ioutil.ReadFile("test/test-key-invalid-format.pem")
	_, err := NewClient("", key)
	assert.Check(t, err != nil)
	assert.Check(t, strings.HasPrefix(err.Error(), "Invalid Key: not PEM-encoded"))
	tempError, temporary := err.(interface {
		Temporary() bool
	})
	assert.Check(t, !temporary || !tempError.Temporary())
}

func TestNewYotiClient_InvalidToken(t *testing.T) {
	var err error
	key, _ := ioutil.ReadFile("test/test-key.pem")

	client, err := NewClient("sdkId", key)
	assert.NilError(t, err)

	_, err = client.GetActivityDetails("")

	assert.Check(t, err != nil)
	assert.Check(t, strings.HasPrefix(err.Error(), "Invalid Token"))
	tempError, temporary := err.(interface {
		Temporary() bool
	})
	assert.Check(t, !temporary || !tempError.Temporary())
}

func TestYotiClient_HttpFailure_ReturnsFailure(t *testing.T) {
	key, err := getValidKey()
	assert.NilError(t, err)

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 500,
				}, nil
			},
		},
		Key: key,
	}

	_, err = client.GetActivityDetails(test.EncryptedToken)

	assert.Check(t, err != nil)
	assert.ErrorContains(t, err, "Unknown HTTP Error")
	tempError, temporary := err.(interface {
		Temporary() bool
		Unwrap() error
	})
	assert.Check(t, temporary)
	assert.Check(t, tempError.Temporary())
	assert.ErrorContains(t, tempError.Unwrap(), "Unknown HTTP Error")
}

func TestYotiClient_HttpFailure_ReturnsProfileNotFound(t *testing.T) {
	key, err := getValidKey()
	assert.NilError(t, err)

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 404,
				}, nil
			},
		},
		Key: key,
	}

	_, err = client.GetActivityDetails(test.EncryptedToken)

	assert.Check(t, err != nil)
	assert.Check(t, strings.HasPrefix(err.Error(), "Profile Not Found"))
	tempError, temporary := err.(interface {
		Temporary() bool
	})
	assert.Check(t, !temporary || !tempError.Temporary())
}

func TestYotiClient_SharingFailure_ReturnsFailure(t *testing.T) {
	key, err := getValidKey()
	assert.NilError(t, err)

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(strings.NewReader(`{"session_data":"session_data","receipt":{"receipt_id": null,"other_party_profile_content": null,"policy_uri":null,"personal_key":null,"remember_me_id":null, "sharing_outcome":"FAILURE","timestamp":"2016-09-23T13:04:11Z"}}`)),
				}, nil
			},
		},
		Key: key,
	}

	_, err = client.GetActivityDetails(test.EncryptedToken)

	assert.Check(t, err != nil)
	assert.Check(t, strings.HasPrefix(err.Error(), profile.ErrSharingFailure.Error()))
	tempError, temporary := err.(interface {
		Temporary() bool
	})
	assert.Check(t, !temporary || !tempError.Temporary())
}

func TestYotiClient_TokenDecodedSuccessfully(t *testing.T) {
	key, err := getValidKey()
	assert.NilError(t, err)

	expectedAbsoluteURL := "/api/v1/profile/" + test.Token

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(request *http.Request) (*http.Response, error) {
				parsed, err := url.Parse(request.URL.String())
				assert.Assert(t, is.Nil(err), "Yoti API did not generate a valid URI.")
				assert.Equal(t, parsed.Path, expectedAbsoluteURL, "Yoti API did not generate a valid URL path.")

				return &http.Response{
					StatusCode: 500,
				}, nil
			},
		},
		Key: key,
	}

	_, err = client.GetActivityDetails(test.EncryptedToken)

	assert.Check(t, err != nil)
	assert.Check(t, strings.HasPrefix(err.Error(), "Unknown HTTP Error"))
	tempError, temporary := err.(interface {
		Temporary() bool
	})
	assert.Check(t, temporary && tempError.Temporary())
}

func TestYotiClient_ParseProfile_Success(t *testing.T) {
	key, err := getValidKey()
	assert.NilError(t, err)

	otherPartyProfileContent := "ChCZAib1TBm9Q5GYfFrS1ep9EnAwQB5shpAPWLBgZgFgt6bCG3S5qmZHhrqUbQr3yL6yeLIDwbM7x4nuT/MYp+LDXgmFTLQNYbDTzrEzqNuO2ZPn9Kpg+xpbm9XtP7ZLw3Ep2BCmSqtnll/OdxAqLb4DTN4/wWdrjnFC+L/oQEECu646"
	rememberMeID := "remember_me_id0123456789"

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(strings.NewReader(`{"receipt":{"wrapped_receipt_key": "` + test.WrappedReceiptKey + `","other_party_profile_content": "` + otherPartyProfileContent + `","remember_me_id":"` + rememberMeID + `", "sharing_outcome":"SUCCESS"}}`)),
				}, nil
			},
		},
		Key: key,
	}

	activityDetails, errorStrings := client.GetActivityDetails(test.EncryptedToken)

	assert.Assert(t, is.Nil(errorStrings))

	profile := activityDetails.UserProfile

	assert.Equal(t, activityDetails.RememberMeID(), rememberMeID)

	assert.Assert(t, is.Nil(activityDetails.ExtraData().AttributeIssuanceDetails()))

	expectedSelfieValue := "selfie0123456789"

	assert.DeepEqual(t, profile.Selfie().Value().Data, []byte(expectedSelfieValue))
	assert.Equal(t, profile.MobileNumber().Value(), "phone_number0123456789")

	assert.Equal(
		t,
		profile.GetAttribute("phone_number").Value(),
		"phone_number0123456789",
	)

	assert.Check(t,
		profile.GetImageAttribute("doesnt_exist") == nil,
	)

	assert.Check(t, profile.GivenNames() == nil)
	assert.Check(t, profile.FamilyName() == nil)
	assert.Check(t, profile.FullName() == nil)
	assert.Check(t, profile.EmailAddress() == nil)
	images, _ := profile.DocumentImages()
	assert.Check(t, images == nil)
	documentDetails, _ := profile.DocumentDetails()
	assert.Check(t, documentDetails == nil)

	expectedDoB := time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

	actualDoB, err := profile.DateOfBirth()
	assert.Assert(t, is.Nil(err))

	assert.Assert(t, actualDoB != nil)
	assert.DeepEqual(t, actualDoB.Value(), &expectedDoB)
}

func TestYotiClient_ParentRememberMeID(t *testing.T) {
	key, err := getValidKey()
	assert.NilError(t, err)

	otherPartyProfileContent := "ChCZAib1TBm9Q5GYfFrS1ep9EnAwQB5shpAPWLBgZgFgt6bCG3S5qmZHhrqUbQr3yL6yeLIDwbM7x4nuT/MYp+LDXgmFTLQNYbDTzrEzqNuO2ZPn9Kpg+xpbm9XtP7ZLw3Ep2BCmSqtnll/OdxAqLb4DTN4/wWdrjnFC+L/oQEECu646"
	parentRememberMeID := "parent_remember_me_id0123456789"

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body: ioutil.NopCloser(strings.NewReader(`{"receipt":{"wrapped_receipt_key": "` + test.WrappedReceiptKey +
						`","other_party_profile_content": "` + otherPartyProfileContent +
						`","parent_remember_me_id":"` + parentRememberMeID + `", "sharing_outcome":"SUCCESS"}}`)),
				}, nil
			},
		},
		Key: key,
	}

	activityDetails, errorStrings := client.GetActivityDetails(test.EncryptedToken)

	assert.Assert(t, is.Nil(errorStrings))
	assert.Equal(t, activityDetails.ParentRememberMeID(), parentRememberMeID)
}
func TestYotiClient_ParseWithoutProfile_Success(t *testing.T) {
	key, err := getValidKey()
	assert.NilError(t, err)

	rememberMeID := "remember_me_id0123456789"
	timestamp := time.Date(1973, 11, 29, 9, 33, 9, 0, time.UTC)
	timestampString := func(a []byte, _ error) string {
		return string(a)
	}(timestamp.MarshalText())
	receiptID := "receipt_id123"

	var otherPartyProfileContents = []string{
		`"other_party_profile_content": null,`,
		`"other_party_profile_content": "",`,
		``}

	for _, otherPartyProfileContent := range otherPartyProfileContents {

		client := Client{
			HTTPClient: &mockHTTPClient{
				do: func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body: ioutil.NopCloser(strings.NewReader(`{"receipt":{"wrapped_receipt_key": "` + test.WrappedReceiptKey + `",` +
							otherPartyProfileContent + `"remember_me_id":"` + rememberMeID + `", "sharing_outcome":"SUCCESS", "timestamp":"` + timestampString + `", "receipt_id":"` + receiptID + `"}}`)),
					}, nil
				},
			},
			Key: key,
		}

		activityDetails, errStrings := client.GetActivityDetails(test.EncryptedToken)

		assert.Assert(t, is.Nil(errStrings))
		assert.Equal(t, activityDetails.RememberMeID(), rememberMeID)
		assert.Equal(t, activityDetails.Timestamp(), timestamp)
		assert.Equal(t, activityDetails.ReceiptID(), receiptID)
	}
}

func TestYotiClient_ShouldParseAndDecryptExtraDataContent(t *testing.T) {
	otherPartyProfileContent := "ChCZAib1TBm9Q5GYfFrS1ep9EnAwQB5shpAPWLBgZgFgt6bCG3S5qmZHhrqUbQr3yL6yeLIDwbM7x4nuT/MYp+LDXgmFTLQNYbDTzrEzqNuO2ZPn9Kpg+xpbm9XtP7ZLw3Ep2BCmSqtnll/OdxAqLb4DTN4/wWdrjnFC+L/oQEECu646"
	rememberMeID := "remember_me_id0123456789"

	pemBytes, err := ioutil.ReadFile("test/test-key.pem")
	assert.NilError(t, err)

	attributeName := "attributeName"
	dataEntries := make([]*yotiprotoshare.DataEntry, 0)
	expiryDate := time.Now().UTC().AddDate(0, 0, 1)
	thirdPartyAttributeDataEntry := test.CreateThirdPartyAttributeDataEntry(t, &expiryDate, []string{attributeName}, "tokenValue")

	dataEntries = append(dataEntries, &thirdPartyAttributeDataEntry)
	protoExtraData := &yotiprotoshare.ExtraData{
		List: dataEntries,
	}

	extraDataContent := createExtraDataContent(t, pemBytes, protoExtraData, test.WrappedReceiptKey)

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body: ioutil.NopCloser(strings.NewReader(`{"receipt":{"wrapped_receipt_key": "` +
						test.WrappedReceiptKey + `","other_party_profile_content": "` + otherPartyProfileContent + `","extra_data_content": "` +
						extraDataContent + `","remember_me_id":"` + rememberMeID + `", "sharing_outcome":"SUCCESS"}}`)),
				}, nil
			},
		},
	}
	client.Key, err = cryptoutil.ParseRSAKey(pemBytes)
	assert.NilError(t, err)

	activityDetails, err := client.GetActivityDetails(test.EncryptedToken)
	assert.NilError(t, err)

	assert.Equal(t, rememberMeID, activityDetails.RememberMeID())
	assert.Assert(t, activityDetails.ExtraData().AttributeIssuanceDetails() != nil)
	assert.Equal(t, activityDetails.UserProfile.MobileNumber().Value(), "phone_number0123456789")
}

func TestYotiClient_ShouldCarryOnProcessingIfIssuanceTokenIsNotPresent(t *testing.T) {
	var attributeName = "attributeName"
	dataEntries := make([]*yotiprotoshare.DataEntry, 0)
	expiryDate := time.Now().UTC().AddDate(0, 0, 1)
	thirdPartyAttributeDataEntry := test.CreateThirdPartyAttributeDataEntry(t, &expiryDate, []string{attributeName}, "")

	dataEntries = append(dataEntries, &thirdPartyAttributeDataEntry)
	protoExtraData := &yotiprotoshare.ExtraData{
		List: dataEntries,
	}

	pemBytes, err := ioutil.ReadFile("test/test-key.pem")
	assert.NilError(t, err)

	extraDataContent := createExtraDataContent(t, pemBytes, protoExtraData, test.WrappedReceiptKey)

	otherPartyProfileContent := "ChCZAib1TBm9Q5GYfFrS1ep9EnAwQB5shpAPWLBgZgFgt6bCG3S5qmZHhrqUbQr3yL6yeLIDwbM7x4nuT/MYp+LDXgmFTLQNYbDTzrEzqNuO2ZPn9Kpg+xpbm9XtP7ZLw3Ep2BCmSqtnll/OdxAqLb4DTN4/wWdrjnFC+L/oQEECu646"

	rememberMeID := "remember_me_id0123456789"

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body: ioutil.NopCloser(strings.NewReader(`{"receipt":{"wrapped_receipt_key": "` +
						test.WrappedReceiptKey + `","other_party_profile_content": "` + otherPartyProfileContent + `","extra_data_content": "` +
						extraDataContent + `","remember_me_id":"` + rememberMeID + `", "sharing_outcome":"SUCCESS"}}`)),
				}, nil
			},
		},
	}
	client.Key, err = cryptoutil.ParseRSAKey(pemBytes)
	assert.NilError(t, err)

	activityDetails, err := client.GetActivityDetails(test.EncryptedToken)

	assert.Check(t, err != nil)
	assert.Check(t, strings.Contains(err.Error(), "Issuance Token is invalid"))

	assert.Equal(t, rememberMeID, activityDetails.RememberMeID())
	assert.Assert(t, is.Nil(activityDetails.ExtraData().AttributeIssuanceDetails()))
	assert.Equal(t, activityDetails.UserProfile.MobileNumber().Value(), "phone_number0123456789")
}
func TestYotiClient_ParseWithoutRememberMeID_Success(t *testing.T) {
	key, err := getValidKey()
	assert.NilError(t, err)

	var otherPartyProfileContents = []string{
		`"other_party_profile_content": null,`,
		`"other_party_profile_content": "",`}

	for _, otherPartyProfileContent := range otherPartyProfileContents {

		client := Client{
			HTTPClient: &mockHTTPClient{
				do: func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body: ioutil.NopCloser(strings.NewReader(`{"receipt":{"wrapped_receipt_key": "` + test.WrappedReceiptKey + `",` +
							otherPartyProfileContent + `"sharing_outcome":"SUCCESS"}}`)),
					}, nil
				},
			},
			Key: key,
		}

		_, errStrings := client.GetActivityDetails(test.EncryptedToken)

		assert.Assert(t, is.Nil(errStrings))
	}
}

func TestYotiClient_UnmarshallJSONValue_InvalidValueThrowsError(t *testing.T) {
	invalidStructuredAddress := []byte("invalidBool")

	_, err := attribute.UnmarshallJSON(invalidStructuredAddress)

	assert.Assert(t, err != nil)
}

func TestYotiClient_UnmarshallJSONValue_ValidValue(t *testing.T) {
	const (
		countryIso  = "IND"
		nestedValue = "NestedValue"
	)

	var structuredAddress = []byte(`
	{
		"address_format": 2,
		"building": "House No.86-A",		
		"state": "Punjab",
		"postal_code": "141012",
		"country_iso": "` + countryIso + `",
		"country": "India",
		"formatted_address": "House No.86-A\nRajgura Nagar\nLudhina\nPunjab\n141012\nIndia",
		"1":
		{
			"1-1":
			{
			  "1-1-1": "` + nestedValue + `"
			}
		}
	}
	`)

	parsedStructuredAddress, err := attribute.UnmarshallJSON(structuredAddress)

	assert.Assert(t, is.Nil(err), "Failed to parse structured address")

	actualCountryIso := parsedStructuredAddress["country_iso"]

	assert.Equal(t, countryIso, actualCountryIso)
}

func TestClient_OverrideAPIURL_ShouldSetAPIURL(t *testing.T) {
	client := &Client{}
	expectedURL := "expectedurl.com"
	client.OverrideAPIURL(expectedURL)
	assert.Equal(t, client.getAPIURL(), expectedURL)
}

func TestYotiClient_GetAPIURLUsesOverriddenBaseUrlOverEnvVariable(t *testing.T) {
	client := Client{}
	client.OverrideAPIURL("overridenBaseUrl")

	os.Setenv("YOTI_API_URL", "envBaseUrl")

	result := client.getAPIURL()

	assert.Equal(t, "overridenBaseUrl", result)
}

func TestYotiClient_GetAPIURLUsesEnvVariable(t *testing.T) {
	client := Client{}

	os.Setenv("YOTI_API_URL", "envBaseUrl")

	result := client.getAPIURL()

	assert.Equal(t, "envBaseUrl", result)
}

func TestYotiClient_GetAPIURLUsesDefaultUrlAsFallbackWithEmptyEnvValue(t *testing.T) {
	client := Client{}

	os.Setenv("YOTI_API_URL", "")

	result := client.getAPIURL()

	assert.Equal(t, "https://api.yoti.com/api/v1", result)
}

func TestYotiClient_GetAPIURLUsesDefaultUrlAsFallbackWithNoEnvValue(t *testing.T) {
	client := Client{}

	os.Unsetenv("YOTI_API_URL")

	result := client.getAPIURL()

	assert.Equal(t, "https://api.yoti.com/api/v1", result)
}

func createStandardAmlProfile() (result aml.AmlProfile) {
	var amlAddress = aml.AmlAddress{
		Country: "GBR"}

	var amlProfile = aml.AmlProfile{
		GivenNames: "Edward Richard George",
		FamilyName: "Heath",
		Address:    amlAddress}

	return amlProfile
}

func TestYotiClient_PerformAmlCheck_WithInvalidJSON(t *testing.T) {
	key, err := getValidKey()
	assert.NilError(t, err)

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(strings.NewReader("Not a JSON document")),
				}, nil
			},
		},
		Key: key,
	}

	_, err = client.PerformAmlCheck(createStandardAmlProfile())
	assert.Check(t, strings.Contains(err.Error(), "invalid character"))
}

func TestYotiClient_PerformAmlCheck_Success(t *testing.T) {
	key, err := getValidKey()
	assert.NilError(t, err)

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(strings.NewReader(`{"on_fraud_list":true,"on_pep_list":true,"on_watch_list":true}`)),
				}, nil
			},
		},
		Key: key,
	}

	result, err := client.PerformAmlCheck(createStandardAmlProfile())

	assert.Assert(t, is.Nil(err))

	assert.Check(t, result.OnFraudList)
	assert.Check(t, result.OnPEPList)
	assert.Check(t, result.OnWatchList)

}

func TestYotiClient_PerformAmlCheck_Unsuccessful(t *testing.T) {
	key, err := getValidKey()
	assert.NilError(t, err)

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 503,
					Body:       ioutil.NopCloser(strings.NewReader(`SERVICE UNAVAILABLE - Unable to reach the Integrity Service`)),
				}, nil
			},
		},
		Key: key,
	}

	_, err = client.PerformAmlCheck(createStandardAmlProfile())

	var expectedErrString = "AML Check was unsuccessful"

	assert.Assert(t, err != nil)
	assert.Check(t, strings.HasPrefix(err.Error(), expectedErrString))
	tempError, temporary := err.(interface {
		Temporary() bool
	})
	assert.Check(t, temporary && tempError.Temporary())
}

func TestAttributeImage_Image_Png(t *testing.T) {
	attributeName := consts.AttrSelfie
	byteValue := []byte("value")

	var attributeImage = &yotiprotoattr.Attribute{
		Name:        attributeName,
		Value:       byteValue,
		ContentType: yotiprotoattr.ContentType_PNG,
		Anchors:     []*yotiprotoattr.Anchor{},
	}

	result := createProfileWithSingleAttribute(attributeImage)
	selfie := result.Selfie()

	assert.DeepEqual(t, selfie.Value().Data, byteValue)
}

func TestAttributeImage_Image_Jpeg(t *testing.T) {
	attributeName := consts.AttrSelfie
	byteValue := []byte("value")

	var attributeImage = &yotiprotoattr.Attribute{
		Name:        attributeName,
		Value:       byteValue,
		ContentType: yotiprotoattr.ContentType_JPEG,
		Anchors:     []*yotiprotoattr.Anchor{},
	}

	result := createProfileWithSingleAttribute(attributeImage)
	selfie := result.Selfie()

	assert.DeepEqual(t, selfie.Value().Data, byteValue)
}

func TestAttributeImage_Image_Default(t *testing.T) {
	attributeName := consts.AttrSelfie
	byteValue := []byte("value")

	var attributeImage = &yotiprotoattr.Attribute{
		Name:        attributeName,
		Value:       byteValue,
		ContentType: yotiprotoattr.ContentType_PNG,
		Anchors:     []*yotiprotoattr.Anchor{},
	}
	result := createProfileWithSingleAttribute(attributeImage)
	selfie := result.Selfie()

	assert.DeepEqual(t, selfie.Value().Data, byteValue)
}
func TestAttributeImage_Base64Selfie_Png(t *testing.T) {
	attributeName := consts.AttrSelfie
	imageBytes := []byte("value")

	var attributeImage = &yotiprotoattr.Attribute{
		Name:        attributeName,
		Value:       imageBytes,
		ContentType: yotiprotoattr.ContentType_PNG,
		Anchors:     []*yotiprotoattr.Anchor{},
	}

	result := createProfileWithSingleAttribute(attributeImage)

	base64ImageExpectedValue := base64.StdEncoding.EncodeToString(imageBytes)

	expectedBase64Selfie := "data:image/png;base64," + base64ImageExpectedValue

	base64Selfie := result.Selfie().Value().Base64URL()

	assert.Equal(t, base64Selfie, expectedBase64Selfie)
}

func TestAttributeImage_Base64URL_Jpeg(t *testing.T) {
	attributeName := consts.AttrSelfie
	imageBytes := []byte("value")

	var attributeImage = &yotiprotoattr.Attribute{
		Name:        attributeName,
		Value:       imageBytes,
		ContentType: yotiprotoattr.ContentType_JPEG,
		Anchors:     []*yotiprotoattr.Anchor{},
	}

	result := createProfileWithSingleAttribute(attributeImage)

	base64ImageExpectedValue := base64.StdEncoding.EncodeToString(imageBytes)

	expectedBase64Selfie := "data:image/jpeg;base64," + base64ImageExpectedValue

	base64Selfie := result.Selfie().Value().Base64URL()

	assert.Equal(t, base64Selfie, expectedBase64Selfie)
}

func ExampleClient_CreateShareURL() {
	key, _ := getValidKey()

	client := Client{
		HTTPClient: &mockHTTPClient{
			do: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 201,
					Body:       ioutil.NopCloser(strings.NewReader(`{"qrcode":"https://code.yoti.com/CAEaJDQzNzllZDc0LTU0YjItNDkxMy04OTE4LTExYzM2ZDU2OTU3ZDAC","ref_id":"0"}`)),
				}, nil
			},
		},
		SdkID: "someSdkId",
		Key:   key,
	}

	policy, err := (&dynamic_sharing_service.DynamicPolicyBuilder{}).WithFullName().WithWantedRememberMe().Build()
	if err != nil {
		return
	}
	scenario, err := (&dynamic_sharing_service.DynamicScenarioBuilder{}).WithPolicy(policy).Build()
	if err != nil {
		return
	}

	result, err := client.CreateShareURL(&scenario)
	if err != nil {
		return
	}
	fmt.Printf("QR code: %s", result.ShareURL)
	// Output: QR code: https://code.yoti.com/CAEaJDQzNzllZDc0LTU0YjItNDkxMy04OTE4LTExYzM2ZDU2OTU3ZDAC
}

func createProfileWithSingleAttribute(attr *yotiprotoattr.Attribute) profile.Profile {
	var attributeSlice []*yotiprotoattr.Attribute
	attributeSlice = append(attributeSlice, attr)

	attributeList := &yotiprotoattr.AttributeList{
		Attributes: attributeSlice,
	}

	return profile.NewUserProfile(attributeList)
}

func getValidKey() (*rsa.PrivateKey, error) {
	keyBytes, err := ioutil.ReadFile("test/test-key.pem")
	if err != nil {
		return nil, err
	}

	return cryptoutil.ParseRSAKey(keyBytes)
}
