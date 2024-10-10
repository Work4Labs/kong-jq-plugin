# Kong JQ Plugin

A custom Kong plugin written in Go that uses [JQ](https://stedolan.github.io/jq/) to modify requests and responses in real-time based on dynamic rules. This plugin allows for manipulation of HTTP methods, paths, query parameters, headers, request bodies, response bodies, and status codes.

## Features

- Modify HTTP method (GET, POST, PUT, DELETE, PATCH) dynamically based on the incoming request.
- Rewrite the request path dynamically.
- Modify query parameters based on JQ rules.
- Add or modify request and response headers.
- Manipulate the response body.
- Override the status code of responses.

## Requirements

- Kong \>= 2.0
- Go \>= 1.17

## Main dependencies

- [Kong Go PDK](https://github.com/Kong/go-pdk)
- [gojq](https://github.com/itchyny/gojq)

## Installation

To build and install this plugin, follow these steps:

1. Clone this repository:

```bash
git clone https://github.com/Work4Labs/kong-jq-plugin.git
cd kong-jq-plugin
```

2. Build the plugin:

```bash
go mod tidy
go build
```

3. Add the plugin to your Kong configuration:

```yaml
plugins:
  name: kong-jq-plugin
  config:
    method: '<JQ_QUERY>'
    path: '<JQ_QUERY>'
    query_params: '<JQ_QUERY>'
    request_headers: '<JQ_QUERY>'
    request_body: '<JQ_QUERY>'
    response_headers: '<JQ_QUERY>'
    response_body: '<JQ_QUERY>'
    status_code: '<JQ_QUERY>'
```

4. Add the built plugin to your Kong instance and reload the Kong configuration:

```bash
export KONG_PLUGINS=bundled,kong-jq-plugin
kong reload
```

## Configuration

You can use JQ to define how requests and responses are modified. The following fields are configurable:

| Field            | Type   | Description |
|------------------|--------|-------------|
| `method`         | string | A JQ query that returns a string to override the HTTP method. |
| `path`           | string | A JQ query that returns a string to override the request path. |
| `query_params`   | string | A JQ query that returns an object of key-value pairs to set the query parameters. |
| `request_headers`| string | A JQ query that returns an object of key-value pairs to set request headers. |
| `request_body`   | string | A JQ query that returns a string representing the new request body. |
| `response_headers`| string| A JQ query that returns an object of key-value pairs to modify response headers. |
| `response_body`  | string | A JQ query that returns a string to modify the response body. |
| `status_code`    | string | A JQ query that returns an integer to set the HTTP status code. |

### Sample JQ Context

The following context is available to all JQ queries, allowing developers to access request and response information to manipulate it dynamically.

```json
{
  "request": {
    "method": "GET",
    "path": "/users",
    "args": ["user_id"],
    "kwargs": {
      "user_id": "98765"
    },
    "query_params": {
      "limit": ["10"],
      "offset": ["0"],
      "user_role": ["admin"],
      "status": ["active"]
    }
  },
  "response": {
    "headers": {
      "Content-Type": ["application/json"],
      "X-Custom-Header": ["CustomValue"]
    },
    "body": "{\"users\": [{\"id\": \"98765\", \"name\": \"John Doe\", \"role\": \"admin\", \"status\": \"active\"}]}",
    "status_code": 200
  }
}
```

### Example Configuration

```yaml
apiVersion: configuration.konghq.com/v1
kind: KongPlugin
metadata:
  name: modify-request
  namespace: kong
plugin: kong-jq-plugin
config:
  method: '"POST"' # Always change the method to POST
  path: '"/new-path"' # Always rewrite the path
  query_params: '{"limit": ["10"], "offset": ["20"]}' # Set query params
  request_headers: '{"x-custom-header": ["value"]}' # Add a custom header
  response_headers: '{"x-response-header": ["response-value"]}' # Modify response headers
  status_code: "200" # Force the status code to 200
```

### Example Request & Response

#### Incoming Request:

```http
GET /old-path?limit=5
Host: example.com
```

#### Modified Request:

```http
POST /new-path?limit=10&offset=20
Host: example.com
x-custom-header: value
```

#### Incoming Response:

```json
{
  "message": "Old response"
}
```

#### Modified Response:

```http
HTTP/1.1 200 OK
x-response-header: response-value

{
  "message": "New response"
}
```

## Error Handling

The plugin captures and logs errors during the JQ query execution. If an error occurs, the plugin will return an appropriate HTTP 500 response with a detailed error message.

Common error messages:
- `query params jq error`: Indicates an issue with the JQ query processing for query parameters.
- `response body jq error`: Indicates an issue with the JQ query processing for the response body.
- `status code jq error`: Indicates an issue with JQ handling the response status code.


## License

This project is licensed under the MIT License. See the `LICENSE` file for details.

## Contributing

Pull requests are welcome. For significant changes, please open an issue first to discuss what you would like to change.
