package metrics

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcmeli/logging/format"
	"github.com/mercadolibre/go-meli-toolkit/gingonic/mlhandlers"
	"github.com/mercadolibre/go-meli-toolkit/godog"
	newrelic "github.com/newrelic/go-agent"
)

type Metric struct {
	metricType string
	Name       string
	Value      float64
	tags       Tags
}

type Tags map[string]interface{}

type Metrics struct {
	Values []Metric
}

type Transaction struct {
	nrTrx newrelic.Transaction
}

var NewRelicApp newrelic.Application

const (
	FULL     = "F"
	SIMPLE   = "S"
	COMPOUND = "C"
	ERROR    = "E" // Sends error to NewRelic
)

var namePrefix = ""
var defaultTags = Tags{}

func UsePrefix(prefix string) {
	namePrefix = prefix
}

func DefaultTags(tags Tags) {
	defaultTags = tags
}

// Returns a metric of type "full"
func (metrics Metrics) Full(name string, value float64, tags ...Tags) Metrics {
	return Metrics{append(metrics.Values, Metric{FULL, name, value, mergeTags(tags)})}
}

// Returns a metric of type "simple"
func (metrics Metrics) Simple(name string, value float64, tags ...Tags) Metrics {
	return Metrics{append(metrics.Values, Metric{SIMPLE, name, value, mergeTags(tags)})}
}

// Returns a metric of type "compound"
func (metrics Metrics) Compound(name string, value float64, tags ...Tags) Metrics {
	return Metrics{append(metrics.Values, Metric{COMPOUND, name, value, mergeTags(tags)})}
}

// Returns a metric of type "simple" with a value of 1
func (metrics Metrics) Counter(name string, tags ...Tags) Metrics {
	return Metrics{append(metrics.Values, Metric{SIMPLE, name, float64(1), mergeTags(tags)})}
}

// Returns a metric of type "simple" with a value of 1
func (metrics Metrics) Error(name string, tags ...Tags) Metrics {
	return Metrics{append(metrics.Values, Metric{ERROR, name, float64(1), mergeTags(tags)})}
}

// Returns a metric of type "full"
func Full(name string, value float64, tags ...Tags) Metrics {
	return Metrics{[]Metric{{FULL, name, value, mergeTags(tags)}}}
}

// Returns a metric of type "simple"
func Simple(name string, value float64, tags ...Tags) Metrics {
	return Metrics{[]Metric{{SIMPLE, name, value, mergeTags(tags)}}}
}

// Returns a metric of type "error"
func Error(name string, tags ...Tags) Metrics {
	return Metrics{[]Metric{{ERROR, name, float64(1), mergeTags(tags)}}}
}

// Returns a metric of type "compound"
func Compound(name string, value float64, tags ...Tags) Metrics {
	return Metrics{[]Metric{{COMPOUND, name, value, mergeTags(tags)}}}
}

// Returns a metric of type "simple" with a value of 1
func Counter(name string, tags ...Tags) Metrics {
	return Metrics{[]Metric{{SIMPLE, name, float64(1), mergeTags(tags)}}}
}

// Pushes a metric
func PushMetric(metric Metric, trx *Transaction, tags ...Tags) error {
	name := namePrefix + "." + metric.Name
	strTags := defaultTags.Merge(mergeTags(tags)).asMetricTags()
	switch metric.metricType {
	case FULL:
		godog.RecordFullMetric(name, metric.Value, strTags...)
	case SIMPLE:
		godog.RecordSimpleMetric(name, metric.Value, strTags...)
	case COMPOUND:
		godog.RecordCompoundMetric(name, metric.Value, strTags...)
	case ERROR:
		if trx != nil {
			trx.NoticeError(name)
		}
		godog.RecordSimpleMetric(name, float64(1), strTags...)
	default:
		return fmt.Errorf("Unkown metric type: %s", metric.metricType)
	}
	return nil
}

func GingonicHandlers() []gin.HandlerFunc {
	return []gin.HandlerFunc{mlhandlers.Datadog(), NewRelic()}
}

func InitNewRelic(debug bool, environment string, appName string, appKey string) error {
	fmt.Println(environment)
	config := newrelic.NewConfig(fmt.Sprintf("%s.%s", environment, appName), appKey)
	if debug {
		config.Logger = newrelic.NewDebugLogger(os.Stdout)
	}
	if app, err := newrelic.NewApplication(config); err != nil {
		return fmt.Errorf("Could not create newrelic agent: %s", err)
	} else {
		NewRelicApp = app
	}
	return nil
}

// Helpers

func MinutesSince(t time.Time) float64 {
	return t.Sub(time.Now()).Minutes()
}

func ElapsedMilliseconds(t time.Time) float64 {
	return format.Milliseconds(time.Since(t))
}

func Trx(id string) *Transaction {
	nrTrx := NewRelicApp.StartTransaction(id, nil, nil)
	return &Transaction{nrTrx}
}

func (trx *Transaction) Segment(name string) *Segment {
	return &Segment{newrelic.StartSegment(trx.nrTrx, name)}
}

func (trx *Transaction) NoticeError(name string) {
	if trx.nrTrx != nil {
		trx.nrTrx.NoticeError(errors.New(name))
	}
}

func (trx *Transaction) End() {
	if trx.nrTrx != nil {
		trx.nrTrx.End()
	}
}

type Segment struct {
	nrSeg *newrelic.Segment
}

func NullSegment() *Segment {
	return &Segment{}
}

func (seg *Segment) End() {
	if seg.nrSeg != nil {
		seg.nrSeg.End()
	}
}

// Middleware to use with New Relic
func NewRelic() gin.HandlerFunc {
	return func(c *gin.Context) {
		txn := NewRelicApp.StartTransaction(c.Request.URL.String(), c.Writer, c.Request)
		defer txn.End()
		c.Set("NR_TXN", txn)
		c.Next()
	}
}

// Datatype to hanlde metric tags
func (tags Tags) asMetricTags() []string {
	res := make([]string, 0, len(tags))
	for k, v := range tags {
		res = append(res, fmt.Sprintf("%v:%v", k, v))
	}
	return res
}

func (tags Tags) Merge(other Tags) Tags {
	return mergeTags([]Tags{tags, other})
}

func mergeTags(tags []Tags) Tags {
	if len(tags) == 1 {
		return tags[0]
	}
	merged := make(Tags, len(tags)*2)
	for _, t := range tags {
		for k, v := range t {
			merged[k] = v
		}
	}
	return merged
}
