package s3cache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	testAK     = "AKIDTEST"
	testSK     = "secret/with+chars"
	testHost   = "cache.example.com:9000"
	testRegion = "us-east-1"
)

var testNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

// presign builds a SigV4 query-signed URL path for tests, written independently of canonicalRequest.
func presign(t *testing.T, method, path string, headers http.Header, signedAt time.Time, expires int) string {
	t.Helper()
	amzDate := signedAt.Format(amzDateLayout)
	scope := amzDate[:8] + "/" + testRegion + "/s3/aws4_request"

	names := []string{"host"}
	for k := range headers {
		names = append(names, strings.ToLower(k))
	}
	sort.Strings(names)
	signed := strings.Join(names, ";")

	q := url.Values{}
	q.Set("X-Amz-Algorithm", sigAlgorithm)
	q.Set("X-Amz-Credential", testAK+"/"+scope)
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", strconv.Itoa(expires))
	q.Set("X-Amz-SignedHeaders", signed)
	query := strings.ReplaceAll(q.Encode(), "+", "%20")

	var canon strings.Builder
	canon.WriteString(method + "\n" + encodePath(path) + "\n" + query + "\n")
	for _, n := range names {
		v := testHost
		if n != "host" {
			v = headers.Get(n)
		}
		canon.WriteString(n + ":" + v + "\n")
	}
	canon.WriteString("\n" + signed + "\n" + unsignedPayload)

	sum := sha256.Sum256([]byte(canon.String()))
	sts := sigAlgorithm + "\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	sig := hex.EncodeToString(hmacSHA256(signingKey(testSK, amzDate[:8], testRegion, "s3"), sts))
	return encodePath(path) + "?" + query + "&X-Amz-Signature=" + sig
}

func newSignedRequest(method, target string, headers http.Header) *http.Request {
	r := httptest.NewRequest(method, "http://"+testHost+target, nil)
	for k, v := range headers {
		r.Header[k] = v
	}
	return r
}

// TestVerifyPresigned_AWSDocVector checks the GET example from the AWS
// "Authenticating Requests: Using Query Parameters" documentation.
func TestVerifyPresigned_AWSDocVector(t *testing.T) {
	target := "https://examplebucket.s3.amazonaws.com/test.txt" +
		"?X-Amz-Algorithm=AWS4-HMAC-SHA256" +
		"&X-Amz-Credential=AKIAIOSFODNN7EXAMPLE%2F20130524%2Fus-east-1%2Fs3%2Faws4_request" +
		"&X-Amz-Date=20130524T000000Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host" +
		"&X-Amz-Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404"
	r := httptest.NewRequest(http.MethodGet, target, nil)
	now := time.Date(2013, 5, 24, 1, 0, 0, 0, time.UTC)

	if e := verifyPresigned(r, http.MethodGet, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", now); e != nil {
		t.Fatalf("AWS doc vector rejected: %s: %s", e.Code, e.Message)
	}
}

func TestVerifyPresigned(t *testing.T) {
	const path = "/bucket/runner/x/project/1/key with space+plus"
	valid := presign(t, http.MethodGet, path, nil, testNow, 3600)
	putHeaders := http.Header{"Content-Type": {"application/octet-stream"}, "X-Amz-Meta-Cachekey": {"k"}}
	validPut := presign(t, http.MethodPut, path, putHeaders, testNow, 3600)

	tests := []struct {
		name     string
		method   string
		target   string
		headers  http.Header
		now      time.Time
		wantCode string // empty = accepted
	}{
		{"valid GET", http.MethodGet, valid, nil, testNow, ""},
		{"valid PUT with signed headers", http.MethodPut, validPut, putHeaders, testNow, ""},
		{"PUT signed header value changed", http.MethodPut, validPut,
			http.Header{"Content-Type": {"text/plain"}, "X-Amz-Meta-Cachekey": {"k"}}, testNow, "SignatureDoesNotMatch"},
		{"method mismatch", http.MethodPut, valid, nil, testNow, "SignatureDoesNotMatch"},
		{"path tampered", http.MethodGet, strings.Replace(valid, "bucket", "bucket2", 1), nil, testNow, "SignatureDoesNotMatch"},
		{"signature tampered", http.MethodGet, valid[:len(valid)-2] + "00", nil, testNow, "SignatureDoesNotMatch"},
		{"expired", http.MethodGet, valid, nil, testNow.Add(2 * time.Hour), "AccessDenied"},
		{"not yet valid", http.MethodGet, valid, nil, testNow.Add(-time.Hour), "AccessDenied"},
		{"within clock skew", http.MethodGet, valid, nil, testNow.Add(-10 * time.Minute), ""},
		{"no signature params", http.MethodGet, encodePath(path), nil, testNow, "AccessDenied"},
		{"wrong access key", http.MethodGet, strings.Replace(valid, testAK, "OTHER", 1), nil, testNow, "InvalidAccessKeyId"},
		{"malformed credential", http.MethodGet, strings.Replace(valid, "%2Fs3%2F", "%2Fec2%2F", 1), nil, testNow, "AuthorizationQueryParametersError"},
		{"credential date mismatch", http.MethodGet, strings.Replace(valid, "%2F20261019%2F", "%2F20261018%2F", 1), nil, testNow, "AuthorizationQueryParametersError"},
		{"bad date", http.MethodGet, strings.Replace(valid, "X-Amz-Date=20261019T120000Z", "X-Amz-Date=nope", 1), nil, testNow, "AuthorizationQueryParametersError"},
		{"expires too long", http.MethodGet, strings.Replace(valid, "X-Amz-Expires=3600", "X-Amz-Expires=604801", 1), nil, testNow, "AuthorizationQueryParametersError"},
		{"expires not a number", http.MethodGet, strings.Replace(valid, "X-Amz-Expires=3600", "X-Amz-Expires=x", 1), nil, testNow, "AuthorizationQueryParametersError"},
		{"host not signed", http.MethodGet, strings.Replace(valid, "X-Amz-SignedHeaders=host", "X-Amz-SignedHeaders=range", 1), nil, testNow, "AuthorizationQueryParametersError"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newSignedRequest(tt.method, tt.target, tt.headers)
			e := verifyPresigned(r, tt.method, testAK, testSK, tt.now)
			switch {
			case tt.wantCode == "" && e != nil:
				t.Fatalf("expected accept, got %s: %s", e.Code, e.Message)
			case tt.wantCode != "" && e == nil:
				t.Fatalf("expected %s, got accept", tt.wantCode)
			case tt.wantCode != "" && e.Code != tt.wantCode:
				t.Fatalf("expected %s, got %s: %s", tt.wantCode, e.Code, e.Message)
			}
		})
	}
}

func TestCanonicalHeaderValueFoldsWhitespace(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://h/", nil)
	r.Header.Add("X-Amz-Meta-A", "  one   two ")
	r.Header.Add("X-Amz-Meta-A", "three")
	if got := canonicalHeaderValue(r, "x-amz-meta-a"); got != "one two,three" {
		t.Fatalf("got %q", got)
	}
}

func TestEncodePath(t *testing.T) {
	tests := map[string]string{
		"":                  "/",
		"/b/k":              "/b/k",
		"/b/a b+c":          "/b/a%20b%2Bc",
		"/b/~_-.ok":         "/b/~_-.ok",
		"/b/한":              "/b/%ED%95%9C",
		"/b/semi;colon=eq?": "/b/semi%3Bcolon%3Deq%3F",
	}
	for in, want := range tests {
		if got := encodePath(in); got != want {
			t.Errorf("encodePath(%q) = %q, want %q", in, got, want)
		}
	}
}
