package inputter

import (
	"fmt"
	"strings"
	"testing"

	goprompt "github.com/c-bata/go-prompt"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/ktr0731/evans/adapter/internal/testhelper"
	"github.com/ktr0731/evans/adapter/prompt"
	"github.com/ktr0731/evans/tests/helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrompt_Input(t *testing.T) {
	const prefixFormat = ">"

	t.Run("normal/simple", func(t *testing.T) {
		env := testhelper.SetupEnv(t, "helloworld.proto", "helloworld", "Greeter")

		p := helper.NewMockPrompt([]string{"rin", "shima"}, nil)
		inputter := newPrompt(p, prefixFormat)

		rpc, err := env.RPC("SayHello")
		require.NoError(t, err)

		dmsg, err := inputter.Input(rpc.RequestMessage().Desc())
		require.NoError(t, err)

		msg, ok := dmsg.(*dynamic.Message)
		require.True(t, ok)

		require.Equal(t, `name:"rin" message:"shima"`, msg.String())
	})

	t.Run("normal/nested_message", func(t *testing.T) {
		env := testhelper.SetupEnv(t, "nested.proto", "library", "Library")

		p := helper.NewMockPrompt([]string{"eriri", "spencer", "sawamura"}, nil)
		inputter := newPrompt(p, prefixFormat)

		rpc, err := env.RPC("BorrowBook")
		require.NoError(t, err)

		dmsg, err := inputter.Input(rpc.RequestMessage().Desc())
		require.NoError(t, err)

		msg, ok := dmsg.(*dynamic.Message)
		require.True(t, ok)

		require.Equal(t, `person:<name:"eriri"> book:<title:"spencer" author:"sawamura">`, msg.String())
	})

	t.Run("normal/enum", func(t *testing.T) {
		p := helper.NewMockPrompt(nil, []string{"PHILOSOPHY"})
		inputter := newPrompt(p, prefixFormat)

		env := testhelper.SetupEnv(t, "enum.proto", "library", "")
		m, err := env.Message("Book")
		require.NoError(t, err)

		dmsg, err := inputter.Input(m.Desc())
		require.NoError(t, err)

		msg, ok := dmsg.(*dynamic.Message)
		require.True(t, ok)

		require.Equal(t, `type:PHILOSOPHY`, msg.String())
	})

	t.Run("error/enum:invalid enum name", func(t *testing.T) {
		p := helper.NewMockPrompt(nil, []string{"kumiko"})
		inputter := newPrompt(p, prefixFormat)

		env := testhelper.SetupEnv(t, "enum.proto", "library", "")
		m, err := env.Message("Book")
		require.NoError(t, err)

		_, err = inputter.Input(m.Desc())
		assert.Error(t, err)
	})

	t.Run("normal/oneof", func(t *testing.T) {
		p := helper.NewMockPrompt([]string{"utaha", "kasumigaoka", "megumi", "kato"}, []string{"book", "book"})
		inputter := newPrompt(p, prefixFormat)

		env := testhelper.SetupEnv(t, "oneof.proto", "shop", "")
		m, err := env.Message("BorrowRequest")
		require.NoError(t, err)

		// Input BorrowRequest containing an oneof field.
		dmsg, err := inputter.Input(m.Desc())
		require.NoError(t, err)

		msg, ok := dmsg.(*dynamic.Message)
		require.True(t, ok)

		require.Equal(t, `book:<title:"utaha" author:"kasumigaoka">`, msg.String())

		// Input BorrowRequest again.
		dmsg, err = inputter.Input(m.Desc())
		require.NoError(t, err)

		msg, ok = dmsg.(*dynamic.Message)
		require.True(t, ok)

		require.Equal(t, `book:<title:"megumi" author:"kato">`, msg.String())
	})

	t.Run("error/oneof:invalid oneof field name", func(t *testing.T) {
		p := helper.NewMockPrompt([]string{"bar"}, []string{"Book"})
		inputter := newPrompt(p, prefixFormat)

		env := testhelper.SetupEnv(t, "oneof.proto", "shop", "")
		m, err := env.Message("BorrowRequest")
		require.NoError(t, err)

		_, err = inputter.Input(m.Desc())
		assert.Error(t, err)
	})

	t.Run("normal/repeated", func(t *testing.T) {
		p := helper.NewMockRepeatedPrompt([][]string{
			{"foo", "", "bar", "", ""},
		}, nil)

		cleanup := injectNewPrompt(p)
		defer cleanup()

		inputter := newPrompt(p, prefixFormat)

		env := testhelper.SetupEnv(t, "repeated.proto", "helloworld", "")
		m, err := env.Message("HelloRequest")
		require.NoError(t, err)

		msg, err := inputter.Input(m.Desc())
		require.NoError(t, err)

		require.Equal(t, `name:"foo" name:"bar"`, msg.String())
	})

	// In actual, maps are represented as repeated message fields.
	// See more details: https://developers.google.com/protocol-buffers/docs/proto#backwards-compatibility
	t.Run("normal/map", func(t *testing.T) {
		prompt := helper.NewMockRepeatedPrompt([][]string{
			{"foo", "", "bar", "", ""},
		}, nil)

		cleanup := injectNewPrompt(prompt)
		defer cleanup()

		inputter := newPrompt(prompt, prefixFormat)

		env := testhelper.SetupEnv(t, "map.proto", "example", "")
		m, err := env.Message("PrimitiveRequest")
		require.NoError(t, err)

		msg, err := inputter.Input(m.Desc())
		require.NoError(t, err)

		require.Equal(t, `foo:<key:"foo" value:"bar">`, msg.String())
	})

	t.Run("normal/map val is message", func(t *testing.T) {
		prompt := helper.NewMockRepeatedPrompt([][]string{
			{"key", "", "val1", "3", "", ""},
		}, nil)

		cleanup := injectNewPrompt(prompt)
		defer cleanup()

		inputter := newPrompt(prompt, prefixFormat)

		env := testhelper.SetupEnv(t, "map.proto", "example", "")
		m, err := env.Message("MessageRequest")
		require.NoError(t, err)

		msg, err := inputter.Input(m.Desc())
		require.NoError(t, err)

		require.Equal(t, `foo:<key:"key" value:<fuga:"val1" piyo:3>>`, msg.String())
	})

	t.Run("normal/circulated", func(t *testing.T) {
		prompt := helper.NewMockRepeatedPrompt(
			[][]string{{"filter1"}, {"and1"}, {"or1"}, {"or2"}, {"10", "1"}},
			[][]string{{"dig down"}, {"dig down"}, {"finish"}, {"finish"}, {"finish"}, {"dig down"}, {"finish"}, {"finish"}, {"dig down"}, {"finish"}, {"finish"}, {"finish"}})

		cleanup := injectNewPrompt(prompt)
		defer cleanup()

		inputter := newPrompt(prompt, prefixFormat)

		env := testhelper.SetupEnv(t, "circulated.proto", "example", "Example")
		m, err := env.Message("FooRequest")
		require.NoError(t, err)

		msg, err := inputter.Input(m.Desc())
		require.NoError(t, err)

		// Expected:
		//
		// {
		//   "filters": {
		//     "name": "filter1",
		//     "and": [
		//       {
		//         "name": "and1"
		//       }
		//     ],
		//     "or": [
		//       {
		//         "name": "or1"
		//       },
		//       {
		//         "name": "or2"
		//       }
		//     ]
		//   },
		//   "page": 10,
		//   "limit": 1
		// }
		require.Equal(t, `filters:<name:"filter1" and:<name:"and1"> or:<name:"or1"> or:<name:"or2">> page:10 limit:1`, msg.String())
	})
}

