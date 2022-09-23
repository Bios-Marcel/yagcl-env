package env

import (
	"encoding"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Bios-Marcel/yagcl"
	env "github.com/Bios-Marcel/yagcl-env"
	"github.com/stretchr/testify/assert"
)

func Test_Parse_String_Valid(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
	}

	t.Setenv("FIELD_A", "content a")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
	}
}

func Test_Parse_Duration(t *testing.T) {
	type configuration struct {
		FieldA time.Duration `key:"field_a"`
	}

	t.Setenv("FIELD_A", "10s")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, time.Second*10, c.FieldA)
	}
}

func Test_Parse_Struct(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
		FieldB struct {
			FieldC string `key:"field_c"`
		} `key:"field_b"`
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C", "content c")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Equal(t, "content c", c.FieldB.FieldC)
	}
}

func Test_Parse_DeeplyNested_Struct(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
		FieldB struct {
			FieldC struct {
				FieldD struct {
					FieldE struct {
						FieldF struct {
							FieldG string `key:"field_g"`
						} `key:"field_f"`
					} `key:"field_e"`
				} `key:"field_d"`
			} `key:"field_c"`
		} `key:"field_b"`
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C_FIELD_D_FIELD_E_FIELD_F_FIELD_G", "content c")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Equal(t, "content c", c.FieldB.FieldC.FieldD.FieldE.FieldF.FieldG)
	}
}

func Test_Parse_SimplePointer(t *testing.T) {
	type configuration struct {
		FieldA *uint `key:"field_a"`
	}

	t.Setenv("FIELD_A", "10")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, uint(10), *c.FieldA)
	}
}

func Test_Parse_DoublePointer(t *testing.T) {
	type configuration struct {
		FieldA **uint `key:"field_a"`
	}

	t.Setenv("FIELD_A", "10")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, uint(10), **c.FieldA)
	}
}

func Test_Parse_PointerOfDoom(t *testing.T) {
	type configuration struct {
		FieldA ***************************************uint `key:"field_a"`
	}

	t.Setenv("FIELD_A", "10")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, uint(10), ***************************************c.FieldA)
	}
}

func Test_Parse_SinglePointerToStruct(t *testing.T) {
	type substruct struct {
		FieldC string `key:"field_c"`
	}
	type configuration struct {
		FieldA string     `key:"field_a"`
		FieldB *substruct `key:"field_b"`
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C", "content c")
	var c configuration
	c.FieldB = &substruct{}
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Equal(t, "content c", (*c.FieldB).FieldC)
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C", "")
	c = configuration{}
	c.FieldB = &substruct{}
	err = yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.NotNil(t, "content c", *c.FieldB)
	}
}

func Test_Parse_SinglePointerToStruct_Invalid(t *testing.T) {
	type substruct struct {
		FieldC int `key:"field_c"`
	}
	type configuration struct {
		FieldA string     `key:"field_a"`
		FieldB *substruct `key:"field_b"`
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C", "ain't no integer here buddy")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	assert.ErrorIs(t, err, yagcl.ErrParseValue)
}

func Test_Parse_Struct_Invalid(t *testing.T) {
	type substruct struct {
		FieldC int `key:"field_c"`
	}
	type configuration struct {
		FieldA string    `key:"field_a"`
		FieldB substruct `key:"field_b"`
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C", "ain't no integer here buddy")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	assert.ErrorIs(t, err, yagcl.ErrParseValue)
}

func Test_Parse_SingleNilPointerToStruct(t *testing.T) {
	type substruct struct {
		FieldC string `key:"field_c"`
	}
	type configuration struct {
		FieldA string     `key:"field_a"`
		FieldB *substruct `key:"field_b"`
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C", "content c")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Equal(t, "content c", (*c.FieldB).FieldC)
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C", "")
	c = configuration{}
	err = yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Nil(t, c.FieldB)
	}
}

func Test_Parse_PointerOfDoomToStruct(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
		FieldB **************struct {
			FieldC string `key:"field_c"`
		} `key:"field_b"`
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C", "content c")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Equal(t, "content c", (**************c.FieldB).FieldC)
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C", "")
	c = configuration{}
	err = yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Nil(t, c.FieldB)
	}
}

func Test_Parse_NestedPointerOfDoomToStruct(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
		FieldB **************struct {
			FieldC **************struct {
				FieldD **************struct {
					FieldE string `key:"field_e"`
				} `key:"field_d"`
			} `key:"field_c"`
		} `key:"field_b"`
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C_FIELD_D_FIELD_E", "content c")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		fieldC := (**************c.FieldB).FieldC
		fieldD := (**************fieldC).FieldD
		fieldE := (**************fieldD).FieldE
		assert.Equal(t, "content c", fieldE)
	}

	t.Setenv("FIELD_A", "content a")
	t.Setenv("FIELD_B_FIELD_C_FIELD_D_FIELD_E", "")
	c = configuration{}
	err = yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "content a", c.FieldA)
		assert.Nil(t, c.FieldB)
	}
}

