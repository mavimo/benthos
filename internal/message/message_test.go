package message

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageSerialization(t *testing.T) {
	m := QuickBatch([][]byte{
		[]byte("hello"),
		[]byte("world"),
		[]byte("12345"),
	})

	b := ToBytes(m)

	m2, err := FromBytes(b)

	if err != nil {
		t.Error(err)
		return
	}

	if !reflect.DeepEqual(GetAllBytes(m), GetAllBytes(m2)) {
		t.Errorf("Messages not equal: %v != %v", m, m2)
	}
}

func TestNew(t *testing.T) {
	m := QuickBatch(nil)
	if act := m.Len(); act > 0 {
		t.Errorf("New returned more than zero message parts: %v", act)
	}
}

func TestIter(t *testing.T) {
	parts := [][]byte{
		[]byte(`foo`),
		[]byte(`bar`),
		[]byte(`baz`),
	}
	m := QuickBatch(parts)
	iters := 0
	_ = m.Iter(func(index int, b *Part) error {
		if exp, act := string(parts[index]), string(b.Get()); exp != act {
			t.Errorf("Unexpected part: %v != %v", act, exp)
		}
		iters++
		return nil
	})
	if exp, act := 3, iters; exp != act {
		t.Errorf("Wrong count of iterations: %v != %v", act, exp)
	}
}

func TestMessageInvalidBytesFormat(t *testing.T) {
	cases := [][]byte{
		[]byte(``),
		[]byte(`this is invalid`),
		{0x00, 0x00},
		{0x00, 0x00, 0x00, 0x05},
		{0x00, 0x00, 0x00, 0x01, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02},
		{0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x00},
	}

	for _, c := range cases {
		if _, err := FromBytes(c); err == nil {
			t.Errorf("Received nil error from invalid byte sequence: %s", c)
		}
	}
}

func TestMessageIncompleteJSON(t *testing.T) {
	tests := []struct {
		message string
		err     string
	}{
		{message: "{}"},
		{
			message: "{} not foo",
			err:     "invalid character 'o' in literal null (expecting 'u')",
		},
		{
			message: "{} {}",
			err:     "message contains multiple valid documents",
		},
		{message: `["foo"]  `},
		{message: `   ["foo"]  `},
		{message: `   ["foo"]
		
		`},
		{
			message: `   ["foo"] 
		
		
		
		{}`,
			err: "message contains multiple valid documents",
		},
	}

	for _, test := range tests {
		msg := QuickBatch([][]byte{[]byte(test.message)})
		_, err := msg.Get(0).JSON()
		if test.err == "" {
			assert.NoError(t, err)
		} else {
			assert.EqualError(t, err, test.err)
		}
	}
}

func TestMessageJSONGet(t *testing.T) {
	msg := QuickBatch(
		[][]byte{[]byte(`{"foo":{"bar":"baz"}}`)},
	)

	if _, err := msg.Get(1).JSON(); err == nil {
		t.Error("Error not returned on bad part")
	}

	jObj, err := msg.Get(0).JSON()
	if err != nil {
		t.Error(err)
	}

	exp := map[string]interface{}{
		"foo": map[string]interface{}{
			"bar": "baz",
		},
	}
	if act := jObj; !reflect.DeepEqual(act, exp) {
		t.Errorf("Wrong output from jsonGet: %v != %v", act, exp)
	}

	msg.Get(0).Set([]byte(`{"foo":{"bar":"baz2"}}`))

	jObj, err = msg.Get(0).JSON()
	if err != nil {
		t.Error(err)
	}

	exp = map[string]interface{}{
		"foo": map[string]interface{}{
			"bar": "baz2",
		},
	}
	if act := jObj; !reflect.DeepEqual(act, exp) {
		t.Errorf("Wrong output from jsonGet: %v != %v", act, exp)
	}
}

func TestMessageJSONSet(t *testing.T) {
	msg := QuickBatch([][]byte{[]byte(`hello world`)})

	msg.Get(1).SetJSON(nil)

	p1Obj := map[string]interface{}{
		"foo": map[string]interface{}{
			"bar": "baz",
		},
	}
	p1Str := `{"foo":{"bar":"baz"}}`

	p2Obj := map[string]interface{}{
		"baz": map[string]interface{}{
			"bar": "foo",
		},
	}
	p2Str := `{"baz":{"bar":"foo"}}`

	msg.Get(0).SetJSON(p1Obj)
	if exp, act := p1Str, string(msg.Get(0).Get()); exp != act {
		t.Errorf("Wrong json blob: %v != %v", act, exp)
	}

	msg.Get(0).SetJSON(p2Obj)
	if exp, act := p2Str, string(msg.Get(0).Get()); exp != act {
		t.Errorf("Wrong json blob: %v != %v", act, exp)
	}

	msg.Get(0).SetJSON(p1Obj)
	if exp, act := p1Str, string(msg.Get(0).Get()); exp != act {
		t.Errorf("Wrong json blob: %v != %v", act, exp)
	}
}