func Test_isCirculatedField(t *testing.T) {
	i := NewPrompt("")

	env := testhelper.SetupEnv(t, "circulated.proto", "example", "Example")

	// See circulated.proto for dependency graphs.
	cases := []struct {
		msgName        string
		fieldName      string
		isCirculated   bool
		circulatedMsgs []string
		assertFunc     func(t *testing.T, circulatedMsgs map[string][]string)
	}{
		{msgName: "A", fieldName: "b", isCirculated: true, circulatedMsgs: []string{"example.B", "example.A"}},
		{msgName: "B", fieldName: "a", isCirculated: true, circulatedMsgs: []string{"example.A", "example.B"}},

		{msgName: "Foo", fieldName: "self", isCirculated: true, circulatedMsgs: []string{"example.Self"}},
		{msgName: "Self", fieldName: "self", isCirculated: true, circulatedMsgs: []string{"example.Self"}},

		{msgName: "Hoge", fieldName: "fuga", isCirculated: true, circulatedMsgs: []string{"example.Fuga", "example.Piyo", "example.Hoge"}},
		{msgName: "Fuga", fieldName: "piyo", isCirculated: true, circulatedMsgs: []string{"example.Piyo", "example.Hoge", "example.Fuga"}},
		{msgName: "Piyo", fieldName: "hoge", isCirculated: true, circulatedMsgs: []string{"example.Hoge", "example.Fuga", "example.Piyo"}},

		{msgName: "D", fieldName: "m", isCirculated: false, assertFunc: func(t *testing.T, circulatedMsgs map[string][]string) {
			msgs, ok := circulatedMsgs["example.D.MEntry.value"]
			require.True(t, ok, "isCirculatedField must record example.D.MEntry.value as a circulated field")
			assert.Equal(t, []string{"example.C", "example.ListC"}, msgs)
		}},
		{msgName: "C", fieldName: "list", isCirculated: true, circulatedMsgs: []string{"example.ListC", "example.C"}},

		{msgName: "E", fieldName: "m1", isCirculated: true, circulatedMsgs: []string{"example.E.M1Entry", "example.F", "example.E"}},
		{msgName: "E", fieldName: "m2", isCirculated: true, circulatedMsgs: []string{"example.E.M", "example.F", "example.E"}},
		{msgName: "F", fieldName: "e", isCirculated: true, circulatedMsgs: []string{"example.E", "example.E.M1Entry", "example.E.M", "example.F"}},

		{msgName: "FooRequest", fieldName: "filters", isCirculated: true, circulatedMsgs: []string{"example.Filters"}},
		{msgName: "Filters", fieldName: "and", isCirculated: true, circulatedMsgs: []string{"example.Filters"}},
		{msgName: "Filters", fieldName: "or", isCirculated: true, circulatedMsgs: []string{"example.Filters"}},

		{msgName: "G", isCirculated: false},
		{msgName: "I", isCirculated: false},
	}
	for _, c := range cases {
		t.Run(c.msgName, func(t *testing.T) {
			m, err := env.Message(c.msgName)
			require.NoError(t, err)

			for _, f := range m.Desc().GetFields() {
				if f.GetName() != c.fieldName {
					return
				}
				assert.Equal(t, c.isCirculated, i.isCirculatedField(f))
				if c.isCirculated {
					assert.Equal(t, c.circulatedMsgs, i.state.circulatedMessages[f.GetFullyQualifiedName()])
				}
				// pp.Println(i.state.circulatedMessages)
				if c.assertFunc != nil {
					c.assertFunc(t, i.state.circulatedMessages)
				}
				return
			}
			t.Fatalf("field '%s' not found", c.fieldName)
		})
	}
}

