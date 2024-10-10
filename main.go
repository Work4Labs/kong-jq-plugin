package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Kong/go-pdk"
	"github.com/Kong/go-pdk/server"
	"github.com/itchyny/gojq"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

var (
	Version  = "0.1"
	Priority = 1
)

var (
	ErrorResponseBodyResult = "response body jq doesn't return any result"
	ErrorResponseBody       = "response body jq error"
	ErrorStatusCodeResult   = "status code jq doesn't return any result"
)

var (
	ErrorStatusCode        = "status code jq error"
	ErrorStatusCodeInteger = "status code jq result is not an integer"
)

var (
	ErrorHeadersResult = "response headers jq doesn't return any result"
	ErrorHeaders       = "response headers jq error"
	ErrorHeadersMap    = "response headers jq result is not a map"
)

var (
	ErrorQueryParamsResult = "query params jq doesn't return any result"
	ErrorQueryParams       = "query params jq error"
	ErrorQueryParamsMap    = "query params jq result is not a map"
)

var (
	ErrorPathResult = "path jq doesn't return any result"
	ErrorPath       = "path jq error"
	ErrorPathString = "path jq result is not a string"
)

var (
	ErrorMethodResult = "method jq doesn't return any result"
	ErrorMethod       = "method jq error"
	ErrorMethodString = "method jq result is not a string"
)

var loggerKey = "logger"

func ContextWithLog(ctx context.Context, fields logrus.Fields) (context.Context, *logrus.Entry) {
	logger := logrus.StandardLogger().WithFields(logrus.Fields{})

	if previousLogger, ok := ctx.Value(loggerKey).(*logrus.Entry); ok {
		logger = previousLogger
	}
	logger = logger.WithFields(fields)

	return context.WithValue(ctx, loggerKey, logger), logger
}

func main() {
	lo.Must0(server.StartServer(New, Version, Priority))
}

type Config struct {
	Method         string // an optional jq query that returns a string to override the method (GET/POST/PUT/DELETE/PATCH)
	Path           string // an optional jq query that returns a string to override the uri
	QueryParams    string // jq query that returns an object of list of values
	RequestHeaders string // jq query that returns an object of list of values
	RequestBody    string // jq query that returns a string reprensenting the new uri

	ResponseHeaders string // jq query that returns an object of list of values
	ResponseBody    string // an optional jq query that returns a string that will override the response body
	StatusCode      string // an optional jq query returning an integer that will override the status code
}

func New() interface{} {
	return &Config{}
}

