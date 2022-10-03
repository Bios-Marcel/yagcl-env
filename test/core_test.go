package env

import (
	"strings"
	"testing"

	"github.com/Bios-Marcel/yagcl"
	env "github.com/Bios-Marcel/yagcl-env"
	"github.com/stretchr/testify/assert"
)

func Test_EventSource_InterfaceCompliance(t *testing.T) {
	var _ yagcl.Source = env.Source()
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
		Add(env.Source()).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Equal(t, "content b", c.FieldB)
	}
}

func Test_Parse_KeyTag_NonEmpty(t *testing.T) {
	assert.NotEmpty(t, env.Source().KeyTag())
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
		Add(env.Source().Prefix("TEST")).
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
			env.Source().
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
		Add(env.Source()).
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
		Add(env.Source()).
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
		Add(env.Source()).
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
