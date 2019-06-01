/*
 * Copyright (c) 2019 TFG Co <backend@tfgco.com>
 * Author: TFG Co <backend@tfgco.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package http

import (
	"fmt"
	"net/http"

	"github.com/opentracing/opentracing-go"
	tracing "github.com/topfreegames/go-extensions-tracing"
)

// New creates and instruments an HTTP client
func New() *http.Client {
	client := &http.Client{}
	Instrument(client)
	return client
}

// Instrument instruments the internal transport object
func Instrument(client *http.Client) {
	inner := getTransport(client)
	client.Transport = &Transport{inner}
}

func getTransport(client *http.Client) http.RoundTripper {
	if client.Transport == nil {
		return http.DefaultTransport
	}
	return client.Transport
}

// Transport instruments another RoundTripper
type Transport struct {
	inner http.RoundTripper
}

// RoundTrip executes a single HTTP transaction
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	Trace(req, func() error {
		resp, err = t.inner.RoundTrip(req)
		return err
	})

	return resp, err
}

// Trace wraps an HTTP request and reports it to tracing
func Trace(req *http.Request, next func() error) {
	var parent opentracing.SpanContext

	ctx := req.Context()
	if span := opentracing.SpanFromContext(ctx); span != nil {
		parent = span.Context()
	}

	operationName := fmt.Sprintf("HTTP %s %s", req.Method, req.Host)
	reference := opentracing.ChildOf(parent)
	tags := opentracing.Tags{
		"http.method":   req.Method,
		"http.host":     req.Host,
		"http.pathname": req.URL.Path,
		"http.query":    req.URL.RawQuery,

		"span.kind": "client",
	}

	span := opentracing.StartSpan(operationName, reference, tags)
	defer span.Finish()
	defer tracing.LogPanic(span)

	tracer := opentracing.GlobalTracer()
	tracer.Inject(span.Context(), opentracing.HTTPHeaders, &req.Header)

	err := next()
	if err != nil {
		message := err.Error()
		tracing.LogError(span, message)
	}
}