func Test_Parse_String_Whitespace(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
	}

	t.Setenv("FIELD_A", "   ")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "   ", c.FieldA)
	}
}

func Test_Parse_Bool_Valid(t *testing.T) {
	type configuration struct {
		True       bool `key:"true"`
		False      bool `key:"false"`
		TrueUpper  bool `key:"true_upper"`
		FalseUpper bool `key:"false_upper"`
	}

	t.Setenv("TRUE", "true")
	t.Setenv("FALSE", "false")
	t.Setenv("TRUE_UPPER", "TRUE")
	t.Setenv("FALSE_UPPER", "FALSE")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, true, c.True)
		assert.Equal(t, false, c.False)
		assert.Equal(t, true, c.TrueUpper)
		assert.Equal(t, false, c.FalseUpper)
	}
}

func Test_Parse_Bool_Invalid(t *testing.T) {
	type configuration struct {
		Bool bool `key:"bool"`
	}

	t.Setenv("BOOL", "cheese")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	assert.ErrorIs(t, err, yagcl.ErrParseValue)
}

func Test_Parse_Complex64_Unsupported(t *testing.T) {
	type configuration struct {
		FieldA complex64 `key:"field_a"`
	}

	t.Setenv("FIELD_A", "value")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	assert.ErrorIs(t, err, yagcl.ErrUnsupportedFieldType)
}

func Test_Parse_Complex128_Unsupported(t *testing.T) {
	type configuration struct {
		FieldA complex128 `key:"field_a"`
	}

	t.Setenv("FIELD_A", "value")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	assert.ErrorIs(t, err, yagcl.ErrUnsupportedFieldType)
}

func Test_Parse_Int_Valid(t *testing.T) {
	type configuration struct {
		Min int `key:"min"`
		Max int `key:"max"`
	}

	t.Setenv("MIN", fmt.Sprintf("%d", math.MinInt))
	t.Setenv("MAX", fmt.Sprintf("%d", math.MaxInt))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, math.MinInt, c.Min)
		assert.Equal(t, math.MaxInt, c.Max)
	}
}

func Test_Parse_Int8_Valid(t *testing.T) {
	type configuration struct {
		Min int8 `key:"min"`
		Max int8 `key:"max"`
	}

	t.Setenv("MIN", fmt.Sprintf("%d", math.MinInt8))
	t.Setenv("MAX", fmt.Sprintf("%d", math.MaxInt8))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, int8(math.MinInt8), c.Min)
		assert.Equal(t, int8(math.MaxInt8), c.Max)
	}
}

func Test_Parse_Int16_Valid(t *testing.T) {
	type configuration struct {
		Min int16 `key:"min"`
		Max int16 `key:"max"`
	}

	t.Setenv("MIN", fmt.Sprintf("%d", math.MinInt16))
	t.Setenv("MAX", fmt.Sprintf("%d", math.MaxInt16))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, int16(math.MinInt16), c.Min)
		assert.Equal(t, int16(math.MaxInt16), c.Max)
	}
}

func Test_Parse_Int32_Valid(t *testing.T) {
	type configuration struct {
		Min int32 `key:"min"`
		Max int32 `key:"max"`
	}

	t.Setenv("MIN", fmt.Sprintf("%d", math.MinInt32))
	t.Setenv("MAX", fmt.Sprintf("%d", math.MaxInt32))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, int32(math.MinInt32), c.Min)
		assert.Equal(t, int32(math.MaxInt32), c.Max)
	}
}

func Test_Parse_Int64_Valid(t *testing.T) {
	type configuration struct {
		Min int64 `key:"min"`
		Max int64 `key:"max"`
	}

	t.Setenv("MIN", fmt.Sprintf("%d", math.MinInt64))
	t.Setenv("MAX", fmt.Sprintf("%d", math.MaxInt64))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, int64(math.MinInt64), c.Min)
		assert.Equal(t, int64(math.MaxInt64), c.Max)
	}
}