func TestMessageMetadata(t *testing.T) {
	m := QuickBatch([][]byte{
		[]byte("foo"),
		[]byte("bar"),
	})

	m.Get(0).MetaSet("foo", "bar")
	if exp, act := "bar", m.Get(0).MetaGet("foo"); exp != act {
		t.Errorf("Wrong result: %v != %v", act, exp)
	}

	m.Get(0).MetaSet("foo", "bar2")
	if exp, act := "bar2", m.Get(0).MetaGet("foo"); exp != act {
		t.Errorf("Wrong result: %v != %v", act, exp)
	}

	m.Get(0).MetaSet("bar", "baz")
	m.Get(0).MetaSet("baz", "qux")

	exp := map[string]string{
		"foo": "bar2",
		"bar": "baz",
		"baz": "qux",
	}
	act := map[string]string{}
	require.NoError(t, m.Get(0).MetaIter(func(k, v string) error {
		act[k] = v
		return nil
	}))
	if !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong result: %s != %s", act, exp)
	}
}

func TestMessageCopy(t *testing.T) {
	m := QuickBatch([][]byte{
		[]byte(`foo`),
		[]byte(`bar`),
	})
	m.Get(0).MetaSet("foo", "bar")

	m2 := m.Copy()
	if exp, act := [][]byte{[]byte(`foo`), []byte(`bar`)}, GetAllBytes(m2); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong result: %s != %s", act, exp)
	}
	if exp, act := "bar", m2.Get(0).MetaGet("foo"); exp != act {
		t.Errorf("Wrong result: %v != %v", act, exp)
	}

	m2.Get(0).MetaSet("foo", "bar2")
	m2.Get(0).Set([]byte(`baz`))
	if exp, act := [][]byte{[]byte(`baz`), []byte(`bar`)}, GetAllBytes(m2); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong result: %s != %s", act, exp)
	}
	if exp, act := "bar2", m2.Get(0).MetaGet("foo"); exp != act {
		t.Errorf("Wrong result: %v != %v", act, exp)
	}
	if exp, act := [][]byte{[]byte(`foo`), []byte(`bar`)}, GetAllBytes(m); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong result: %s != %s", act, exp)
	}
	if exp, act := "bar", m.Get(0).MetaGet("foo"); exp != act {
		t.Errorf("Wrong result: %v != %v", act, exp)
	}
}

func TestMessageErrors(t *testing.T) {
	p1 := NewPart([]byte("foo"))
	assert.NoError(t, p1.ErrorGet())

	p2 := p1.WithContext(context.Background())
	assert.NoError(t, p2.ErrorGet())

	p3 := p2.Copy()
	assert.NoError(t, p3.ErrorGet())

	p1.ErrorSet(errors.New("err1"))
	assert.EqualError(t, p1.ErrorGet(), "err1")
	assert.EqualError(t, p2.ErrorGet(), "err1")
	assert.NoError(t, p3.ErrorGet())

	p2.ErrorSet(errors.New("err2"))
	assert.EqualError(t, p1.ErrorGet(), "err2")
	assert.EqualError(t, p2.ErrorGet(), "err2")
	assert.NoError(t, p3.ErrorGet())

	p3.ErrorSet(errors.New("err3"))
	assert.EqualError(t, p1.ErrorGet(), "err2")
	assert.EqualError(t, p2.ErrorGet(), "err2")
	assert.EqualError(t, p3.ErrorGet(), "err3")
}

