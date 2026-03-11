package webdav

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

// DefaultTimeout is the default timeout for WebDAV operations.
const DefaultTimeout = 30 * time.Second

// DefaultRetryCount is the default number of retries for failed operations.
const DefaultRetryCount = 5

// DefaultRetryDelay is the initial delay between retries.
const DefaultRetryDelay = 100 * time.Millisecond

// Client is a WebDAV client for interacting with WebDAV servers.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	username   string
	password   string
	token      string
	headers    map[string]string

	mu          sync.RWMutex
	retryCount  int
	retryDelay  time.Duration
	maxRetries  int
	backoffFunc func(int) time.Duration
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithTransport sets the HTTP transport.
func WithTransport(transport http.RoundTripper) ClientOption {
	return func(c *Client) {
		c.httpClient.Transport = transport
	}
}

// WithRetryConfig sets the retry configuration.
func WithRetryConfig(count int, delay time.Duration) ClientOption {
	return func(c *Client) {
		c.maxRetries = count
		c.retryDelay = delay
	}
}

// WithHeaders sets custom headers to be sent with each request.
func WithHeaders(headers map[string]string) ClientOption {
	return func(c *Client) {
		for k, v := range headers {
			c.headers[k] = v
		}
	}
}

// WithSkipTLSVerify disables TLS certificate verification (for testing only).
func WithSkipTLSVerify(skip bool) ClientOption {
	return func(c *Client) {
		if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
			if transport.TLSClientConfig == nil {
				transport.TLSClientConfig = &tls.Config{}
			}
			transport.TLSClientConfig.InsecureSkipVerify = skip
		}
	}
}

// NewClient creates a new WebDAV client.
func NewClient(baseURL string, opts ...ClientOption) (*Client, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Ensure URL ends with /
	if !strings.HasSuffix(parsedURL.Path, "/") {
		parsedURL.Path += "/"
	}

	client := &Client{
		baseURL: parsedURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{},
			},
		},
		headers:    make(map[string]string),
		maxRetries: DefaultRetryCount,
		retryDelay: DefaultRetryDelay,
	}

	// Set up exponential backoff
	client.backoffFunc = func(attempt int) time.Duration {
		return client.retryDelay * time.Duration(1<<uint(attempt))
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// SetAuth sets the authentication credentials.
func (c *Client) SetAuth(username, password string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.username = username
	c.password = password
	c.token = ""
}

// SetToken sets the bearer token for authentication.
func (c *Client) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
	c.username = ""
	c.password = ""
}