func Test_Parse_Uint_Valid(t *testing.T) {
	type configuration struct {
		Min uint `key:"min"`
		Max uint `key:"max"`
	}

	t.Setenv("MIN", "0")
	t.Setenv("MAX", fmt.Sprintf("%d", uint(math.MaxUint)))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, uint(0), c.Min)
		assert.Equal(t, uint(math.MaxUint), c.Max)
	}
}

func Test_Parse_Uint8_Valid(t *testing.T) {
	type configuration struct {
		Min uint8 `key:"min"`
		Max uint8 `key:"max"`
	}

	t.Setenv("MIN", "0")
	t.Setenv("MAX", fmt.Sprintf("%d", uint8(math.MaxUint8)))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, uint8(0), c.Min)
		assert.Equal(t, uint8(math.MaxUint8), c.Max)
	}
}

func Test_Parse_Uint16_Valid(t *testing.T) {
	type configuration struct {
		Min uint16 `key:"min"`
		Max uint16 `key:"max"`
	}

	t.Setenv("MIN", "0")
	t.Setenv("MAX", fmt.Sprintf("%d", uint16(math.MaxUint16)))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, uint16(0), c.Min)
		assert.Equal(t, uint16(math.MaxUint16), c.Max)
	}
}

func Test_Parse_Uint32_Valid(t *testing.T) {
	type configuration struct {
		Min uint32 `key:"min"`
		Max uint32 `key:"max"`
	}

	t.Setenv("MIN", "0")
	t.Setenv("MAX", fmt.Sprintf("%d", uint32(math.MaxUint32)))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, uint32(0), c.Min)
		assert.Equal(t, uint32(math.MaxUint32), c.Max)
	}
}

func Test_Parse_Uint64_Valid(t *testing.T) {
	type configuration struct {
		Min uint64 `key:"min"`
		Max uint64 `key:"max"`
	}

	t.Setenv("MIN", "0")
	t.Setenv("MAX", fmt.Sprintf("%d", uint64(math.MaxUint64)))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, uint64(0), c.Min)
		assert.Equal(t, uint64(math.MaxUint64), c.Max)
	}
}

func Test_Parse_Float32_Valid(t *testing.T) {
	type configuration struct {
		Float float32 `key:"float"`
	}

	var floatValue float32 = 5.5
	bytes, _ := json.Marshal(floatValue)
	t.Setenv("FLOAT", string(bytes))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, floatValue, c.Float)
	}
}

func Test_Parse_Float64_Valid(t *testing.T) {
	type configuration struct {
		Float float64 `key:"float"`
	}

	var floatValue float64 = 5.5
	bytes, _ := json.Marshal(floatValue)
	t.Setenv("FLOAT", string(bytes))
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, floatValue, c.Float)
	}
}

func Test_Parse_Float32_Invalid(t *testing.T) {
	type configuration struct {
		Float float32 `key:"float"`
	}

	t.Setenv("FLOAT", "5.5no float here")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	assert.ErrorIs(t, err, yagcl.ErrParseValue)
}

func Test_Parse_Float64_Invalid(t *testing.T) {
	type configuration struct {
		Float float64 `key:"float"`
	}

	t.Setenv("FLOAT", "5.5no float here")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	assert.ErrorIs(t, err, yagcl.ErrParseValue)
}

func Test_Parse_Int_Invalid(t *testing.T) {
	type configuration struct {
		FieldA int `key:"field_a"`
	}

	t.Setenv("FIELD_A", "10no int here")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	assert.ErrorIs(t, err, yagcl.ErrParseValue)
}

func Test_Parse_Uint_Invalid(t *testing.T) {
	type configuration struct {
		FieldA uint `key:"field_a"`
	}

	t.Setenv("FIELD_A", "10no int here")
	var c configuration
	err := yagcl.New[configuration]().Add(env.Source()).Parse(&c)
	assert.ErrorIs(t, err, yagcl.ErrParseValue)
}

func Test_Parse_DefaultValue_String(t *testing.T) {
	type configuration struct {
		FieldA string `key:"field_a"`
	}

	c := configuration{
		FieldA: "i am the default",
	}
	err := yagcl.
		New[configuration]().
		Add(env.Source()).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, "i am the default", c.FieldA)
	}
}

func Test_Parse_DefaultValue_Int(t *testing.T) {
	type configuration struct {
		FieldA int `key:"field_a"`
	}

	c := configuration{
		FieldA: 1,
	}
	err := yagcl.
		New[configuration]().
		Add(env.Source()).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, 1, c.FieldA)
	}
}

