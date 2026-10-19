package s3cache

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	sigAlgorithm    = "AWS4-HMAC-SHA256"
	unsignedPayload = "UNSIGNED-PAYLOAD"
	amzDateLayout   = "20060102T150405Z"
	maxPresignTTL   = 7 * 24 * time.Hour
	maxClockSkew    = 15 * time.Minute
)

// verifyPresigned checks an AWS Signature Version 4 query-string (presigned)
// request against the configured credentials. It returns nil on success.
func verifyPresigned(r *http.Request, method, accessKey, secretKey string, now time.Time) *s3Error {
	q := r.URL.Query()

	if q.Get("X-Amz-Algorithm") != sigAlgorithm {
		return errAccessDenied("presigned AWS4-HMAC-SHA256 URL required")
	}

	credParts := strings.Split(q.Get("X-Amz-Credential"), "/")
	if len(credParts) != 5 || credParts[3] != "s3" || credParts[4] != "aws4_request" {
		return errAuthMalformed("malformed X-Amz-Credential")
	}
	if subtle.ConstantTimeCompare([]byte(credParts[0]), []byte(accessKey)) != 1 {
		return errInvalidAccessKey()
	}

	amzDate := q.Get("X-Amz-Date")
	signedAt, err := time.Parse(amzDateLayout, amzDate)
	if err != nil || credParts[1] != amzDate[:8] {
		return errAuthMalformed("malformed X-Amz-Date")
	}

	expSec, err := strconv.Atoi(q.Get("X-Amz-Expires"))
	if err != nil || expSec <= 0 || time.Duration(expSec)*time.Second > maxPresignTTL {
		return errAuthMalformed("X-Amz-Expires must be between 1 and 604800 seconds")
	}
	if now.Before(signedAt.Add(-maxClockSkew)) {
		return errAccessDenied("request is not yet valid")
	}
	if now.After(signedAt.Add(time.Duration(expSec) * time.Second)) {
		return errAccessDenied("request has expired")
	}

	signedHeaders := q.Get("X-Amz-SignedHeaders")
	if !slices.Contains(strings.Split(signedHeaders, ";"), "host") {
		return errAuthMalformed("host must be a signed header")
	}

	scope := strings.Join(credParts[1:], "/")
	canonical := canonicalRequest(r, method, q, signedHeaders)
	sum := sha256.Sum256([]byte(canonical))
	stringToSign := sigAlgorithm + "\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])

	key := signingKey(secretKey, credParts[1], credParts[2], credParts[3])
	expected := hex.EncodeToString(hmacSHA256(key, stringToSign))

	if !hmac.Equal([]byte(expected), []byte(q.Get("X-Amz-Signature"))) {
		return errSignatureMismatch()
	}
	return nil
}

func canonicalRequest(r *http.Request, method string, q url.Values, signedHeaders string) string {
	query := url.Values{}
	for k, v := range q {
		if k != "X-Amz-Signature" {
			query[k] = v
		}
	}

	var b strings.Builder
	b.WriteString(method)
	b.WriteByte('\n')
	b.WriteString(encodePath(r.URL.Path))
	b.WriteByte('\n')
	// Values.Encode sorts keys and escapes like AWS except for spaces.
	b.WriteString(strings.ReplaceAll(query.Encode(), "+", "%20"))
	b.WriteByte('\n')
	for _, h := range strings.Split(signedHeaders, ";") {
		b.WriteString(h)
		b.WriteByte(':')
		b.WriteString(canonicalHeaderValue(r, h))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(signedHeaders)
	b.WriteByte('\n')
	b.WriteString(unsignedPayload)
	return b.String()
}

func canonicalHeaderValue(r *http.Request, name string) string {
	if name == "host" {
		return r.Host
	}
	vals := r.Header.Values(name)
	trimmed := make([]string, len(vals))
	for i, v := range vals {
		trimmed[i] = strings.Join(strings.Fields(v), " ")
	}
	return strings.Join(trimmed, ",")
}

// encodePath URI-encodes every byte outside the AWS unreserved set, keeping '/'.
func encodePath(p string) string {
	if p == "" {
		return "/"
	}
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		c := p[i]
		if isUnreserved(c) || c == '/' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexDigits[c>>4])
		b.WriteByte(hexDigits[c&0x0f])
	}
	return b.String()
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '~'
}

func signingKey(secret, date, region, service string) []byte {
	k := hmacSHA256([]byte("AWS4"+secret), date)
	k = hmacSHA256(k, region)
	k = hmacSHA256(k, service)
	return hmacSHA256(k, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}
