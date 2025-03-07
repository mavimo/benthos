package processor

import (
	"context"
	"encoding/json"
	"fmt"

	jmespath "github.com/jmespath/go-jmespath"

	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/component/processor"
	"github.com/benthosdev/benthos/v4/internal/docs"
	"github.com/benthosdev/benthos/v4/internal/interop"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/message"
)

//------------------------------------------------------------------------------

func init() {
	Constructors[TypeJMESPath] = TypeSpec{
		constructor: func(conf Config, mgr interop.Manager, log log.Modular, stats metrics.Type) (processor.V1, error) {
			p, err := newJMESPath(conf.JMESPath, mgr)
			if err != nil {
				return nil, err
			}
			return processor.NewV2ToV1Processor("jmespath", p, mgr.Metrics()), nil
		},
		Categories: []string{
			"Mapping",
		},
		Summary: `
Executes a [JMESPath query](http://jmespath.org/) on JSON documents and replaces
the message with the resulting document.`,
		Description: `
:::note Try out Bloblang
For better performance and improved capabilities try out native Benthos mapping with the [bloblang processor](/docs/components/processors/bloblang).
:::
`,
		Examples: []docs.AnnotatedExample{
			{
				Title: "Mapping",
				Summary: `
When receiving JSON documents of the form:

` + "```json" + `
{
  "locations": [
    {"name": "Seattle", "state": "WA"},
    {"name": "New York", "state": "NY"},
    {"name": "Bellevue", "state": "WA"},
    {"name": "Olympia", "state": "WA"}
  ]
}
` + "```" + `

We could collapse the location names from the state of Washington into a field ` + "`Cities`" + `:

` + "```json" + `
{"Cities": "Bellevue, Olympia, Seattle"}
` + "```" + `

With the following config:`,
				Config: `
pipeline:
  processors:
    - jmespath:
        query: "locations[?state == 'WA'].name | sort(@) | {Cities: join(', ', @)}"
`,
			},
		},
		Config: docs.FieldComponent().WithChildren(
			docs.FieldString("query", "The JMESPath query to apply to messages."),
		),
	}
}

//------------------------------------------------------------------------------

// JMESPathConfig contains configuration fields for the JMESPath processor.
type JMESPathConfig struct {
	Query string `json:"query" yaml:"query"`
}

// NewJMESPathConfig returns a JMESPathConfig with default values.
func NewJMESPathConfig() JMESPathConfig {
	return JMESPathConfig{
		Query: "",
	}
}

//------------------------------------------------------------------------------

type jmespathProc struct {
	query *jmespath.JMESPath
	log   log.Modular
}

func newJMESPath(conf JMESPathConfig, mgr interop.Manager) (processor.V2, error) {
	query, err := jmespath.Compile(conf.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JMESPath query: %v", err)
	}
	j := &jmespathProc{
		query: query,
		log:   mgr.Logger(),
	}
	return j, nil
}

func safeSearch(part interface{}, j *jmespath.JMESPath) (res interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("jmespath panic: %v", r)
		}
	}()
	return j.Search(part)
}

// JMESPath doesn't like json.Number so we walk the tree and replace them.
func clearNumbers(v interface{}) (interface{}, bool) {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, v := range t {
			if nv, ok := clearNumbers(v); ok {
				t[k] = nv
			}
		}
	case []interface{}:
		for i, v := range t {
			if nv, ok := clearNumbers(v); ok {
				t[i] = nv
			}
		}
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			if i, err := t.Int64(); err == nil {
				return i, true
			}
		}
		return f, true
	}
	return nil, false
}

func (p *jmespathProc) Process(ctx context.Context, msg *message.Part) ([]*message.Part, error) {
	newMsg := msg.Copy()

	jsonPart, err := newMsg.JSON()
	if err != nil {
		p.log.Debugf("Failed to parse part into json: %v\n", err)
		return nil, err
	}
	if v, replace := clearNumbers(jsonPart); replace {
		jsonPart = v
	}

	var result interface{}
	if result, err = safeSearch(jsonPart, p.query); err != nil {
		p.log.Debugf("Failed to search json: %v\n", err)
		return nil, err
	}

	newMsg.SetJSON(result)
	return []*message.Part{newMsg}, nil
}

func (p *jmespathProc) Close(context.Context) error {
	return nil
}