func Test_makePrefix(t *testing.T) {
	cases := map[string]struct {
		protoName                string
		pkgName                  string // TODO: remove it.
		svcName                  string // TODO: remove it.
		msgName                  string
		fieldName                string
		ancestor                 []string
		ancestorHasRepeatedField bool
	}{
		"normal":                                 {protoName: "helloworld.proto", pkgName: "helloworld", svcName: "Greeter", msgName: "HelloRequest", fieldName: "name"},
		"nested":                                 {protoName: "helloworld.proto", pkgName: "helloworld", svcName: "Greeter", msgName: "HelloRequest", fieldName: "name", ancestor: []string{"Foo", "Bar"}},
		"repeated (repeated field)":              {protoName: "repeated.proto", pkgName: "helloworld", svcName: "Greeter", msgName: "HelloRequest", fieldName: "name", ancestor: []string{"Foo", "Bar"}},
		"repeated (ancestor has repeated field)": {protoName: "helloworld.proto", pkgName: "helloworld", svcName: "Greeter", msgName: "HelloRequest", fieldName: "name", ancestor: []string{"Foo", "Bar"}, ancestorHasRepeatedField: true},
		"repeated (both)":                        {protoName: "repeated.proto", pkgName: "helloworld", svcName: "Greeter", msgName: "HelloRequest", fieldName: "name", ancestor: []string{"Foo", "Bar"}, ancestorHasRepeatedField: true},
	}

	const prefix = "{ancestor}{name} ({type})"

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			env := testhelper.SetupEnv(t, c.protoName, c.pkgName, c.svcName)
			m, err := env.Message(c.msgName)
			require.NoError(t, err)

			for _, f := range m.Desc().GetFields() {
				if f.GetName() == c.fieldName {
					expected := fmt.Sprintf("%s (%s)", strings.Join(append(c.ancestor, f.GetName()), ancestorDelimiter), f.GetType().String())
					if f.IsRepeated() || c.ancestorHasRepeatedField {
						expected = repeatedStr + expected
					}
					actual := makePrefix(prefix, f, c.ancestor, c.ancestorHasRepeatedField)

					require.Equal(t, expected, actual)
					return
				}
			}
			t.Fatalf("field '%s' is not found", c.fieldName)
		})
	}
}

func injectNewPrompt(p prompt.Prompt) func() {
	old := prompt.New
	prompt.New = func(func(string), func(goprompt.Document) []goprompt.Suggest, ...goprompt.Option) prompt.Prompt {
		return p
	}
	return func() {
		prompt.New = old
	}
}