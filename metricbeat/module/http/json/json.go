package json

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/metricbeat/helper"
	"github.com/elastic/beats/metricbeat/mb"
	"github.com/elastic/beats/metricbeat/mb/parse"
)

// init registers the MetricSet with the central registry.
// The New method will be called after the setup of the module and before starting to fetch data
func init() {
	if err := mb.Registry.AddMetricSet("http", "json", New, hostParser); err != nil {
		panic(err)
	}
}

const (
	// defaultScheme is the default scheme to use when it is not specified in the host config.
	defaultScheme = "http"

	// defaultPath is the dto use when it is not specified in the host config.
	defaultPath = ""
)

var (
	hostParser = parse.URLHostParserBuilder{
		DefaultScheme: defaultScheme,
		PathConfigKey: "path",
		DefaultPath:   defaultPath,
	}.Build()
)

// MetricSet type defines all fields of the MetricSet
// As a minimum it must inherit the mb.BaseMetricSet fields, but can be extended with
// additional entries. These variables can be used to persist data or configuration between
// multiple fetch calls.
type MetricSet struct {
	mb.BaseMetricSet
	namespace       string
	http            *helper.HTTP
	method          string
	body            string
	requestEnabled  bool
	responseEnabled bool
}

// New create a new instance of the MetricSet
// Part of new is also setting up the configuration by processing additional
// configuration entries if needed.
func New(base mb.BaseMetricSet) (mb.MetricSet, error) {

	logp.Warn("The http json metricset is in beta.")

	config := struct {
		Namespace       string `config:"namespace" validate:"required"`
		Method          string `config:"method"`
		Body            string `config:"body"`
		RequestEnabled  bool   `config:"request.enabled"`
		ResponseEnabled bool   `config:"response.enabled"`
	}{}

	if err := base.Module().UnpackConfig(&config); err != nil {
		return nil, err
	}

	http := helper.NewHTTP(base)
	http.SetMethod(config.Method)
	http.SetBody([]byte(config.Body))

	return &MetricSet{
		BaseMetricSet:   base,
		namespace:       config.Namespace,
		method:          config.Method,
		body:            config.Body,
		http:            http,
		requestEnabled:  config.RequestEnabled,
		responseEnabled: config.ResponseEnabled,
	}, nil
}

// Fetch methods implements the data gathering and data conversion to the right format
// It returns the event which is then forward to the output. In case of an error, a
// descriptive error must be returned.
func (m *MetricSet) Fetch() (common.MapStr, error) {

	response, err := m.http.FetchResponse()
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var jsonBody map[string]interface{}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &jsonBody)
	if err != nil {
		return nil, err
	}

	event := jsonBody

	if m.requestEnabled {
		event[mb.ModuleData] = common.MapStr{
			"request": common.MapStr{
				"headers": m.getHeaders(response.Request.Header),
				"method":  response.Request.Method,
				"body":    m.body,
			},
		}
	}

	if m.responseEnabled {
		event[mb.ModuleData] = common.MapStr{
			"response": common.MapStr{
				"status_code": response.StatusCode,
				"headers":     m.getHeaders(response.Header),
			},
		}
	}

	// Set dynamic namespace
	event["_namespace"] = m.namespace

	return event, nil
}

func (m *MetricSet) getHeaders(header http.Header) map[string]string {

	headers := make(map[string]string)
	for k, v := range header {
		value := ""
		for _, h := range v {
			value += h + " ,"
		}
		value = strings.TrimRight(value, " ,")
		headers[k] = value
	}
	return headers
}
