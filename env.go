package env

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Bios-Marcel/yagcl"
)

// DO NOT CREATE INSTANCES MANUALLY, THIS IS ONLY PUBLIC IN ORDER FOR GODOC
// TO RENDER AVAILABLE FUNCTIONS.
type EnvSource struct {
	prefix            string
	keyValueConverter func(string) string
	keyJoiner         func(string, string) string
}

// Source creates a source for environment variables of the current
// process.
func Source() *EnvSource {
	return &EnvSource{
		keyValueConverter: defaultKeyValueConverter,
		keyJoiner:         defaultKeyJoiner,
	}
}

// Prefix specified the prefixes expected in environment variable keys.
// For example "PREFIX_FIELD_NAME".
func (s *EnvSource) Prefix(prefix string) *EnvSource {
	s.prefix = prefix
	return s
}

// KeyValueConverter defines how the yagcl.DefaultKeyTagName value should be
// converted for this source. If you are setting this, you'll most likely
// also have to set EnvSource.KeyJoiner(string,string) string.
// Note that calling this isn't required, as there's a best practise default
// behaviour.
func (s *EnvSource) KeyValueConverter(keyValueConverter func(string) string) *EnvSource {
	s.keyValueConverter = keyValueConverter
	return s
}

func defaultKeyValueConverter(s string) string {
	// Since by default we expect keys to be of
	// format `word_word_...`, we just uppercase everything to meet
	// the defacto standard of environment variables.
	return strings.ToUpper(s)
}

// KeyJoiner defines the function that builds the environment variable keys.
// For example consider the following struct:
//     type Config struct {
//         Sub struct {
//             Field int `key:"field"`
//         } `key:"sub"`
//     }
// The joiner could for example produce sub_field, depending. In combination
// with KeyValueConverter, this could then become SUB_FIELD.
func (s *EnvSource) KeyJoiner(keyJoiner func(string, string) string) *EnvSource {
	s.keyJoiner = keyJoiner
	return s
}

func defaultKeyJoiner(s1, s2 string) string {
	if s1 == "" {
		return s2
	}

	// We don't check for s2 emptiness, as it is expected to always hold a
	// non-empty value.

	// By default we want to use whatever keys we have, and join them
	// with underscores, preventing duplicate underscores.
	return strings.Trim(s1, "_") + "_" + strings.Trim(s2, "_")
}

// KeyTag implements Source.Key.
func (s *EnvSource) KeyTag() string {
	return "env"
}

// Parse implements Source.Parse.
func (s *EnvSource) Parse(parsingCompanion yagcl.ParsingCompanion, configurationStruct any) (bool, error) {
	// FIXME For now we always say we've loaded something, this should change
	// at some point, using some kind of "was at least one variable loaded"
	// check.
	return true, s.parse(parsingCompanion, s.prefix, reflect.Indirect(reflect.ValueOf(configurationStruct)))
}

