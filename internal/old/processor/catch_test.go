package processor

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/manager/mock"
	"github.com/benthosdev/benthos/v4/internal/message"
)

//------------------------------------------------------------------------------

func TestCatchEmpty(t *testing.T) {
	conf := NewConfig()
	conf.Type = TypeCatch

	proc, err := New(conf, mock.NewManager(), log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}

	exp := [][]byte{
		[]byte("foo bar baz"),
	}
	msgs, res := proc.ProcessMessage(message.QuickBatch(exp))
	if res != nil {
		t.Fatal(res)
	}

	if len(msgs) != 1 {
		t.Errorf("Wrong count of result msgs: %v", len(msgs))
	}
	if act := message.GetAllBytes(msgs[0]); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong results: %s != %s", act, exp)
	}
	_ = msgs[0].Iter(func(i int, p *message.Part) error {
		if p.ErrorGet() != nil {
			t.Errorf("Unexpected part %v failed flag", i)
		}
		return nil
	})
}

func TestCatchBasic(t *testing.T) {
	encodeConf := NewConfig()
	encodeConf.Type = TypeBloblang
	encodeConf.Bloblang = `root = if batch_index() == 0 { content().encode("base64") }`

	conf := NewConfig()
	conf.Type = TypeCatch
	conf.Catch = append(conf.Catch, encodeConf)

	proc, err := New(conf, mock.NewManager(), log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}

	parts := [][]byte{
		[]byte("foo bar baz"),
		[]byte("1 2 3 4"),
		[]byte("hello foo world"),
	}
	exp := [][]byte{
		[]byte("Zm9vIGJhciBiYXo="),
		[]byte("MSAyIDMgNA=="),
		[]byte("aGVsbG8gZm9vIHdvcmxk"),
	}

	msg := message.QuickBatch(parts)
	_ = msg.Iter(func(i int, p *message.Part) error {
		p.ErrorSet(errors.New("foo"))
		return nil
	})
	msgs, res := proc.ProcessMessage(msg)
	if res != nil {
		t.Fatal(res)
	}

	if len(msgs) != 1 {
		t.Errorf("Wrong count of result msgs: %v", len(msgs))
	}
	if act := message.GetAllBytes(msgs[0]); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong results: %s != %s", act, exp)
	}
	_ = msgs[0].Iter(func(i int, p *message.Part) error {
		if p.ErrorGet() != nil {
			t.Errorf("Unexpected part %v failed flag", i)
		}
		return nil
	})
}

func TestCatchFilterSome(t *testing.T) {
	filterConf := NewConfig()
	filterConf.Type = TypeBloblang
	filterConf.Bloblang = `root = if !content().contains("foo") { deleted() }`

	conf := NewConfig()
	conf.Type = TypeCatch
	conf.Catch = append(conf.Catch, filterConf)

	proc, err := New(conf, mock.NewManager(), log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}

	parts := [][]byte{
		[]byte("foo bar baz"),
		[]byte("1 2 3 4"),
		[]byte("hello foo world"),
	}
	exp := [][]byte{
		[]byte("foo bar baz"),
		[]byte("hello foo world"),
	}
	msg := message.QuickBatch(parts)
	_ = msg.Iter(func(i int, p *message.Part) error {
		p.ErrorSet(errors.New("foo"))
		return nil
	})
	msgs, res := proc.ProcessMessage(msg)
	if res != nil {
		t.Fatal(res)
	}

	if len(msgs) != 1 {
		t.Errorf("Wrong count of result msgs: %v", len(msgs))
	}
	if act := message.GetAllBytes(msgs[0]); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong results: %s != %s", act, exp)
	}
	_ = msgs[0].Iter(func(i int, p *message.Part) error {
		if p.ErrorGet() != nil {
			t.Errorf("Unexpected part %v failed flag", i)
		}
		return nil
	})
}

