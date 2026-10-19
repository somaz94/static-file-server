package s3cache

import (
	"encoding/xml"
	"net/http"
)

// s3Error is an S3-style error response.
type s3Error struct {
	Status  int    `xml:"-"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

func errAccessDenied(msg string) *s3Error {
	return &s3Error{http.StatusForbidden, "AccessDenied", msg}
}

func errAuthMalformed(msg string) *s3Error {
	return &s3Error{http.StatusBadRequest, "AuthorizationQueryParametersError", msg}
}

func errInvalidAccessKey() *s3Error {
	return &s3Error{http.StatusForbidden, "InvalidAccessKeyId", "the access key ID does not exist"}
}

func errSignatureMismatch() *s3Error {
	return &s3Error{http.StatusForbidden, "SignatureDoesNotMatch", "the request signature does not match"}
}

func errNoSuchKey() *s3Error {
	return &s3Error{http.StatusNotFound, "NoSuchKey", "the specified key does not exist"}
}

func errNoSuchBucket() *s3Error {
	return &s3Error{http.StatusNotFound, "NoSuchBucket", "the specified bucket does not exist"}
}

func errInvalidRequest(msg string) *s3Error {
	return &s3Error{http.StatusBadRequest, "InvalidRequest", msg}
}

func errMissingContentLength() *s3Error {
	return &s3Error{http.StatusLengthRequired, "MissingContentLength", "Content-Length is required"}
}

func errEntityTooLarge() *s3Error {
	return &s3Error{http.StatusBadRequest, "EntityTooLarge", "object exceeds the maximum allowed size"}
}

func errIncompleteBody() *s3Error {
	return &s3Error{http.StatusBadRequest, "IncompleteBody", "body is shorter than Content-Length"}
}

func errKeyConflict() *s3Error {
	return &s3Error{http.StatusConflict, "KeyConflict", "key collides with an existing key prefix on the filesystem"}
}

func errNotImplemented() *s3Error {
	return &s3Error{http.StatusNotImplemented, "NotImplemented", "only GET, HEAD and PUT on objects are supported"}
}

func errInternal() *s3Error {
	return &s3Error{http.StatusInternalServerError, "InternalError", "internal error"}
}

func (e *s3Error) write(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		w.WriteHeader(e.Status)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(e.Status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(struct {
		XMLName xml.Name `xml:"Error"`
		*s3Error
	}{s3Error: e})
}