func (conf Config) Access(kong *pdk.PDK) {
	ctx, logger := ContextWithLog(context.Background(), logrus.Fields{
		"app":    "kong-jq",
		"method": lo.Must(kong.Request.GetMethod()),
		"path":   lo.Must(kong.Request.GetPath()),
	})

	headers := lo.Must(kong.Request.GetHeaders(-1))

	args, kwargs := lo.Must2(kong.Request.GetUriCaptures())

	queryParams := lo.MapValues(
		lo.Must(kong.Request.GetQuery(-1)),
		func(s []string, _ string) any {
			return lo.Map(
				s,
				func(s string, _ int) any {
					return s
				},
			)
		},
	)

	arguments := map[string]any{
		"request": map[string]any{
			"method":       lo.Must(kong.Request.GetMethod()),
			"path":         lo.Must(kong.Request.GetPath()),
			"args":         lo.Map(args, func(s []byte, _ int) any { return string(s) }),
			"kwargs":       lo.MapValues(kwargs, func(s []byte, _ string) any { return string(s) }),
			"query_params": queryParams,
			"headers": lo.MapValues(
				headers,
				func(s []string, _ string) any {
					return lo.Map(
						s,
						func(s string, _ int) any {
							return s
						},
					)
				},
			),
		},
	}

	// logger = logger.WithField(
	// 	"arguments",
	// 	spew.Sdump(arguments),
	// )

	if conf.Method != "" {
		jqMethod := lo.Must(gojq.Parse(conf.Method))

		iter := jqMethod.RunWithContext(ctx, arguments)

		next, ok := iter.Next()
		if !ok {
			logger.Error(ErrorMethodResult)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorMethodResult), map[string][]string{})

			return
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorMethod)
			kong.Response.Exit(
				http.StatusInternalServerError,
				[]byte(fmt.Sprintf("%s: %+v", ErrorMethod, err)),
				map[string][]string{},
			)

			return
		}

		newMethod, ok := next.(string)
		if !ok {
			logger.Error(ErrorMethodString)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorMethodString), map[string][]string{})

			return
		}

		arguments["request"].(map[string]any)["method"] = newMethod
		lo.Must0(kong.ServiceRequest.SetMethod(newMethod))
	}

	if conf.Path != "" {
		jqPath := lo.Must(gojq.Parse(conf.Path))

		iter := jqPath.RunWithContext(ctx, arguments)

		next, ok := iter.Next()
		if !ok {
			logger.Error(ErrorPathResult)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorPathResult), map[string][]string{})

			return
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorPath)
			kong.Response.Exit(
				http.StatusInternalServerError,
				[]byte(fmt.Sprintf("%s: %+v", ErrorPath, err)),
				map[string][]string{},
			)

			return
		}

		newPath, ok := next.(string)
		if !ok {
			logger.Error(ErrorPathString)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorPathString), map[string][]string{})

			return
		}

		arguments["request"].(map[string]any)["path"] = newPath
		lo.Must0(kong.ServiceRequest.SetPath(newPath))
	}

	if conf.QueryParams != "" {
		jqQueryParams := lo.Must(gojq.Parse(conf.QueryParams))

		iter := jqQueryParams.RunWithContext(ctx, arguments)

		next, ok := iter.Next()
		if !ok {
			logger.Error(ErrorQueryParamsResult)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorQueryParamsResult), map[string][]string{})

			return
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorQueryParams)
			kong.Response.Exit(
				http.StatusInternalServerError,
				[]byte(fmt.Sprintf("%s: %+v", ErrorQueryParams, err)),
				map[string][]string{},
			)

			return
		}

		newQueryParams, ok := next.(map[string]any) // jq results are forced to be map[string]any
		if !ok {
			logger.Error(ErrorQueryParamsMap)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorQueryParamsMap), map[string][]string{})

			return
		}

		arguments["request"].(map[string]any)["query_params"] = newQueryParams
		lo.Must0( // we want to bypass errors
			kong.ServiceRequest.SetQuery( // we will set new query params
				lo.MapValues( // we want to transform all values of query params that are []any at this time into []string
					newQueryParams,
					func(v any, _ string) []string {
						return lo.Map( // we want to transform the []any into []string, currently the []any is represented as any above
							v.([]any), // jq presents us a any type with we should be able to convert to []any
							func(s any, _ int) string { // this any should be transformable into string
								return s.(string)
							},
						)
					},
				),
			),
		)
	} else {
		lo.Must0(kong.ServiceRequest.SetQuery(map[string][]string{}))
	}

	allRequestHeaders := lo.Must(kong.Request.GetHeaders(-1))
	for k := range allRequestHeaders {
		kong.ServiceRequest.ClearHeader(k)
	}

	if conf.RequestHeaders != "" {
		jqRequestHeaders := lo.Must(gojq.Parse(conf.RequestHeaders))

		iter := jqRequestHeaders.RunWithContext(ctx, arguments)

		next, ok := iter.Next()
		if !ok {
			logger.Error(ErrorHeadersResult)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorHeadersResult), map[string][]string{})

			return
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorHeaders)
			kong.Response.Exit(
				http.StatusInternalServerError,
				[]byte(fmt.Sprintf("%s: %+v", ErrorHeaders, err)),
				map[string][]string{},
			)

			return
		}

		newRequestHeaders, ok := next.(map[string]any)
		if !ok {
			logger.Error(ErrorHeadersMap)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorHeadersMap), map[string][]string{})

			return
		}

		for k, v := range newRequestHeaders {
			logger := logger.WithFields(logrus.Fields{
				"header key":   k,
				"header value": v,
			})

			if values, ok := v.([]any); ok {
				for _, value := range values {
					if value, ok := value.(string); ok {
						lo.Must0(kong.ServiceRequest.AddHeader(k, value))
					} else {
						logger.Error("header value is not a string")
						kong.Response.Exit(http.StatusInternalServerError, []byte("header value is not a string"), map[string][]string{})

						return
					}
				}
			} else {
				if value, ok := v.(string); ok {
					lo.Must0(kong.ServiceRequest.SetHeader(k, value))
				} else {
					logger.Error("header value is not a string or a list of strings")
					kong.Response.Exit(http.StatusInternalServerError, []byte("header value is not a string or a list of strings"), map[string][]string{})

					return
				}
			}
		}
	}
}