func TestCatchMultiProcs(t *testing.T) {
	encodeConf := NewConfig()
	encodeConf.Type = TypeBloblang
	encodeConf.Bloblang = `root = if batch_index() == 0 { content().encode("base64") }`

	filterConf := NewConfig()
	filterConf.Type = TypeBloblang
	filterConf.Bloblang = `root = if !content().contains("foo") { deleted() }`

	conf := NewConfig()
	conf.Type = TypeCatch
	conf.Catch = append(conf.Catch, filterConf, encodeConf)

	proc, err := New(conf, mock.NewManager(), log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}

	parts := [][]byte{
		[]byte("foo bar baz"),
		[]byte("1 2 3 4"),
		[]byte("hello foo world"),
	}
	exp := [][]byte{
		[]byte("Zm9vIGJhciBiYXo="),
		[]byte("aGVsbG8gZm9vIHdvcmxk"),
	}
	msg := message.QuickBatch(parts)
	_ = msg.Iter(func(i int, p *message.Part) error {
		p.ErrorSet(errors.New("foo"))
		return nil
	})
	msgs, res := proc.ProcessMessage(msg)
	if res != nil {
		t.Fatal(res)
	}

	if len(msgs) != 1 {
		t.Errorf("Wrong count of result msgs: %v", len(msgs))
	}
	if act := message.GetAllBytes(msgs[0]); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong results: %s != %s", act, exp)
	}
	_ = msgs[0].Iter(func(i int, p *message.Part) error {
		if p.ErrorGet() != nil {
			t.Errorf("Unexpected part %v failed flag", i)
		}
		return nil
	})
}

func TestCatchNotFails(t *testing.T) {
	encodeConf := NewConfig()
	encodeConf.Type = TypeBloblang
	encodeConf.Bloblang = `root = if batch_index() == 0 { content().encode("base64") }`

	conf := NewConfig()
	conf.Type = TypeCatch
	conf.Catch = append(conf.Catch, encodeConf)

	proc, err := New(conf, mock.NewManager(), log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}

	parts := [][]byte{
		[]byte(`FAILED ENCODE ME PLEASE`),
		[]byte("NOT FAILED, DO NOT ENCODE"),
		[]byte(`FAILED ENCODE ME PLEASE 2`),
	}
	exp := [][]byte{
		[]byte("RkFJTEVEIEVOQ09ERSBNRSBQTEVBU0U="),
		[]byte("NOT FAILED, DO NOT ENCODE"),
		[]byte("RkFJTEVEIEVOQ09ERSBNRSBQTEVBU0UgMg=="),
	}
	msg := message.QuickBatch(parts)
	msg.Get(0).ErrorSet(errors.New("foo"))
	msg.Get(2).ErrorSet(errors.New("foo"))
	msgs, res := proc.ProcessMessage(msg)
	if res != nil {
		t.Fatal(res)
	}

	if len(msgs) != 1 {
		t.Errorf("Wrong count of result msgs: %v", len(msgs))
	}
	if act := message.GetAllBytes(msgs[0]); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong results: %s != %s", act, exp)
	}
	_ = msgs[0].Iter(func(i int, p *message.Part) error {
		if p.ErrorGet() != nil {
			t.Errorf("Unexpected part %v failed flag", i)
		}
		return nil
	})
}

func TestCatchFilterAll(t *testing.T) {
	filterConf := NewConfig()
	filterConf.Type = TypeBloblang
	filterConf.Bloblang = `root = if !content().contains("foo") { deleted() }`

	conf := NewConfig()
	conf.Type = TypeCatch
	conf.Catch = append(conf.Catch, filterConf)

	proc, err := New(conf, mock.NewManager(), log.Noop(), metrics.Noop())
	if err != nil {
		t.Fatal(err)
	}

	parts := [][]byte{
		[]byte("bar baz"),
		[]byte("1 2 3 4"),
		[]byte("hello world"),
	}
	msg := message.QuickBatch(parts)
	_ = msg.Iter(func(i int, p *message.Part) error {
		p.ErrorSet(errors.New("foo"))
		return nil
	})
	msgs, res := proc.ProcessMessage(msg)
	assert.NoError(t, res)
	if len(msgs) != 0 {
		t.Errorf("Wrong count of result msgs: %v", len(msgs))
	}
}

//------------------------------------------------------------------------------
