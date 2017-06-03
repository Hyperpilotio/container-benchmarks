package http

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/Sirupsen/logrus"
	simplejson "github.com/bitly/go-simplejson"
	"github.com/dghubble/sling"
)

// Response sends the HTTP request and reads the body of the HTTP response
func Response(s *sling.Sling) *ResponseExt {
	/**
	 * Set up the HTTP request
	 */
	req, err := s.Request()
	if err != nil {
		panic(err)
	}
	// :~)

	return responseExtByRequest(req)
}

func responseExtByRequest(req *http.Request) *ResponseExt {
	/**
	 * Send the HTTP request
	 */
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	// :~)

	return responseExtByResponse(resp)
}

func responseExtByResponse(resp *http.Response) *ResponseExt {
	/**
	 * Read the body of the HTTP response
	 */
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	// :~)

	return &ResponseExt{
		Response: resp,
		body:     bodyBytes,
	}
}

// ResponseExt is an extended type of `http.Response`
type ResponseExt struct {
	Response *http.Response
	body     []byte
}

// JSONBody returns the body of the HTTP response in JSON format. It panics if
// any error occurs during the parsing.
func (r *ResponseExt) JSONBody() *simplejson.Json {
	jsonResult, err := simplejson.NewJson(r.body)
	if err != nil {
		panic(err)
	}

	return jsonResult
}

// ClientConfig is the configuration of the http client
type ClientConfig struct {
	Ssl      bool   // host of the http service
	Host     string // port of the http service
	Port     uint16 // enable SSL for the http service
	Resource string // specify the resource, i.e. `http://<host>:<port>/<resource>`

	slingBase *sling.Sling
}

// NewClientConfig returns a new ClientConfig. The values of the fields
// can be set by the command line:
//	Ssl: by `-http.host`
//	Host: by `-http.port`
//	Port: by `-http.ssl`
//	Resource: by `-http.resource`
func NewClientConfig() *ClientConfig {
	return newClientConfigByFlag()
}

func newClientConfigByFlag() *ClientConfig {
	var host = flag.String("http.host", "127.0.0.1", "Host of the tested HTTP service")
	var port = flag.Int("http.port", 7778, "Port of the tested HTTP service")
	var ssl = flag.Bool("http.ssl", false, "Enable SSL for the tested HTTP service")
	var resource = flag.String("http.resource", "", "specify the resource, i.e. 'http://<host>:<port>/<resource>'")

	flag.Parse()

	config := &ClientConfig{
		Host:     *host,
		Port:     uint16(*port),
		Ssl:      *ssl,
		Resource: *resource,
	}
	config.slingBase = sling.New().Base(
		config.hostAndPort(),
	)

	if config.Resource != "" {
		config.slingBase.Path(config.Resource + "/")
	}

	log.Infof("Sling URL for testing: %s", config.String())

	return config
}

func (c *ClientConfig) hostAndPort() string {
	schema := "http"
	if c.Ssl {
		schema = "https"
	}

	return fmt.Sprintf("%s://%s:%d", schema, c.Host, c.Port)
}

// String gets the URL string the client accesses to
func (c *ClientConfig) String() string {
	url := c.hostAndPort()

	if c.Resource != "" {
		url += "/" + c.Resource
	}

	return url
}

// NewSling returns a new `Sling` variable based on the `ClientConfig`.
func (c *ClientConfig) NewSling() *sling.Sling {
	return c.slingBase.New()
}