func (s *EnvSource) parse(parsingCompanion yagcl.ParsingCompanion, envPrefix string, structValue reflect.Value) error {
	structType := structValue.Type()
	for i := 0; i < structValue.NumField(); i++ {
		structField := structType.Field(i)
		if !parsingCompanion.IncludeField(structField) {
			continue
		}

		envKey, errExtractKey := s.extractEnvKey(parsingCompanion, structField)
		if errExtractKey != nil {
			return errExtractKey
		}
		joinedEnvKey := s.keyJoiner(envPrefix, envKey)
		envValue, set := os.LookupEnv(joinedEnvKey)
		if !set {
			// Since we handle pointers and structs differently, we must not do early exists / errors in these cases.
			if structField.Type.Kind() != reflect.Struct && structField.Type.Kind() != reflect.Pointer {
				continue
			}
		}

		value := structValue.Field(i)

		// For pointers, we require the non-pointer type underneath.
		underlyingType := extractNonPointerFieldType(value.Type())

		// In this section we check whether custom unmarshallers are present.
		// Types with a custom unmarshaller have to be checked first before
		// attempting to parse them using default behaviour, as the behaviour
		// might differ from std/json otherwise.
		var parsed reflect.Value

		// Technically this check isn't required, as we already filter out
		// unexported fields. However, I am unsure whether this behaviour is set
		// in stone, as it hasn't been documented properly.
		// https://stackoverflow.com/questions/50279840/when-is-go-reflect-caninterface-false
		if value.CanInterface() {
			// New pointer value, since non-pointers can't implement json.UnmarshalText.
			parsed = reflect.New(underlyingType)
			if u, ok := parsed.Interface().(encoding.TextUnmarshaler); ok {
				if err := u.UnmarshalText([]byte(envValue)); err != nil {
					return fmt.Errorf("value '%s' isn't parsable as an '%s' for field '%s'; %s: %w", envValue, underlyingType.String(), structField.Name, err, yagcl.ErrParseValue)
				}

				parsed = reflect.Indirect(parsed)
			} else {
				// Make sure we attempt a manual parse later.
				parsed = reflect.Zero(underlyingType)
			}
		}

		if parsed.IsZero() {
			var errParseValue error
			parsed, errParseValue = parseValue(structField.Name, structField.Type, envValue)
			if errParseValue != nil {
				if errParseValue != errEmbeddedStructDetected {
					return errParseValue
				}

				// If we have a non-pointer struct, it may contain default
				// values, which we want to preserve by not creating a new
				// instance of the struct.
				if value.Kind() != reflect.Pointer {
					if errParse := s.parse(parsingCompanion, joinedEnvKey, value); errParse != nil {
						return errParse
					}
					continue
				}

				// Non-nil Pointervalue, therefore we gotta use the existing
				// value in order to preserve potentially existing defaults.
				if !value.IsZero() {
					if errParse := s.parse(parsingCompanion, joinedEnvKey, value.Elem()); errParse != nil {
						return errParse
					}
					continue
				}

				underlyingType := extractNonPointerFieldType(structField.Type.Elem())
				newStruct := reflect.Indirect(reflect.New(underlyingType))
				if errParse := s.parse(parsingCompanion, joinedEnvKey, newStruct); errParse != nil {
					return errParse
				}
				parsed = newStruct
			}
			// Make sure that we have the correct alias type if necessary.
			parsed = parsed.Convert(underlyingType)
		}

		if parsed.IsZero() {
			continue
		}

		value.Set(convertValueToPointerIfRequired(value, parsed))
	}

	return nil
}

// errEmbeddedStructDetected is abused internally to detect that we need to
// recurse. This error should never reach the outer world.
var errEmbeddedStructDetected = errors.New("embedded struct detected")

// convertValueToPointerIfRequired creates reflect.Value wrapper of the
// required pointer types if necessary, otherwise, this is basically
// a no-op.
func convertValueToPointerIfRequired(targetValue reflect.Value, newValue reflect.Value) reflect.Value {
	if targetValue.Kind() != reflect.Pointer {
		return newValue
	}

	//Create as many values as we have pointers pointing to things.
	var pointers []reflect.Value
	lastPointer := reflect.New(targetValue.Type().Elem())
	pointers = append(pointers, lastPointer)
	for lastPointer.Elem().Kind() == reflect.Pointer {
		lastPointer = reflect.New(lastPointer.Elem().Type().Elem())
		pointers = append(pointers, lastPointer)
	}

	pointers[len(pointers)-1].Elem().Set(newValue)
	for i := len(pointers) - 2; i >= 0; i-- {
		pointers[i].Elem().Set(pointers[i+1])
	}
	return pointers[0]
}

func (s *EnvSource) extractEnvKey(parsingCompanion yagcl.ParsingCompanion, structField reflect.StructField) (string, error) {
	// Custom tag
	key := structField.Tag.Get(s.KeyTag())
	if key != "" {
		return key, nil
	}

	// Fallback tag
	if key := parsingCompanion.ExtractFieldKey(structField); key != "" {
		return s.keyValueConverter(key), nil
	}

	// No tag found
	return "", fmt.Errorf("neither tag '%s' nor the standard tag '%s' have been set for field '%s': %w", s.KeyTag(), yagcl.DefaultKeyTagName, structField.Name, yagcl.ErrExportedFieldMissingKey)
}