func TestMessageDeepCopy(t *testing.T) {
	m := QuickBatch([][]byte{
		[]byte(`foo`),
		[]byte(`bar`),
	})
	m.Get(0).MetaSet("foo", "bar")

	m2 := m.DeepCopy()
	if exp, act := [][]byte{[]byte(`foo`), []byte(`bar`)}, GetAllBytes(m2); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong result: %s != %s", act, exp)
	}
	if exp, act := "bar", m2.Get(0).MetaGet("foo"); exp != act {
		t.Errorf("Wrong result: %v != %v", act, exp)
	}

	m2.Get(0).MetaSet("foo", "bar2")
	m2.Get(0).Set([]byte(`baz`))
	if exp, act := [][]byte{[]byte(`baz`), []byte(`bar`)}, GetAllBytes(m2); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong result: %s != %s", act, exp)
	}
	if exp, act := "bar2", m2.Get(0).MetaGet("foo"); exp != act {
		t.Errorf("Wrong result: %v != %v", act, exp)
	}
	if exp, act := [][]byte{[]byte(`foo`), []byte(`bar`)}, GetAllBytes(m); !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong result: %s != %s", act, exp)
	}
	if exp, act := "bar", m.Get(0).MetaGet("foo"); exp != act {
		t.Errorf("Wrong result: %v != %v", act, exp)
	}
}

func TestMessageJSONSetGet(t *testing.T) {
	msg := QuickBatch([][]byte{[]byte(`hello world`)})

	p1Obj := map[string]interface{}{
		"foo": map[string]interface{}{
			"bar": "baz",
		},
	}
	p1Str := `{"foo":{"bar":"baz"}}`

	p2Obj := map[string]interface{}{
		"baz": map[string]interface{}{
			"bar": "foo",
		},
	}
	p2Str := `{"baz":{"bar":"foo"}}`

	var err error
	var jObj interface{}

	msg.Get(0).SetJSON(p1Obj)
	if exp, act := p1Str, string(msg.Get(0).Get()); exp != act {
		t.Errorf("Wrong json blob: %v != %v", act, exp)
	}
	if jObj, err = msg.Get(0).JSON(); err != nil {
		t.Fatal(err)
	}
	if exp, act := p1Obj, jObj; !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong json obj: %v != %v", act, exp)
	}

	msg.Get(0).SetJSON(p2Obj)
	if exp, act := p2Str, string(msg.Get(0).Get()); exp != act {
		t.Errorf("Wrong json blob: %v != %v", act, exp)
	}
	if jObj, err = msg.Get(0).JSON(); err != nil {
		t.Fatal(err)
	}
	if exp, act := p2Obj, jObj; !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong json obj: %v != %v", act, exp)
	}

	msg.Get(0).SetJSON(p1Obj)
	if exp, act := p1Str, string(msg.Get(0).Get()); exp != act {
		t.Errorf("Wrong json blob: %v != %v", act, exp)
	}
	if jObj, err = msg.Get(0).JSON(); err != nil {
		t.Fatal(err)
	}
	if exp, act := p1Obj, jObj; !reflect.DeepEqual(exp, act) {
		t.Errorf("Wrong json obj: %v != %v", act, exp)
	}
}

func TestMessageSplitJSON(t *testing.T) {
	msg1 := QuickBatch([][]byte{
		[]byte("Foo plain text"),
		[]byte(`nothing here`),
	})

	msg1.Get(1).SetJSON(map[string]interface{}{"foo": "bar"})
	msg2 := msg1.Copy()

	if exp, act := GetAllBytes(msg1), GetAllBytes(msg2); !reflect.DeepEqual(exp, act) {
		t.Errorf("Parts unmatched from shallow copy: %s != %s", act, exp)
	}

	msg2.Get(0).Set([]byte("Bar different text"))

	if exp, act := "Foo plain text", string(msg1.Get(0).Get()); exp != act {
		t.Errorf("Original content was changed from shallow copy: %v != %v", act, exp)
	}

	msg1.Get(1).SetJSON(map[string]interface{}{"foo": "baz"})
	jCont, err := msg1.Get(1).JSON()
	if err != nil {
		t.Fatal(err)
	}
	if exp, act := map[string]interface{}{"foo": "baz"}, jCont; !reflect.DeepEqual(exp, act) {
		t.Errorf("Unexpected json content: %v != %v", exp, act)
	}
	if exp, act := `{"foo":"baz"}`, string(msg1.Get(1).Get()); exp != act {
		t.Errorf("Unexpected original content: %v != %v", act, exp)
	}

	jCont, err = msg2.Get(1).JSON()
	if err != nil {
		t.Fatal(err)
	}
	if exp, act := map[string]interface{}{"foo": "bar"}, jCont; !reflect.DeepEqual(exp, act) {
		t.Errorf("Unexpected json content: %v != %v", exp, act)
	}
	if exp, act := `{"foo":"bar"}`, string(msg2.Get(1).Get()); exp != act {
		t.Errorf("Unexpected shallow content: %v != %v", act, exp)
	}

	msg2.Get(1).SetJSON(map[string]interface{}{"foo": "baz2"})
	jCont, err = msg2.Get(1).JSON()
	if err != nil {
		t.Fatal(err)
	}
	if exp, act := map[string]interface{}{"foo": "baz2"}, jCont; !reflect.DeepEqual(exp, act) {
		t.Errorf("Unexpected json content: %v != %v", exp, act)
	}
	if exp, act := `{"foo":"baz2"}`, string(msg2.Get(1).Get()); exp != act {
		t.Errorf("Unexpected shallow copy content: %v != %v", act, exp)
	}

	jCont, err = msg1.Get(1).JSON()
	if err != nil {
		t.Fatal(err)
	}
	if exp, act := map[string]interface{}{"foo": "baz"}, jCont; !reflect.DeepEqual(exp, act) {
		t.Errorf("Unexpected original json content: %v != %v", exp, act)
	}
	if exp, act := `{"foo":"baz"}`, string(msg1.Get(1).Get()); exp != act {
		t.Errorf("Unexpected original content: %v != %v", act, exp)
	}
}

