// Package query support parsing of JSON API based query parameters.
// It supports the following parameters
//		include - /url?include=foo,bar,baz
//		fields(sparse fieldsets) - /url?fields[articles]=title,body&fields[people]=name
//		filter - /url?filter[name]=foo&filter[country]=argentina
// The include and fields are part of JSON API whereas filter is a custom
// extension for dictybase. For details look here
// https://github.com/json-api/json-api/blob/9c7a03dbc37f80f6ca81b16d444c960e96dd7a57/extensions/index.md#-extension-negotiation
// and here
// https://github.com/dictyBase/Migration/blob/master/Webservice-specs.md#filtering
// This middleware terminates the chain in case of incorrect or inappropriate
// http headers for filter query parameters.
package query

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/manyminds/api2go"
)

type contextKey string

// String output the details of context key
func (c contextKey) String() string {
	return "pagination context key " + string(c)
}

var (
	// ContextKeyQueryParams is the key used for stroing Params struct in
	// request context
	ContextKeyQueryParams = contextKey("jsparams")
	acceptH               = http.CanonicalHeaderKey("accept")
	contentType           = http.CanonicalHeaderKey("content-type")
	filterMediaType       = strconv.Quote(`application/vnd.api+json; supported-ext="dictybase/filtering-resouce"`)
	qregx                 = regexp.MustCompile(`^\w+\[(\w+)\]$`)
)

// Params is container for various query parameters
type Params struct {
	// contain include query paramters
	Includes []string
	// contain fields query paramters
	Fields map[string][]string
	// contain filter query parameters
	Filters map[string]string
	// check for presence of fields parameters
	HasFields bool
	// check for presence of include parameters
	HasIncludes bool
	// check for presence of filter parameters
	HasFilters bool
}

func newParams() *Params {
	return &Params{
		Fields:  make(map[string][]string),
		Filters: make(map[string]string),
	}
}

// MiddlewareFn parses the includes, fields and filter query strings and stores
// it in request context under  ContextKeyQueryParam variable as a Params type
// For filter query parameters, the client should include the appropiate media
// type and media type parameters as described here
// https://github.com/dictyBase/Migration/blob/master/Webservice-specs.md#dictybase-specifications.
// Otherwise, the request never gets passed to the handler and either of
// 406(Not Acceptable) or 415(Unsupported Media Type) http status is returned.
func MiddlewareFn(fn http.HandlerFunc) http.HandlerFunc {
	newFn := func(w http.ResponseWriter, r *http.Request) {
		params := newParams()
		values := r.URL.Query()
		for k, v := range values {
			switch {
			case strings.HasPrefix(k, "filter"):
				// check for correct header
				if !validateHeader(w, r) {
					return
				}
				if m := qregx.FindStringSubmatch(k); m != nil {
					params.Filters[m[1]] = v[0]
					if !params.HasFilters {
						params.HasFilters = true
					}
				} else {
					queryParamError(
						w,
						http.StatusBadRequest,
						"Invalid query parameter",
						fmt.Sprintf("Unable to match filter query param %s", v[0]),
					)
					return
				}
			case strings.HasPrefix(k, "fields"):
				if m := qregx.FindStringSubmatch(k); m != nil {
					if strings.Contains(v[0], ",") {
						params.Fields[m[1]] = strings.Split(v[0], ",")
					} else {
						params.Fields[m[1]] = []string{v[0]}
					}
					if !params.HasFields {
						params.HasFields = true
					}
				} else {
					queryParamError(
						w,
						http.StatusBadRequest,
						"Invalid query parameter",
						fmt.Sprintf("Unable to match fields query param %s", v[0]),
					)
					return
				}
			case k == "include":
				if strings.Contains(v[0], ",") {
					params.Includes = strings.Split(v[0], ",")
				} else {
					params.Includes = []string{v[0]}
				}
				if !params.HasIncludes {
					params.HasIncludes = true
				}
			default:
				continue
			}
		}
		if params.HasFilters || params.HasFields || params.HasIncludes {
			ctx := context.WithValue(r.Context(), ContextKeyQueryParams, params)
			fn(w, r.WithContext(ctx))
		} else {
			fn(w, r)
		}
	}
	return newFn
}

func queryParamError(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/vnd.api+json")
	w.WriteHeader(status)
	jsnErr := api2go.Error{
		Status: strconv.Itoa(status),
		Title:  title,
		Detail: detail,
		Meta: map[string]interface{}{
			"creator": "query middleware",
		},
	}
	err := json.NewEncoder(w).Encode(api2go.HTTPError{Errors: []api2go.Error{jsnErr}})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func validateHeader(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get(acceptH) != filterMediaType {
		queryParamError(
			w,
			http.StatusNotAcceptable,
			"Accept header is not acceptable",
			fmt.Sprintf(
				"The given Accept header value %s is incorrect for filter query extension",
				r.Header.Get(acceptH),
			),
		)
		return false
	}
	if r.Header.Get(acceptH) != r.Header.Get(contentType) {
		queryParamError(
			w,
			http.StatusUnsupportedMediaType,
			"Media type is not supported",
			fmt.Sprintf(
				"The given media type %s in Content-Type header is not supported",
				r.Header.Get(contentType),
			),
		)
		return false
	}
	return true
}