func parseValue(fieldName string, fieldType reflect.Type, envValue string) (reflect.Value, error) {
	switch fieldType.Kind() {
	case reflect.String:
		{
			return reflect.ValueOf(envValue), nil
		}
	case reflect.Int64:
		{
			// Since there are no constants for alias / struct types, we have
			// to an additional check with custom parsing, since durations
			// also contain a duration unit, such as "s" for seconds.
			if fieldType.AssignableTo(reflect.TypeOf(time.Duration(0))) {
				value, errParse := time.ParseDuration(envValue)
				if errParse != nil {
					return reflect.Value{}, fmt.Errorf("value '%s' isn't parsable as an 'time.Duration' for field '%s': %w", envValue, fieldName, yagcl.ErrParseValue)
				}
				return reflect.ValueOf(value).Convert(fieldType), nil
			}
		}
		// Since we seem to just have a normal int64 (or other alias type), we
		// want to proceed treating it as a normal int, which is why we
		// fallthrough.
		fallthrough
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		{
			value, errParse := strconv.ParseInt(envValue, 10, int(fieldType.Size())*8)
			if errParse != nil {
				return reflect.Value{}, fmt.Errorf("value '%s' isn't parsable as an '%s' for field '%s': %w", envValue, fieldType.String(), fieldName, yagcl.ErrParseValue)
			}
			return reflect.ValueOf(value).Convert(fieldType), nil
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		{
			value, errParse := strconv.ParseUint(envValue, 10, int(fieldType.Size())*8)
			if errParse != nil {
				return reflect.Value{}, fmt.Errorf("value '%s' isn't parsable as an '%s' for field '%s': %w", envValue, fieldType.String(), fieldName, yagcl.ErrParseValue)
			}
			return reflect.ValueOf(value).Convert(fieldType), nil
		}
	case reflect.Float32, reflect.Float64:
		{
			// We use the stdlib json encoder here, since there seems to be
			// special behaviour.
			var value float64
			if errParse := json.Unmarshal([]byte(envValue), &value); errParse != nil {
				return reflect.Value{}, fmt.Errorf("value '%s' isn't parsable as an '%s' for field '%s': %w", envValue, fieldType.String(), fieldName, yagcl.ErrParseValue)
			}
			return reflect.ValueOf(value).Convert(fieldType), nil
		}
	case reflect.Bool:
		{
			boolValue := strings.EqualFold(envValue, "true")
			// FIXME Allow enabling lax-behaviour?
			// Instead of assuming everything != true equals false, we assume
			// that the value is unintentionally wrong and return an error.
			if !boolValue && !strings.EqualFold(envValue, "false") {
				return reflect.Value{}, fmt.Errorf("value '%s' isn't parsable as a '%s' for field '%s': %w", envValue, fieldType.String(), fieldName, yagcl.ErrParseValue)
			}
			return reflect.ValueOf(boolValue), nil
		}
	case reflect.Struct:
		{
			return reflect.Value{}, errEmbeddedStructDetected
		}
	case reflect.Pointer:
		{
			nonPointerFieldType := extractNonPointerFieldType(fieldType)
			if nonPointerFieldType.Kind() == reflect.Struct {
				return reflect.Value{}, errEmbeddedStructDetected
			}
			return parseValue(fieldName, extractNonPointerFieldType(fieldType), envValue)
		}
	case reflect.Map:
		{
			//FIXME Check if map-type is supported.

			rawEntries := splitString(envValue, ',')
			targetMap := reflect.MakeMapWithSize(fieldType, len(rawEntries))
			for index, entry := range rawEntries {
				keyValue := splitString(entry, '=')
				if len(keyValue) == 1 {
					return reflect.Value{}, fmt.Errorf("field '%s' contains possibly misformatted value at index %d ('%s'); no unescaped '=' was found to separate key from value: %w", fieldName, index, entry, yagcl.ErrParseValue)
				}
				if len(keyValue) > 2 {
					return reflect.Value{}, fmt.Errorf("field '%s' contains possibly misformatted value at index %d ('%s'); more than one unescaped '=' has been found: %w", fieldName, index, entry, yagcl.ErrParseValue)
				}
				parsedKey, errParseKey := parseValue(fieldName, fieldType.Key(), keyValue[0])
				if errParseKey != nil {
					return reflect.Value{}, fmt.Errorf("field '%s' contained unparsable key '%s': %w", fieldName, keyValue[0], yagcl.ErrParseValue)
				}
				parsedValue, errParseValue := parseValue(fieldName, fieldType.Elem(), keyValue[1])
				if errParseValue != nil {
					return reflect.Value{}, fmt.Errorf("field '%s' contained unparsable value '%s': %w", fieldName, keyValue[1], yagcl.ErrParseValue)
				}
				targetMap.SetMapIndex(parsedKey, parsedValue)
			}

			return targetMap, nil
		}
	case reflect.Slice:
		{
			if !isSliceTypeSupported(fieldType.Elem()) {
				return reflect.Value{}, fmt.Errorf("field '%s' has unsupported type '%s': %w", fieldName, fieldType.String(), yagcl.ErrUnsupportedFieldType)
			}

			arrayRawValues := splitString(envValue, ',')
			targetArray := reflect.MakeSlice(fieldType, len(arrayRawValues), len(arrayRawValues))
			if err := parseIntoArray(fieldName, fieldType, targetArray, arrayRawValues); err != nil {
				// Wrapping ErrParseValue isn't necessary, as this internally
				// calls parseValue, which should already take care of that.
				return reflect.Value{}, err
			}
			return targetArray, nil
		}

	case reflect.Array:
		// Arrays are of fixed size (for example the definition int[3]),
		// therefore we treat them separately from slices, as not passing
		// correct amount of values indicates a configuration error.
		{
			if !isSliceTypeSupported(fieldType.Elem()) {
				return reflect.Value{}, fmt.Errorf("field '%s' has unsupported type '%s': %w", fieldName, fieldType.String(), yagcl.ErrUnsupportedFieldType)
			}

			targetArray := reflect.Indirect(reflect.New(fieldType))
			arrayRawValues := splitString(envValue, ',')
			if targetArray.Len() != len(arrayRawValues) {
				return reflect.Value{}, fmt.Errorf("value specified for field '%s' is an array of incorrect length, expected length %d, but got %d: %w", fieldName, targetArray.Len(), len(arrayRawValues), yagcl.ErrParseValue)
			}
			if err := parseIntoArray(fieldName, fieldType, targetArray, arrayRawValues); err != nil {
				// Wrapping ErrParseValue isn't necessary, as this internally
				// calls parseValue, which should already take care of that.
				return reflect.Value{}, err
			}
			return targetArray, nil
		}
	case reflect.Complex64, reflect.Complex128:
		{
			// Complex isn't supported, as for example it also isn't supported
			// by the stdlib json encoder / decoder.
			return reflect.Value{}, fmt.Errorf("field '%s' has unsupported type '%s' (Support not planned): %w", fieldName, fieldType.String(), yagcl.ErrUnsupportedFieldType)
		}
	default:
		{
			return reflect.Value{}, fmt.Errorf("field '%s' has unsupported type '%s': %w", fieldName, fieldType.String(), yagcl.ErrUnsupportedFieldType)
		}
	}
}

// splitString splits the given "literal" at each "splitChar" found.
// Additionally it allows you to escape the "splitChar" by using "\", which
// on the other hand can be escaped the same way.
func splitString(literal string, splitChar rune) []string {
	var values []string
	var buffer []rune
	var escapeNext bool
	maxIndex := len(literal) - 1
	for index, character := range literal {
		if index == maxIndex {
			if character != splitChar || escapeNext {
				buffer = append(buffer, character)
			}
			values = append(values, string(buffer))
			break
		}

		escape := escapeNext
		escapeNext = false

		if character == splitChar && !escape {
			values = append(values, string(buffer))
			buffer = buffer[:0]
		} else if character == '\\' && !escape {
			escapeNext = true
		} else {
			buffer = append(buffer, character)
		}
	}

	return values
}

func isSliceTypeSupported(sliceType reflect.Type) bool {
	switch extractNonPointerFieldType(sliceType).Kind() {
	case
		reflect.UnsafePointer,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Struct,
		//FIXME These two still need to be implemented
		reflect.Array,
		reflect.Slice:

		return false
	}

	return true
}

func parseIntoArray(fieldName string, fieldType reflect.Type, targetArray reflect.Value, arrayRawValues []string) error {
	for index, rawValue := range arrayRawValues {
		parsedValue, err := parseValue(fieldName, fieldType.Elem(), rawValue)
		if err != nil {
			return err
		}
		targetIndex := targetArray.Index(index)
		targetIndex.Set(convertValueToPointerIfRequired(targetIndex, parsedValue))
	}
	return nil
}

func extractNonPointerFieldType(fieldType reflect.Type) reflect.Type {
	if fieldType.Kind() != reflect.Pointer {
		return fieldType
	}

	return extractNonPointerFieldType(fieldType.Elem())
}
