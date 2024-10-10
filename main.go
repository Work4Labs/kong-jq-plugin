package main

import (
	"context"
	"encoding/json"
	"fmt"

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

var ErrorResponseBodyResult = "response body jq doesn't return any result"
var ErrorResponseBody = "response body jq error"
var ErrorStatusCodeResult = "status code jq doesn't return any result"

var ErrorStatusCode = "status code jq error"
var ErrorStatusCodeInteger = "status code jq result is not an integer"

var ErrorHeadersResult = "response headers jq doesn't return any result"
var ErrorHeaders = "response headers jq error"
var ErrorHeadersMap = "response headers jq result is not a map"

var ErrorQueryParamsResult = "query params jq doesn't return any result"
var ErrorQueryParams = "query params jq error"
var ErrorQueryParamsMap = "query params jq result is not a map"

var ErrorPathResult = "path jq doesn't return any result"
var ErrorPath = "path jq error"
var ErrorPathString = "path jq result is not a string"

var ErrorMethodResult = "method jq doesn't return any result"
var ErrorMethod = "method jq error"
var ErrorMethodString = "method jq result is not a string"

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

func ContextWithLog(ctx context.Context, fields logrus.Fields) (context.Context, *logrus.Entry) {
	logger := logrus.StandardLogger().WithFields(logrus.Fields{})

	if previousLogger, ok := ctx.Value("logger").(*logrus.Entry); ok {
		logger = previousLogger
	}
	logger = logger.WithFields(fields)

	return context.WithValue(ctx, "logger", logger), logger
}

func main() {
	lo.Must0(server.StartServer(New, Version, Priority))
}

func New() interface{} {
	return &Config{}
}

func (conf Config) Access(kong *pdk.PDK) {

	ctx, logger := ContextWithLog(context.Background(), logrus.Fields{
		"app":    "kong-jq-plugins",
		"method": lo.Must(kong.Request.GetMethod()),
		"path":   lo.Must(kong.Request.GetPath()),
	})

	args, kwargs := lo.Must2(kong.Request.GetUriCaptures())

	var queryParams map[string]any = lo.MapValues(
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
		},
	}

	if conf.Method != "" {
		jqMethod := lo.Must(gojq.Parse(conf.Method))

		iter := jqMethod.RunWithContext(ctx, arguments)
		next, ok := iter.Next()
		if !ok {
			logger.Error(ErrorMethodResult)
			kong.Response.Exit(500, []byte(ErrorMethodResult), map[string][]string{})
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorMethod)
			kong.Response.Exit(500, []byte(fmt.Sprintf("%s: %+v", ErrorMethod, err)), map[string][]string{})
		}

		newMethod, ok := next.(string)
		if !ok {
			logger.Error(ErrorMethodString)
			kong.Response.Exit(500, []byte(ErrorMethodString), map[string][]string{})
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
			kong.Response.Exit(500, []byte(ErrorPathResult), map[string][]string{})
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorPath)
			kong.Response.Exit(500, []byte(fmt.Sprintf("%s: %+v", ErrorPath, err)), map[string][]string{})
		}

		newPath, ok := next.(string)
		if !ok {
			logger.Error(ErrorPathString)
			kong.Response.Exit(500, []byte(ErrorPathString), map[string][]string{})
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
			kong.Response.Exit(500, []byte(ErrorQueryParamsResult), map[string][]string{})
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorQueryParams)
			kong.Response.Exit(500, []byte(fmt.Sprintf("%s: %+v", ErrorQueryParams, err)), map[string][]string{})
		}

		newQueryParams, ok := next.(map[string]any) // jq results are forced to be map[string]any
		if !ok {
			logger.Error(ErrorQueryParamsMap)
			kong.Response.Exit(500, []byte(ErrorQueryParamsMap), map[string][]string{})
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

}

func (conf Config) Response(kong *pdk.PDK) {

	ctx, logger := ContextWithLog(context.Background(), logrus.Fields{
		"app":    "kong-jq-plugins",
		"method": lo.Must(kong.Request.GetMethod()),
		"path":   lo.Must(kong.Request.GetPath()),
	})

	args, kwargs := lo.Must2(kong.Request.GetUriCaptures())

	var queryParams map[string]any = lo.MapValues(
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
			kong.Response.Exit(500, []byte(ErrorHeadersResult), map[string][]string{})
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorHeaders)
			kong.Response.Exit(500, []byte(fmt.Sprintf("%s: %+v", ErrorHeaders, err)), map[string][]string{})
		}

		newResponseHeaders, ok := next.(map[string]any)
		if !ok {
			logger.Error(ErrorHeadersMap)
			kong.Response.Exit(500, []byte(ErrorHeadersMap), map[string][]string{})
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
			kong.Response.Exit(500, []byte(ErrorStatusCodeResult), map[string][]string{})
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorStatusCode)
			kong.Response.Exit(500, []byte(fmt.Sprintf("%s: %+v", ErrorStatusCode, err)), map[string][]string{})
		}

		newStatusCode, ok := next.(int)
		if !ok {
			logger.Error(ErrorStatusCodeInteger)
			kong.Response.Exit(500, []byte(ErrorStatusCodeInteger), map[string][]string{})
		}

		statusCode = newStatusCode
	}

	if conf.ResponseBody != "" {
		jqResponseBody := lo.Must(gojq.Parse(conf.ResponseBody))

		iter := jqResponseBody.RunWithContext(ctx, arguments)
		next, ok := iter.Next()
		if !ok {
			logger.Error(ErrorResponseBodyResult)
			kong.Response.Exit(500, []byte(ErrorResponseBodyResult), map[string][]string{})
		}

		if err, ok := next.(error); ok {
			logger.WithError(err).Error(ErrorResponseBody)
			kong.Response.Exit(500, []byte(fmt.Sprintf("%s: %+v", ErrorResponseBody, err)), map[string][]string{})
		}

		var err error
		body, err = json.Marshal(next)
		if err != nil {
			logger.WithError(err).Error(ErrorResponseBody)
			kong.Response.Exit(500, []byte(fmt.Sprintf("%s: %+v", ErrorResponseBody, err)), map[string][]string{})
		}
	}

	kong.Response.Exit(statusCode, body, headers)
}
