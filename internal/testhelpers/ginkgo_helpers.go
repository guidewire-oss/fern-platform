package testhelpers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/gomega"
)

// GinTestContext creates a test gin context for API testing
type GinTestContext struct {
	Context  *gin.Context
	Recorder *httptest.ResponseRecorder
}

// NewGinTestContext creates a new test context for Gin handlers
func NewGinTestContext(method, path string, body interface{}) *GinTestContext {
	gin.SetMode(gin.TestMode)
	
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	var bodyReader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		Expect(err).NotTo(HaveOccurred())
		bodyReader = bytes.NewReader(jsonBytes)
	} else {
		bodyReader = nil
	}
	
	c.Request = httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	
	return &GinTestContext{
		Context:  c,
		Recorder: w,
	}
}

// SetParams sets URL parameters
func (gtc *GinTestContext) SetParams(params gin.Params) {
	gtc.Context.Params = params
}

// SetQuery sets query parameters
func (gtc *GinTestContext) SetQuery(key, value string) {
	q := gtc.Context.Request.URL.Query()
	q.Add(key, value)
	gtc.Context.Request.URL.RawQuery = q.Encode()
}

// SetUser sets the authenticated user in the context
func (gtc *GinTestContext) SetUser(userID string) {
	gtc.Context.Set("user_id", userID)
}

// AssertStatus asserts the response status code
func (gtc *GinTestContext) AssertStatus(expected int) {
	Expect(gtc.Recorder.Code).To(Equal(expected))
}

// AssertJSONResponse asserts and returns the JSON response
func (gtc *GinTestContext) AssertJSONResponse(target interface{}) {
	Expect(gtc.Recorder.Header().Get("Content-Type")).To(ContainSubstring("application/json"))
	
	err := json.Unmarshal(gtc.Recorder.Body.Bytes(), target)
	Expect(err).NotTo(HaveOccurred())
}

// AssertErrorResponse asserts an error response with expected message
func (gtc *GinTestContext) AssertErrorResponse(expectedStatus int, expectedMessage string) {
	gtc.AssertStatus(expectedStatus)
	
	var response map[string]interface{}
	gtc.AssertJSONResponse(&response)
	
	Expect(response).To(HaveKey("error"))
	Expect(response["error"]).To(ContainSubstring(expectedMessage))
}

// HTTPTestClient creates a test HTTP client for integration testing
type HTTPTestClient struct {
	Client  *http.Client
	BaseURL string
	Token   string
}

// NewHTTPTestClient creates a new HTTP test client
func NewHTTPTestClient(baseURL string) *HTTPTestClient {
	return &HTTPTestClient{
		Client:  &http.Client{},
		BaseURL: baseURL,
	}
}

// SetAuthToken sets the authorization token
func (htc *HTTPTestClient) SetAuthToken(token string) {
	htc.Token = token
}

// Get performs a GET request
func (htc *HTTPTestClient) Get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", htc.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	
	if htc.Token != "" {
		req.Header.Set("Authorization", "Bearer "+htc.Token)
	}
	
	return htc.Client.Do(req)
}

// Post performs a POST request
func (htc *HTTPTestClient) Post(path string, body interface{}) (*http.Response, error) {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequest("POST", htc.BaseURL+path, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	if htc.Token != "" {
		req.Header.Set("Authorization", "Bearer "+htc.Token)
	}
	
	return htc.Client.Do(req)
}

// Put performs a PUT request
func (htc *HTTPTestClient) Put(path string, body interface{}) (*http.Response, error) {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequest("PUT", htc.BaseURL+path, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	if htc.Token != "" {
		req.Header.Set("Authorization", "Bearer "+htc.Token)
	}
	
	return htc.Client.Do(req)
}

// Delete performs a DELETE request
func (htc *HTTPTestClient) Delete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", htc.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	
	if htc.Token != "" {
		req.Header.Set("Authorization", "Bearer "+htc.Token)
	}
	
	return htc.Client.Do(req)
}

// AssertResponse asserts the response and decodes JSON
func (htc *HTTPTestClient) AssertResponse(resp *http.Response, expectedStatus int, target interface{}) {
	Expect(resp.StatusCode).To(Equal(expectedStatus))
	
	if target != nil {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		
		err = json.Unmarshal(body, target)
		Expect(err).NotTo(HaveOccurred())
	}
}