// Ping tests the connection to the WebDAV server.
func (c *Client) Ping(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodOptions, "/", nil)
	if err != nil {
		return wrapError("ping", "", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Get downloads a file from the WebDAV server.
func (c *Client) Get(ctx context.Context, remotePath string) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodGet, remotePath, nil)
	if err != nil {
		return nil, wrapError("get", remotePath, err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapError("get", remotePath, err)
	}

	return data, nil
}

// GetWithETag downloads a file and returns its ETag.
func (c *Client) GetWithETag(ctx context.Context, remotePath string) ([]byte, string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, remotePath, nil)
	if err != nil {
		return nil, "", wrapError("get", remotePath, err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", wrapError("get", remotePath, err)
	}

	etag := resp.Header.Get("ETag")
	return data, etag, nil
}

// Put uploads a file to the WebDAV server.
func (c *Client) Put(ctx context.Context, remotePath string, data []byte) error {
	req, err := c.newRequest(ctx, http.MethodPut, remotePath, bytes.NewReader(data))
	if err != nil {
		return wrapError("put", remotePath, err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(len(data))

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// PutIfNoneMatch uploads a file only if it doesn't exist (If-None-Match: *).
func (c *Client) PutIfNoneMatch(ctx context.Context, remotePath string, data []byte) error {
	req, err := c.newRequest(ctx, http.MethodPut, remotePath, bytes.NewReader(data))
	if err != nil {
		return wrapError("put", remotePath, err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("If-None-Match", "*")
	req.ContentLength = int64(len(data))

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// PutIfMatch uploads a file only if the ETag matches.
func (c *Client) PutIfMatch(ctx context.Context, remotePath string, data []byte, etag string) error {
	req, err := c.newRequest(ctx, http.MethodPut, remotePath, bytes.NewReader(data))
	if err != nil {
		return wrapError("put", remotePath, err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("If-Match", etag)
	req.ContentLength = int64(len(data))

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Delete removes a file or directory from the WebDAV server.
func (c *Client) Delete(ctx context.Context, remotePath string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, remotePath, nil)
	if err != nil {
		return wrapError("delete", remotePath, err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// MkCol creates a directory on the WebDAV server.
func (c *Client) MkCol(ctx context.Context, remotePath string) error {
	req, err := c.newRequest(ctx, "MKCOL", remotePath, nil)
	if err != nil {
		return wrapError("mkcol", remotePath, err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Copy copies a file or directory on the WebDAV server.
func (c *Client) Copy(ctx context.Context, srcPath, dstPath string, overwrite bool) error {
	req, err := c.newRequest(ctx, "COPY", srcPath, nil)
	if err != nil {
		return wrapError("copy", srcPath, err)
	}

	destURL := *c.baseURL
	destURL.Path = path.Join(destURL.Path, dstPath)
	req.Header.Set("Destination", destURL.String())

	if overwrite {
		req.Header.Set("Overwrite", "T")
	} else {
		req.Header.Set("Overwrite", "F")
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Move moves a file or directory on the WebDAV server.
func (c *Client) Move(ctx context.Context, srcPath, dstPath string, overwrite bool) error {
	req, err := c.newRequest(ctx, "MOVE", srcPath, nil)
	if err != nil {
		return wrapError("move", srcPath, err)
	}

	destURL := *c.baseURL
	destURL.Path = path.Join(destURL.Path, dstPath)
	req.Header.Set("Destination", destURL.String())

	if overwrite {
		req.Header.Set("Overwrite", "T")
	} else {
		req.Header.Set("Overwrite", "F")
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// PropFind performs a PROPFIND request to retrieve properties.
func (c *Client) PropFind(ctx context.Context, remotePath string, depth int, propFindXML string) (*PropFindResponse, error) {
	req, err := c.newRequest(ctx, "PROPFIND", remotePath, strings.NewReader(propFindXML))
	if err != nil {
		return nil, wrapError("propfind", remotePath, err)
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Depth", fmt.Sprintf("%d", depth))

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var propFindResp PropFindResponse
	if err := xml.NewDecoder(resp.Body).Decode(&propFindResp); err != nil {
		return nil, wrapError("propfind", remotePath, err)
	}

	return &propFindResp, nil
}

// PropPatch performs a PROPPATCH request to update properties.
func (c *Client) PropPatch(ctx context.Context, remotePath string, propPatchXML string) error {
	req, err := c.newRequest(ctx, "PROPPATCH", remotePath, strings.NewReader(propPatchXML))
	if err != nil {
		return wrapError("proppatch", remotePath, err)
	}
	req.Header.Set("Content-Type", "application/xml")

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Lock creates a lock on a resource.
func (c *Client) Lock(ctx context.Context, remotePath string, timeout string, owner string) (string, error) {
	lockXML := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<D:lockinfo xmlns:D="DAV:">
	<D:lockscope><D:exclusive/></D:lockscope>
	<D:locktype><D:write/></D:locktype>
	<D:owner>%s</D:owner>
</D:lockinfo>`, owner)

	req, err := c.newRequest(ctx, "LOCK", remotePath, strings.NewReader(lockXML))
	if err != nil {
		return "", wrapError("lock", remotePath, err)
	}
	req.Header.Set("Content-Type", "application/xml")
	if timeout != "" {
		req.Header.Set("Timeout", timeout)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	lockToken := resp.Header.Get("Lock-Token")
	if lockToken == "" {
		return "", wrapError("lock", remotePath, ErrInvalidResponse)
	}

	// Remove angle brackets if present
	lockToken = strings.Trim(lockToken, "<>")
	return lockToken, nil
}

// Unlock removes a lock from a resource.
func (c *Client) Unlock(ctx context.Context, remotePath string, lockToken string) error {
	req, err := c.newRequest(ctx, "UNLOCK", remotePath, nil)
	if err != nil {
		return wrapError("unlock", remotePath, err)
	}
	req.Header.Set("Lock-Token", "<"+lockToken+">")

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// newRequest creates a new HTTP request with the given method and path.
func (c *Client) newRequest(ctx context.Context, method, remotePath string, body io.Reader) (*http.Request, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Build URL
	reqURL := *c.baseURL
	reqURL.Path = path.Join(reqURL.Path, remotePath)

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), body)
	if err != nil {
		return nil, err
	}

	// Set authentication
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	// Set custom headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

// doRequest executes an HTTP request with retry logic.
func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = &Error{
				Op:         req.Method,
				Path:       req.URL.Path,
				Err:        err,
				StatusCode: 0,
			}

			// Check if error is retryable
			if !isRetryableError(err) {
				return nil, lastErr
			}

			// Wait before retry
			if attempt < c.maxRetries {
				select {
				case <-req.Context().Done():
					return nil, &Error{
						Op:   req.Method,
						Path: req.URL.Path,
						Err:  ErrCancelled,
					}
				case <-time.After(c.backoffFunc(attempt)):
					continue
				}
			}
			continue
		}

		// Check response status
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		// Handle error response
		respErr := c.handleErrorResponse(resp)
		lastErr = respErr

		// Check if error is retryable
		if !respErr.IsRetryable() || attempt >= c.maxRetries {
			resp.Body.Close()
			return nil, respErr
		}

		resp.Body.Close()

		// Wait before retry
		select {
		case <-req.Context().Done():
			return nil, &Error{
				Op:   req.Method,
				Path: req.URL.Path,
				Err:  ErrCancelled,
			}
		case <-time.After(c.backoffFunc(attempt)):
			continue
		}
	}

	return nil, lastErr
}

// handleErrorResponse handles HTTP error responses.
func (c *Client) handleErrorResponse(resp *http.Response) *Error {
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var opErr error
	if len(body) > 0 {
		opErr = fmt.Errorf("%s", string(body))
	} else {
		opErr = fmt.Errorf("status: %s", resp.Status)
	}

	wErr := newError(resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, opErr)

	// Check for Retry-After header
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if d, parseErr := time.ParseDuration(retryAfter + "s"); parseErr == nil {
			wErr.RetryAfter = d
		}
	}

	return wErr
}

// isRetryableError checks if an error is retryable.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// Network errors are generally retryable
	return true
}

// ResourceInfo represents information about a WebDAV resource.
type ResourceInfo struct {
	Path         string
	Name         string
	IsCollection bool
	Size         int64
	ETag         string
	LastModified time.Time
	CreatedAt    time.Time
	ContentType  string
}

// PropFindResponse represents a PROPFIND response.
type PropFindResponse struct {
	XMLName   xml.Name       `xml:"multistatus"`
	Responses []PropFindResp `xml:"response"`
}

// PropFindResp represents a single response in a PROPFIND response.
type PropFindResp struct {
	XMLName  xml.Name   `xml:"response"`
	Href     string     `xml:"href"`
	PropStat []PropStat `xml:"propstat"`
}

// PropStat represents property status in a PROPFIND response.
type PropStat struct {
	XMLName xml.Name `xml:"propstat"`
	Prop    DAVProp  `xml:"D:prop"`
	Status  string   `xml:"D:status"`
}

// DAVProp represents WebDAV properties.
type DAVProp struct {
	XMLName       xml.Name  `xml:"D:prop"`
	ResourceType  *xml.Name `xml:"resourcetype"`
	DisplayName   string    `xml:"displayname"`
	ContentType   string    `xml:"D:getcontenttype"`
	ContentLength int64     `xml:"D:getcontentlength"`
	ETag          string    `xml:"D:getetag"`
	LastModified  string    `xml:"D:getlastmodified"`
	CreatedAt     string    `xml:"D:creationdate"`
}

// IsCollection checks if the resource is a collection (directory).
func (p *DAVProp) IsCollection() bool {
	if p.ResourceType == nil {
		return false
	}
	// Check if resourcetype contains D:collection
	return p.ResourceType.Space == "DAV:" && p.ResourceType.Local == "collection"
}

// ToResourceInfo converts a PropFindResp to ResourceInfo.
func (r *PropFindResp) ToResourceInfo() ResourceInfo {
	info := ResourceInfo{
		Path: r.Href,
		Name: path.Base(r.Href),
	}

	if len(r.PropStat) > 0 {
		prop := r.PropStat[0].Prop
		info.IsCollection = prop.IsCollection()
		info.Size = prop.ContentLength
		info.ETag = strings.Trim(prop.ETag, "\"")
		info.ContentType = prop.ContentType

		if prop.LastModified != "" {
			if t, err := time.Parse(time.RFC1123, prop.LastModified); err == nil {
				info.LastModified = t
			}
		}

		if prop.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, prop.CreatedAt); err == nil {
				info.CreatedAt = t
			}
		}
	}

	return info
}
