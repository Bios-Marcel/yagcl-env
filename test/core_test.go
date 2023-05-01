package env

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Bios-Marcel/yagcl"
	env "github.com/Bios-Marcel/yagcl-env"
	"github.com/stretchr/testify/assert"
)

func Test_EventSource_InterfaceCompliance(t *testing.T) {
	var _ yagcl.Source = env.Source().Env()
}
func Test_EnvSource_ErrNoSource(t *testing.T) {
	source, ok := env.Source().(yagcl.Source)
	if assert.True(t, ok) {
		loaded, err := source.Parse(nil, nil)
		assert.False(t, loaded)
		assert.ErrorIs(t, err, env.ErrNoDataSourceSpecified)
	}
}

func Test_EnvSource_MultipleSources(t *testing.T) {
	stepOne := env.Source()
	stepOne.Bytes([]byte{1})
	stepOne.Path("irrelevant.env")
	if source, ok := stepOne.(yagcl.Source); assert.True(t, ok) {
		loaded, err := source.Parse(nil, nil)
		assert.False(t, loaded)
		assert.ErrorIs(t, err, env.ErrMultipleDataSourcesSpecified)
	}

	stepOne = env.Source()
	stepOne.String("{}")
	stepOne.Path("irrelevant.env")
	if source, ok := stepOne.(yagcl.Source); assert.True(t, ok) {
		loaded, err := source.Parse(nil, nil)
		assert.False(t, loaded)
		assert.ErrorIs(t, err, env.ErrMultipleDataSourcesSpecified)
	}

	stepOne = env.Source()
	stepOne.Reader(bytes.NewReader([]byte{1}))
	stepOne.Path("irrelevant.env")
	if source, ok := stepOne.(yagcl.Source); assert.True(t, ok) {
		loaded, err := source.Parse(nil, nil)
		assert.False(t, loaded)
		assert.ErrorIs(t, err, env.ErrMultipleDataSourcesSpecified)
	}

	stepOne = env.Source()
	stepOne.Reader(bytes.NewReader([]byte{1}))
	stepOne.Bytes([]byte{1})
	if source, ok := stepOne.(yagcl.Source); assert.True(t, ok) {
		loaded, err := source.Parse(nil, nil)
		assert.False(t, loaded)
		assert.ErrorIs(t, err, env.ErrMultipleDataSourcesSpecified)
	}
}

func Test_Parse_Source_IsDirectory(t *testing.T) {
	stepOne := env.Source()
	stepOne.Path("./")
	if source, ok := stepOne.(yagcl.Source); assert.True(t, ok) {
		loaded, err := source.Parse(nil, nil)
		assert.False(t, loaded)
		assert.Error(t, err)
	}
}

func Test_Parse_StringSource(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
		FieldB string `env:"FIELD_B"`
	}
	var c configuration
	err := yagcl.New[configuration]().Add(env.
		Source().
		String(`
FIELD_A=content a
FIELD_B=content b
		`)).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Equal(t, "content b", c.FieldB)
	}
}

func Test_Parse_PathSource(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
		FieldB string `env:"FIELD_B"`
	}
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source().Path("./test.env")).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Equal(t, "content b", c.FieldB)
	}
}

func Test_Parse_PathSource_NotFound(t *testing.T) {
	type configuration struct{}
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source().Path("./doesntexist.env").Must()).Parse(&c)
	assert.ErrorIs(t, err, yagcl.ErrSourceNotFound)
	err = yagcl.New[configuration]().Add(env.Source().Path("./doesntexist.env")).Parse(&c)
	assert.NoError(t, err)
}

func Test_Parse_PathSource_Dir(t *testing.T) {
	type configuration struct{}
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source().Path("./").Must()).Parse(&c)
	assert.Error(t, err)
	err = yagcl.New[configuration]().Add(env.Source().Path("./")).Parse(&c)
	assert.Error(t, err)
}

func Test_Parse_ReaderSource(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
		FieldB string `env:"FIELD_B"`
	}
	var c configuration
	handle, errOpen := os.OpenFile("./test.env", os.O_RDONLY, os.ModePerm)
	if assert.NoError(t, errOpen) {
		err := yagcl.New[configuration]().Add(env.Source().Reader(handle)).Parse(&c)
		if assert.NoError(t, err) {
			assert.Equal(t, "content a", c.FieldA)
			assert.Equal(t, "content b", c.FieldB)
		}
	}
}

type failingReader struct {
	io.Reader
}

func (fr failingReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("you shall not read")
}

func Test_Parse_ReaderSource_Error(t *testing.T) {
	type configuration struct{}
	var c configuration
	assert.Error(t, yagcl.New[configuration]().Add(env.Source().Reader(&failingReader{}).Must()).Parse(&c))
	assert.Error(t, yagcl.New[configuration]().Add(env.Source().Reader(&failingReader{})).Parse(&c))
}

func Test_Parse_KeyTags(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
		FieldB string `env:"FIELD_B"`
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B", "content b")
	var c configuration
	err := yagcl.New[configuration]().
		Add(env.Source().Env()).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Equal(t, "content b", c.FieldB)
	}
}

func Test_Parse_KeyTag_NonEmpty(t *testing.T) {
	assert.NotEmpty(t, env.Source().Env().KeyTag())
}

func Test_Parse_Prefix(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
		FieldB string `env:"FIELD_B"`
	}

	t.Setenv("TEST_FIELD_A", "content a")
	t.Setenv("TEST_FIELD_B", "content b")
	var c configuration
	err := yagcl.
		New[configuration]().
		Add(env.Source().Env().Prefix("TEST")).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Equal(t, "content b", c.FieldB)
	}
}
func Test_Parse_KeyValueConverter(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
		FieldB string `env:"FIELD_B"`
	}

	t.Setenv("TEST_field_a", "content a")
	t.Setenv("TEST_FIELD_B", "content b")
	var c configuration
	err := yagcl.
		New[configuration]().
		Add(
			env.Source().Env().
				Prefix("TEST_").
				KeyValueConverter(func(s string) string {
					return s
				}),
		).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Equal(t, "content b", c.FieldB)
	}
}

func Test_Parse_MissingFieldKey(t *testing.T) {
	type configuration struct {
		FieldA string
	}

	var c configuration
	err := yagcl.
		New[configuration]().
		Add(env.Source().Env()).
		Parse(&c)
	assert.ErrorIs(t, err, yagcl.ErrExportedFieldMissingKey)
}

func Test_Parse_IgnoreField(t *testing.T) {
	type configuration struct {
		FieldA string `ignore:"true"`
	}

	var c configuration
	err := yagcl.
		New[configuration]().
		Add(env.Source().Env()).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Empty(t, c.FieldA)
	}
}

func Test_Parse_UnexportedFieldsIgnored(t *testing.T) {
	type configuration struct {
		fieldA string `key:"field_a"`
	}

	t.Setenv("FIELD_A", "content a")
	var c configuration
	err := yagcl.
		New[configuration]().
		Add(env.Source().Env()).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Empty(t, c.fieldA)
	}
}

func Test_Parse_CustomKeyJoiner(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
	}

	// The prefixed will stay uppercased, as its a hardcoded value chosen by
	// the user of the API.
	t.Setenv("KEKjoinedfielda", "content a")
	var c configuration
	err := yagcl.
		New[configuration]().
		Add(env.
			Source().
			Env().
			Prefix("KEK").
			KeyJoiner(func(s1, s2 string) string {
				return s1 + "joined" + s2
			}).
			KeyValueConverter(func(s string) string {
				return strings.ToLower(strings.Replace(s, "_", "", -1))
			})).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
	}
}