func TestMessageCrossContaminateJSON(t *testing.T) {
	msg1 := QuickBatch([][]byte{
		[]byte(`{"foo":"bar"}`),
	})

	var jCont1, jCont2 interface{}
	var err error

	if jCont1, err = msg1.Get(0).JSON(); err != nil {
		t.Fatal(err)
	}

	msg2 := msg1.DeepCopy()

	jMap1, ok := jCont1.(map[string]interface{})
	if !ok {
		t.Fatal("Couldnt cast to map")
	}
	jMap1["foo"] = "baz"

	msg1.Get(0).SetJSON(jMap1)
	if jCont1, err = msg1.Get(0).JSON(); err != nil {
		t.Fatal(err)
	}
	if exp, act := map[string]interface{}{"foo": "baz"}, jCont1; !reflect.DeepEqual(exp, act) {
		t.Errorf("Unexpected json content: %v != %v", exp, act)
	}
	if exp, act := `{"foo":"baz"}`, string(msg1.Get(0).Get()); exp != act {
		t.Errorf("Unexpected raw content: %v != %v", exp, act)
	}

	if jCont2, err = msg2.Get(0).JSON(); err != nil {
		t.Fatal(err)
	}
	if exp, act := map[string]interface{}{"foo": "bar"}, jCont2; !reflect.DeepEqual(exp, act) {
		t.Errorf("Unexpected json content: %v != %v", exp, act)
	}
	if exp, act := `{"foo":"bar"}`, string(msg2.Get(0).Get()); exp != act {
		t.Errorf("Unexpected raw content: %v != %v", exp, act)
	}
}

func BenchmarkJSONGet(b *testing.B) {
	sample1 := []byte(`{
	"foo":{
		"bar":"baz",
		"this":{
			"will":{
				"be":{
					"very":{
						"nested":true
					}
				},
				"dont_forget":"me"
			},
			"dont_forget":"me"
		},
		"dont_forget":"me"
	},
	"numbers": [0,1,2,3,4,5,6,7]
}`)
	sample2 := []byte(`{
	"foo2":{
		"bar":"baz2",
		"this":{
			"will":{
				"be":{
					"very":{
						"nested":false
					}
				},
				"dont_forget":"me too"
			},
			"dont_forget":"me too"
		},
		"dont_forget":"me too"
	},
	"numbers": [0,1,2,3,4,5,6,7]
}`)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := QuickBatch([][]byte{sample1})

		jObj, err := msg.Get(0).JSON()
		if err != nil {
			b.Error(err)
		}
		if _, ok := jObj.(map[string]interface{}); !ok {
			b.Error("Couldn't cast to map")
		}

		jObj, err = msg.Get(0).JSON()
		if err != nil {
			b.Error(err)
		}
		if _, ok := jObj.(map[string]interface{}); !ok {
			b.Error("Couldn't cast to map")
		}

		msg.Get(0).Set(sample2)

		jObj, err = msg.Get(0).JSON()
		if err != nil {
			b.Error(err)
		}
		if _, ok := jObj.(map[string]interface{}); !ok {
			b.Error("Couldn't cast to map")
		}
	}
}