type customTextUnmarshalable string

func (uc *customTextUnmarshalable) UnmarshalText(data []byte) error {
	*uc = customTextUnmarshalable(strings.ToUpper(string(data)))
	return nil
}

func (uc customTextUnmarshalable) String() string {
	return string(uc)
}

func Test_CustomTextUnmarshaler_InterfaceCompliance(t *testing.T) {
	var temp = customTextUnmarshalable("")
	var _ encoding.TextUnmarshaler = &temp
}

func Test_Parse_CustomTextUnmarshaler(t *testing.T) {
	type configuration struct {
		FieldA customTextUnmarshalable `key:"field_a"`
	}

	t.Setenv("FIELD_A", "lower")
	var c configuration
	err := yagcl.New[configuration]().
		Add(env.Source()).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, customTextUnmarshalable("LOWER"), c.FieldA)
	}
}

func Test_Parse_CustomTextUnmarshaler_Nested(t *testing.T) {
	type thing struct {
		FieldA customTextUnmarshalable `key:"field_a"`
	}
	type configuration struct {
		Thing thing `key:"thing"`
	}

	t.Setenv("THING_FIELD_A", "lower")
	var c configuration
	err := yagcl.New[configuration]().
		Add(env.Source()).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, customTextUnmarshalable("LOWER"), c.Thing.FieldA)
	}
}

//TODO Write test for struct with multiplie fields that get ignored due to the
//struct already being parsed.

func Test_Parse_CustomTextUnmarshaler_Pointers(t *testing.T) {
	t.Run("single pointer", func(t *testing.T) {
		type configuration struct {
			FieldA *customTextUnmarshalable `key:"field_a"`
		}

		t.Setenv("FIELD_A", "lower")
		var c configuration
		err := yagcl.New[configuration]().
			Add(env.Source()).
			Parse(&c)
		if assert.NoError(t, err) {
			assert.Equal(t, customTextUnmarshalable("LOWER"), *c.FieldA)
		}
	})

	t.Run("multi pointer", func(t *testing.T) {
		type configuration struct {
			FieldA ***customTextUnmarshalable `key:"field_a"`
		}

		t.Setenv("FIELD_A", "lower")
		var c configuration
		err := yagcl.New[configuration]().
			Add(env.Source()).
			Parse(&c)
		if assert.NoError(t, err) {
			assert.Equal(t, customTextUnmarshalable("LOWER"), ***c.FieldA)
		}
	})
}

func Test_Parse_TypeAlias_NoCustomUnmarshal(t *testing.T) {
	type noopstring string
	type configuration struct {
		FieldA noopstring `key:"field_a"`
	}
	t.Setenv("FIELD_A", "lower")
	var c configuration
	err := yagcl.New[configuration]().
		Add(env.Source()).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, noopstring("lower"), c.FieldA)
	}
}

func Test_Parse_TypeAlias_Pointer_NoCustomUnmarshal(t *testing.T) {
	type noopstring string
	type configuration struct {
		FieldA *noopstring `key:"field_a"`
	}
	t.Setenv("FIELD_A", "lower")
	var c configuration
	err := yagcl.New[configuration]().
		Add(env.Source()).
		Parse(&c)
	if assert.NoError(t, err) {
		result := noopstring("lower")
		assert.Equal(t, &result, c.FieldA)
	}
}

func Test_Parse_TypeAlias_CustomStructType(t *testing.T) {
	type noopstring struct {
		Value string `key:"value"`
	}
	type noopstringwrapper noopstring
	type configuration struct {
		FieldA noopstringwrapper `key:"field_a"`
	}
	t.Setenv("FIELD_A_VALUE", "lower")
	var c configuration
	err := yagcl.New[configuration]().
		Add(env.Source()).
		Parse(&c)
	if assert.NoError(t, err) {
		assert.Equal(t, noopstringwrapper{Value: "lower"}, c.FieldA)
	}
}

func Test_Parse_TypeAlias_Pointer_CustomStructType(t *testing.T) {
	type noopstring struct {
		Value string `key:"value"`
	}
	type noopstringwrapper noopstring
	type configuration struct {
		FieldA *noopstringwrapper `key:"field_a"`
	}
	t.Setenv("FIELD_A_VALUE", "lower")
	var c configuration
	err := yagcl.New[configuration]().
		Add(env.Source()).
		Parse(&c)
	if assert.NoError(t, err) {
		result := noopstringwrapper{Value: "lower"}
		assert.Equal(t, &result, c.FieldA)
	}
}