func (conf Config) Response(kong *pdk.PDK) {
	ctx, logger := ContextWithLog(context.Background(), logrus.Fields{
		"app":    "kong-jq",
		"method": lo.Must(kong.Request.GetMethod()),
		"path":   lo.Must(kong.Request.GetPath()),
	})

	args, kwargs := lo.Must2(kong.Request.GetUriCaptures())

	queryParams := lo.MapValues(
		lo.Must(kong.Request.GetQuery(-1)),
		func(s []string, _ string) any {
			return lo.Map(
				s,
				func(s string, _ int) any {
					return s
				},
			)
		},
	)

	allResponseHeaders, err := kong.Response.GetHeaders(-1)
	if err != nil {
		logger.WithError(err).Error("failed to get all response headers")
		kong.Response.Exit(
			http.StatusInternalServerError,
			[]byte("failed to get all response headers"),
			map[string][]string{},
		)

		return
	}

	logger.WithField("response_headers_count", len(allResponseHeaders)).Info("clearing response headers…")

	for k := range allResponseHeaders {
		logger.WithField("header", k).Info("clearing header")

		if err := kong.Response.ClearHeader(k); err != nil {
			logger.WithError(err).Error("failed to clear header")
			kong.Response.Exit(http.StatusInternalServerError, []byte("failed to clear header"), map[string][]string{})

			return
		}
	}

	statusCode := lo.Must(kong.ServiceResponse.GetStatus())
	body := lo.Must(kong.ServiceResponse.GetRawBody())

	arguments := map[string]any{
		"request": map[string]any{
			"method":       lo.Must(kong.Request.GetMethod()),
			"path":         lo.Must(kong.Request.GetPath()),
			"args":         lo.Map(args, func(s []byte, _ int) any { return string(s) }),
			"kwargs":       lo.MapValues(kwargs, func(s []byte, _ string) any { return string(s) }),
			"query_params": queryParams,
		},
		"response": map[string]any{
			"headers":     map[string]any{},
			"body":        string(body),
			"status_code": statusCode,
		},
	}

	headers := map[string][]string{}

	if conf.ResponseHeaders != "" {
		jqResponseHeaders := lo.Must(gojq.Parse(conf.ResponseHeaders))
		iter := jqResponseHeaders.RunWithContext(ctx, arguments)

		next, ok := iter.Next()
		if !ok {
			logger.Error(ErrorHeadersResult)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorHeadersResult), map[string][]string{})

			return
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorHeaders)
			kong.Response.Exit(
				http.StatusInternalServerError,
				[]byte(fmt.Sprintf("%s: %+v", ErrorHeaders, err)),
				map[string][]string{},
			)

			return
		}

		newResponseHeaders, ok := next.(map[string]any)
		if !ok {
			logger.Error(ErrorHeadersMap)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorHeadersMap), map[string][]string{})

			return
		}

		headers = lo.MapValues(
			newResponseHeaders,
			func(s any, _ string) []string {
				return lo.Map(
					s.([]any),
					func(s any, _ int) string {
						return s.(string)
					},
				)
			},
		)
	}

	if conf.StatusCode != "" {
		jqStatusCode := lo.Must(gojq.Parse(conf.StatusCode))
		iter := jqStatusCode.RunWithContext(ctx, arguments)

		next, ok := iter.Next()
		if !ok {
			logger.Error(ErrorStatusCodeResult)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorStatusCodeResult), map[string][]string{})

			return
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorStatusCode)
			kong.Response.Exit(
				http.StatusInternalServerError,
				[]byte(fmt.Sprintf("%s: %+v", ErrorStatusCode, err)),
				map[string][]string{},
			)

			return
		}

		newStatusCode, ok := next.(int)
		if !ok {
			logger.Error(ErrorStatusCodeInteger)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorStatusCodeInteger), map[string][]string{})

			return
		}

		statusCode = newStatusCode
	}

	if conf.ResponseBody != "" {
		jqResponseBody := lo.Must(gojq.Parse(conf.ResponseBody))

		iter := jqResponseBody.RunWithContext(ctx, arguments)

		next, ok := iter.Next()
		if !ok {
			logger.Error(ErrorResponseBodyResult)
			kong.Response.Exit(http.StatusInternalServerError, []byte(ErrorResponseBodyResult), map[string][]string{})

			return
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorResponseBody)
			kong.Response.Exit(
				http.StatusInternalServerError,
				[]byte(fmt.Sprintf("%s: %+v", ErrorResponseBody, err)),
				map[string][]string{},
			)

			return
		}

		var err error

		body, err = json.Marshal(next)
		if err != nil {
			logger.WithError(err).Error(ErrorResponseBody)
			kong.Response.Exit(
				http.StatusInternalServerError,
				[]byte(fmt.Sprintf("%s: %+v", ErrorResponseBody, err)),
				map[string][]string{},
			)

			return
		}
	}

	kong.Response.Exit(statusCode, body, headers)
}
