package gosepehr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

type GoSepehr struct {
	basePath    string
	restyClient *resty.Client
	Config      struct {
		GetTokenEndpoint string
		AdviceEndpoint   string
	}
}

const (
	adminClientID string = "admin-cli"
	urlSeparator  string = "/"
)

func makeURL(path ...string) string {
	return strings.Join(path, urlSeparator)
}

// GetRequest returns a request for calling endpoints.
func (g *GoSepehr) GetRequest(ctx context.Context) *resty.Request {
	var err HTTPErrorResponse
	return injectTracingHeaders(
		ctx, g.restyClient.R().
			SetContext(ctx).
			SetError(&err),
	)
}

func injectTracingHeaders(ctx context.Context, req *resty.Request) *resty.Request {
	// look for span in context, do nothing if span is not found
	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		return req
	}

	// look for tracer in context, use global tracer if not found
	tracer, ok := ctx.Value(tracerContextKey).(opentracing.Tracer)
	if !ok || tracer == nil {
		tracer = opentracing.GlobalTracer()
	}

	// inject tracing header into request
	err := tracer.Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	if err != nil {
		return req
	}

	return req
}

// GetRequestWithBearerAuthNoCache returns a JSON base request configured with an auth token and no-cache header.
func (g *GoSepehr) GetRequestWithBearerAuthNoCache(ctx context.Context, token string) *resty.Request {
	return g.GetRequest(ctx).
		SetAuthToken(token).
		SetHeader("Content-Type", "application/json").
		SetHeader("Cache-Control", "no-cache")
}

// GetRequestWithBearerAuth returns a JSON base request configured with an auth token.
func (g *GoSepehr) GetRequestWithBearerAuth(ctx context.Context, token string) *resty.Request {
	return g.GetRequest(ctx).
		SetAuthToken(token).
		SetHeader("Content-Type", "application/json")
}

func (g *GoSepehr) GetRequestNormal(ctx context.Context) *resty.Request {
	return g.GetRequest(ctx).
		SetHeader("Content-Type", "application/json")
}

func NewClient(basePath string, options ...func(*GoSepehr)) *GoSepehr {
	c := GoSepehr{
		basePath:    strings.TrimRight(basePath, urlSeparator),
		restyClient: resty.New(),
	}

	c.Config.GetTokenEndpoint = makeURL("V1", "PeymentApi", "GetToken")
	c.Config.AdviceEndpoint = makeURL("V1", "PeymentApi", "Advice")

	for _, option := range options {
		option(&c)
	}

	return &c
}

// RestyClient returns the internal resty g.
// This can be used to configure the g.
func (g *GoSepehr) RestyClient() *resty.Client {
	return g.restyClient
}

// SetRestyClient overwrites the internal resty g.
func (g *GoSepehr) SetRestyClient(restyClient *resty.Client) {
	g.restyClient = restyClient
	g.restyClient.SetTimeout(30 * time.Second)
}

func checkForError(resp *resty.Response, err error, errMessage string) error {
	if err != nil {
		return &APIError{
			Code:    0,
			Message: errors.Wrap(err, errMessage).Error(),
			Type:    ParseAPIErrType(err),
		}
	}

	if resp == nil {
		return &APIError{
			Message: "empty response",
			Type:    ParseAPIErrType(err),
		}
	}

	if resp.IsError() {
		var msg string

		// Parse the error message from the body if available
		if e, ok := resp.Error().(*HTTPErrorResponse); ok && e.NotEmpty() {
			msg = fmt.Sprintf("%s: %s", resp.Status(), e)
		} else if resp.Body() != nil {
			// If the body contains a message, include it
			bodyMsg := string(resp.Body())
			msg = fmt.Sprintf("%s: %s", resp.Status(), bodyMsg)
		} else {
			msg = resp.Status()
		}

		return &APIError{
			Code:    resp.StatusCode(),
			Message: msg,
			Type:    ParseAPIErrType(err),
		}
	}

	return nil
}

func (g *GoSepehr) GetToken(ctx context.Context, req GetTokenRequest) (*GetTokenResponse, error) {
	var resp GetTokenResponse

	response, err := g.GetRequestNormal(ctx).
		SetBody(req).
		SetResult(&resp).
		Post(g.basePath + "/" + g.Config.GetTokenEndpoint)

	if err := checkForError(response, err, "error getting token"); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (g *GoSepehr) Advice(ctx context.Context, req AdviceRequest) (*AdviceResponse, error) {
	var resp AdviceResponse

	response, err := g.GetRequestNormal(ctx).
		SetBody(req).
		SetResult(&resp).
		Post(g.basePath + "/" + g.Config.AdviceEndpoint)

	if err := checkForError(response, err, "error getting advice"); err != nil {
		return nil, err
	}

	return &resp, nil
}
