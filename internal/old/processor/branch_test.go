package processor

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/manager/mock"
	"github.com/benthosdev/benthos/v4/internal/message"
)

type mockMsg struct {
	content string
	meta    map[string]string
	err     error
}

func (m mockMsg) withErr(err error) mockMsg {
	m.err = err
	return m
}

func TestBranchBasic(t *testing.T) {
	msg := func(content string, meta ...string) mockMsg {
		t.Helper()
		m := mockMsg{
			content: content,
			meta:    map[string]string{},
		}
		for i, v := range meta {
			if i%2 == 1 {
				m.meta[meta[i-1]] = v
			}
		}
		return m
	}

	tests := map[string]struct {
		requestMap   string
		processorMap string
		resultMap    string
		input        []mockMsg
		output       []mockMsg
	}{
		"empty request mapping": {
			requestMap:   "",
			processorMap: "root.nested = this",
			resultMap:    "root.result = this.nested",
			input: []mockMsg{
				msg(`{"value":"foobar"}`),
			},
			output: []mockMsg{
				msg(`{"result":{"value":"foobar"},"value":"foobar"}`),
			},
		},
		"empty result mapping": {
			requestMap:   "root.nested = this",
			processorMap: "root = this",
			resultMap:    "",
			input: []mockMsg{
				msg(`{"value":"foobar"}`),
			},
			output: []mockMsg{
				msg(`{"value":"foobar"}`),
			},
		},
		"copy metadata over only": {
			requestMap:   `meta foo = meta("foo")`,
			processorMap: `meta foo = meta("foo") + " and this"`,
			resultMap:    `meta new_foo = meta("foo")`,
			input: []mockMsg{
				msg(
					`{"value":"foobar"}`,
					"foo", "bar",
				),
			},
			output: []mockMsg{
				msg(
					`{"value":"foobar"}`,
					"foo", "bar",
					"new_foo", "bar and this",
				),
			},
		},
		"do not carry error into branch": {
			requestMap: `root = this`,
			processorMap: `root = this
				root.name_upper = this.name.uppercase()`,
			resultMap: `root.result = if this.failme.bool(false) {
					throw("this is a branch error")
				} else {
					this.name_upper
				}`,
			input: []mockMsg{
				msg(`{"id":0,"name":"first"}`).withErr(errors.New("this is a pre-existing failure")),
				msg(`{"failme":true,"id":1,"name":"second"}`),
				msg(`{"failme":true,"id":2,"name":"third"}`).withErr(errors.New("this is a pre-existing failure")),
			},
			output: []mockMsg{
				msg(`{"id":0,"name":"first","result":"FIRST"}`).withErr(errors.New("this is a pre-existing failure")),
				msg(`{"failme":true,"id":1,"name":"second"}`).withErr(errors.New("result mapping failed: failed assignment (line 1): this is a branch error")),
				msg(`{"failme":true,"id":2,"name":"third"}`).withErr(errors.New("result mapping failed: failed assignment (line 1): this is a branch error")),
			},
		},
		"map error into branch": {
			requestMap:   `root.err = error()`,
			processorMap: `root.err = this.err.string().uppercase()`,
			resultMap:    `root.result_err = this.err`,
			input: []mockMsg{
				msg(`{"id":0,"name":"first"}`).withErr(errors.New("this is a pre-existing failure")),
				msg(`{"id":1,"name":"second"}`),
			},
			output: []mockMsg{
				msg(`{"id":0,"name":"first","result_err":"THIS IS A PRE-EXISTING FAILURE"}`).withErr(errors.New("this is a pre-existing failure")),
				msg(`{"id":1,"name":"second","result_err":"NULL"}`),
			},
		},
		"filtered and failed mappings": {
			requestMap: `root = match {
				this.id == 0 => throw("i dont like zero"),
				this.id == 3 => deleted(),
				_ => {"name":this.name,"id":this.id}
			}`,
			processorMap: `root = this
			root.name_upper = this.name.uppercase()`,
			resultMap: `root.result = match {
				this.id == 2 => throw("i dont like two either"),
				_ => this.name_upper
			}`,
			input: []mockMsg{
				msg(`{"id":0,"name":"first"}`),
				msg(`{"id":1,"name":"second"}`),
				msg(`{"id":2,"name":"third"}`),
				msg(`{"id":3,"name":"fourth"}`),
				msg(`{"id":4,"name":"fifth"}`),
			},
			output: []mockMsg{
				msg(`{"id":0,"name":"first"}`).withErr(errors.New("request mapping failed: failed assignment (line 1): i dont like zero")),
				msg(`{"id":1,"name":"second","result":"SECOND"}`),
				msg(`{"id":2,"name":"third"}`).withErr(errors.New("result mapping failed: failed assignment (line 1): i dont like two either")),
				msg(`{"id":3,"name":"fourth"}`),
				msg(`{"id":4,"name":"fifth","result":"FIFTH"}`),
			},
		},
		"filter all requests": {
			requestMap:   `root = deleted()`,
			processorMap: `root = this`,
			resultMap:    `root.result = this`,
			input: []mockMsg{
				msg(`{"id":0,"name":"first"}`),
				msg(`{"id":1,"name":"second"}`),
				msg(`{"id":2,"name":"third"}`),
				msg(`{"id":3,"name":"fourth"}`),
				msg(`{"id":4,"name":"fifth"}`),
			},
			output: []mockMsg{
				msg(`{"id":0,"name":"first"}`),
				msg(`{"id":1,"name":"second"}`),
				msg(`{"id":2,"name":"third"}`),
				msg(`{"id":3,"name":"fourth"}`),
				msg(`{"id":4,"name":"fifth"}`),
			},
		},
		"filter during processing": {
			requestMap:   `root = if this.id == 3 { throw("foo") } else { this }`,
			processorMap: `root = deleted()`,
			resultMap:    `root.result = this`,
			input: []mockMsg{
				msg(`{"id":0,"name":"first"}`),
				msg(`{"id":1,"name":"second"}`),
				msg(`{"id":2,"name":"third"}`),
				msg(`{"id":3,"name":"fourth"}`),
				msg(`{"id":4,"name":"fifth"}`),
			},
			output: []mockMsg{
				msg(`{"id":0,"name":"first"}`).withErr(errors.New("child processors resulted in zero messages")),
				msg(`{"id":1,"name":"second"}`).withErr(errors.New("child processors resulted in zero messages")),
				msg(`{"id":2,"name":"third"}`).withErr(errors.New("child processors resulted in zero messages")),
				msg(`{"id":3,"name":"fourth"}`).withErr(errors.New("request mapping failed: failed assignment (line 1): foo")),
				msg(`{"id":4,"name":"fifth"}`).withErr(errors.New("child processors resulted in zero messages")),
			},
		},
		"filter some during processing": {
			requestMap:   `root = if this.id == 3 { throw("foo") } else { this }`,
			processorMap: `root = if this.id == 2 { deleted() }`,
			resultMap:    `root.result = this`,
			input: []mockMsg{
				msg(`{"id":0,"name":"first"}`),
				msg(`{"id":1,"name":"second"}`),
				msg(`{"id":2,"name":"third"}`),
				msg(`{"id":3,"name":"fourth"}`),
				msg(`{"id":4,"name":"fifth"}`),
			},
			output: []mockMsg{
				msg(`{"id":0,"name":"first"}`).withErr(errors.New("message count from branch processors does not match request, started with 4 messages, finished with 5")),
				msg(`{"id":1,"name":"second"}`).withErr(errors.New("message count from branch processors does not match request, started with 4 messages, finished with 5")),
				msg(`{"id":2,"name":"third"}`).withErr(errors.New("message count from branch processors does not match request, started with 4 messages, finished with 5")),
				msg(`{"id":3,"name":"fourth"}`).withErr(errors.New("request mapping failed: failed assignment (line 1): foo")),
				msg(`{"id":4,"name":"fifth"}`).withErr(errors.New("message count from branch processors does not match request, started with 4 messages, finished with 5")),
			},
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			procConf := NewConfig()
			procConf.Type = TypeBloblang
			procConf.Bloblang = test.processorMap

			conf := NewConfig()
			conf.Type = TypeBranch
			conf.Branch.RequestMap = test.requestMap
			conf.Branch.Processors = append(conf.Branch.Processors, procConf)
			conf.Branch.ResultMap = test.resultMap

			proc, err := New(conf, mock.NewManager(), log.Noop(), metrics.Noop())
			require.NoError(t, err)

			msg := message.QuickBatch(nil)
			for _, m := range test.input {
				part := message.NewPart([]byte(m.content))
				if m.meta != nil {
					for k, v := range m.meta {
						part.MetaSet(k, v)
					}
				}
				if m.err != nil {
					part.ErrorSet(m.err)
				}
				msg.Append(part)
			}

			outMsgs, res := proc.ProcessMessage(msg)

			require.Nil(t, res)
			require.Len(t, outMsgs, 1)

			assert.Equal(t, len(test.output), outMsgs[0].Len())
			for i, out := range test.output {
				comparePart := mockMsg{
					content: string(outMsgs[0].Get(i).Get()),
					meta:    map[string]string{},
				}

				_ = outMsgs[0].Get(i).MetaIter(func(k, v string) error {
					comparePart.meta[k] = v
					return nil
				})

				if out.err != nil {
					assert.EqualError(t, outMsgs[0].Get(i).ErrorGet(), out.err.Error())
				} else {
					assert.NoError(t, outMsgs[0].Get(i).ErrorGet())
				}
				outMsgs[0].Get(i).ErrorSet(nil)
				out.err = nil

				assert.Equal(t, out, comparePart)
			}

			// Ensure nothing changed
			for i, m := range test.input {
				doc, err := msg.Get(i).JSON()
				if err == nil {
					msg.Get(i).SetJSON(doc)
				}
				assert.Equal(t, m.content, string(msg.Get(i).Get()))
			}

			proc.CloseAsync()
			assert.NoError(t, proc.WaitForClose(time.Second))
		})
	}
}
