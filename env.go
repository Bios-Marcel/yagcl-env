package env

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Bios-Marcel/yagcl"
	"github.com/subosito/gotenv"
)

// ErrNoDataSourceSpecified is thrown if none Bytes, String, Path or Reader
// of the EnvSourceSetupStepOne interface have been called.
var ErrNoDataSourceSpecified = errors.New("no data source specified; call Bytes(), String(), Reader() or Path()")

// ErrNoDataSourceSpecified is thrown if more than one of Bytes, String, Path
// or Reader of the EnvSourceSetupStepOne interface have been called.
var ErrMultipleDataSourcesSpecified = errors.New("more than one data source specified; only call one of Bytes(), String(), Reader() or Path()")

type envSourceImpl struct {
	path    string
	bytes   []byte
	reader  io.Reader
	readEnv bool

	must              bool
	loadIntoEnv       bool
	prefix            string
	keyValueConverter func(string) string
	keyJoiner         func(string, string) string
}

type EnvSourceSetupStepOne[T yagcl.Source] interface {
	// Bytes defines a byte array to read from directly.
	Bytes([]byte) EnvSourceSetupStepTwoEnvFile[T]
	// Bytes defines a string to read from directly.
	String(string) EnvSourceSetupStepTwoEnvFile[T]
	// Path defines a filepath that is accessed when YAGCL.Parse is called.
	Path(string) EnvSourceSetupStepTwoEnvFile[T]
	// Reader defines a reader that is accessed when YAGCL.Parse is called. IF
	// available, io.Closer.Close() is called.
	Reader(io.Reader) EnvSourceSetupStepTwoEnvFile[T]
	// Env instructs the source to read directly from the environment
	// variables.
	Env() EnvSourceSetupStepTwoEnv[T]
}

type EnvSourceSetupStepTwoEnvFile[T yagcl.Source] interface {
	EnvSourceOptionalSetup[T]

	// LoadIntoEnv activates loading the unparsed data into the environment
	// variables of the process.
	LoadIntoEnv() T
	// Must declares this source as mandatory, erroring in case no data can
	// be loaded. In case of loading directly from the environment, this
	// will always succeed though, as the environment is always there, even
	// if we can't load any values, due to the fact they aren't available.
	Must() T
}

type EnvSourceSetupStepTwoEnv[T yagcl.Source] interface {
	EnvSourceOptionalSetup[T]
}

type EnvSourceOptionalSetup[T yagcl.Source] interface {
	yagcl.Source

	// Prefix specified the prefixes expected in environment variable keys.
	// For example "PREFIX_FIELD_NAME".
	Prefix(string) T
	// KeyValueConverter defines how the yagcl.DefaultKeyTagName value should be
	// converted for this source. If you are setting this, you'll most likely
	// also have to set EnvSource.KeyJoiner(string,string) string.
	// Note that calling this isn't required, as there's a best practise default
	// behaviour.
	KeyValueConverter(func(string) string) T
	// KeyJoiner defines the function that builds the environment variable keys.
	// For example consider the following struct:
	//
	//	type Config struct {
	//	    Sub struct {
	//	        Field int `key:"field"`
	//	    } `key:"sub"`
	//	}
	//
	// The joiner could for example produce sub_field, depending. In combination
	// with KeyValueConverter, this could then become SUB_FIELD.
	KeyJoiner(func(string, string) string) T
}

// Source creates a source for environment variables of the current
// process.
func Source() EnvSourceSetupStepOne[*envSourceImpl] {
	return &envSourceImpl{
		keyValueConverter: defaultKeyValueConverter,
		keyJoiner:         defaultKeyJoiner,
	}
}

// Bytes implements EnvSourceSetupStepOne.Bytes.
func (s *envSourceImpl) Bytes(bytes []byte) EnvSourceSetupStepTwoEnvFile[*envSourceImpl] {
	s.bytes = bytes
	return s
}

// String implements EnvSourceSetupStepOne.String.
func (s *envSourceImpl) String(str string) EnvSourceSetupStepTwoEnvFile[*envSourceImpl] {
	s.bytes = []byte(str)
	return s
}

// Path implements EnvSourceSetupStepOne.Path.
func (s *envSourceImpl) Path(path string) EnvSourceSetupStepTwoEnvFile[*envSourceImpl] {
	s.path = path
	return s
}

// Reader implements EnvSourceSetupStepOne.Reader.
func (s *envSourceImpl) Reader(reader io.Reader) EnvSourceSetupStepTwoEnvFile[*envSourceImpl] {
	s.reader = reader
	return s
}

// Reader implements EnvSourceSetupStepOne.Env.
func (s *envSourceImpl) Env() EnvSourceSetupStepTwoEnv[*envSourceImpl] {
	s.readEnv = true
	return s
}

// LoadIntoEnv implements EnvSourceOptionalSetup.LoadIntoEnv.
func (s *envSourceImpl) LoadIntoEnv() *envSourceImpl {
	// note that this is nonsensical if Env() was called, however, technically
	// it doesn't matter, as the result will be the same.
	s.loadIntoEnv = true
	return s
}

// Must implements EnvSourceOptionalSetup.Must.
func (s *envSourceImpl) Must() *envSourceImpl {
	s.must = true
	return s
}

// Prefix implements EnvSourceOptionalSetup.Prefix.
func (s *envSourceImpl) Prefix(prefix string) *envSourceImpl {
	s.prefix = prefix
	return s
}

// KeyValueConverter implements EnvSourceOptionalSetup.KeyValueConverter.
func (s *envSourceImpl) KeyValueConverter(keyValueConverter func(string) string) *envSourceImpl {
	s.keyValueConverter = keyValueConverter
	return s
}

func defaultKeyValueConverter(s string) string {
	// Since by default we expect keys to be of
	// format `word_word_...`, we just uppercase everything to meet
	// the defacto standard of environment variables.
	return strings.ToUpper(s)
}

// KeyJoiner implements EnvSourceOptionalSetup.KeyJoiner.
func (s *envSourceImpl) KeyJoiner(keyJoiner func(string, string) string) *envSourceImpl {
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
func (s *envSourceImpl) KeyTag() string {
	return "env"
}

type envLookup func(key string) (string, bool)

func (s *envSourceImpl) load() (lookup envLookup, err error) {
	var env gotenv.Env

	// We attempt to check if the source can't be found. While we only do
	// direct file access in case a path is passed, a reader might also
	// attempt reading from a file source, therefore we try to check that
	// error as well. If the source has been set successfuly, we convert it
	// into a lookup function.
	defer func() {
		if err != nil {
			return
		}

		lookup = func(key string) (string, bool) {
			val, set := env[key]
			return val, set
		}

		if s.loadIntoEnv {
			for key, val := range env {
				os.Setenv(key, val)
			}
		}
	}()

	// Do bytes first, since it saves us the error handling code.
	if len(s.bytes) > 0 {
		env, err = gotenv.StrictParse(bytes.NewReader(s.bytes))
		return
	}

	if s.path != "" {
		env, err = gotenv.Read(s.path)
		return
	}

	if s.reader != nil {
		if closer, ok := s.reader.(io.Closer); ok {
			defer closer.Close()
		}

		env, err = gotenv.StrictParse(s.reader)
		return
	}

	// This should be dead code and therefore isn't covered by a test either.
	err = errors.New("verification process must have failed, please report this to the maintainer")
	return
}

// verify checks whether the source has been configured correctly. We attempt
// avoiding any condiguration errors by API design.
func (s *envSourceImpl) verify() error {
	var (
		dataSourcesCount uint
	)
	if s.readEnv {
		dataSourcesCount++
	}
	if s.path != "" {
		dataSourcesCount++
	}
	if len(s.bytes) > 0 {
		dataSourcesCount++
	}
	if s.reader != nil {
		dataSourcesCount++
	}

	if dataSourcesCount == 0 {
		return ErrNoDataSourceSpecified
	}
	if dataSourcesCount > 1 {
		return ErrMultipleDataSourcesSpecified
	}

	if s.path != "" {
		info, err := os.Stat(s.path)
		if err != nil {
			return err
		}
		if info != nil && info.IsDir() {
			return errors.New("path, must be a directory: %s")
		}
	}

	return nil
}

// Parse implements Source.Parse.
func (s *envSourceImpl) Parse(parsingCompanion yagcl.ParsingCompanion, configurationStruct any) (dataLoaded bool, err error) {
	defer func() {
		if os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist) {
			err = yagcl.ErrSourceNotFound
		}
		if !s.must && errors.Is(err, yagcl.ErrSourceNotFound) {
			err = nil
		}
		if err != nil {
			dataLoaded = false
		}
	}()

	if err = s.verify(); err != nil {
		return
	}

	var lookup envLookup
	if s.readEnv {
		lookup = os.LookupEnv
	} else {
		lookup, err = s.load()
		if err != nil {
			return
		}
	}

	// FIXME For now we always say we've loaded something, this should change
	// at some point, using some kind of "was at least one variable loaded"
	// check.
	dataLoaded = true
	err = s.parse(parsingCompanion, lookup, s.prefix, reflect.Indirect(reflect.ValueOf(configurationStruct)))
	return
}

func (s *envSourceImpl) parse(parsingCompanion yagcl.ParsingCompanion, lookup envLookup, envPrefix string, structValue reflect.Value) error {
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
		envValue, set := lookup(joinedEnvKey)
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

		// Technically this check isn't required, as we already filter out
		// unexported fields. However, I am unsure whether this behaviour is set
		// in stone, as it hasn't been documented properly.
		// https://stackoverflow.com/questions/50279840/when-is-go-reflect-caninterface-false
		if value.CanInterface() {
			// Here we try to find the deepest pointer type. As something
			// like ***type doesn't allow calling `TextUnmarshal` and a value
			// type doesn't allow it either. If we get a value type instead of
			// a pointer, we manually wrap it.
			var target reflect.Value
			if deepestPotentialPointer := extractDeepestPotentialPointer(value); deepestPotentialPointer.Kind() == reflect.Pointer {
				if deepestPotentialPointer.IsNil() {
					target = reflect.New(underlyingType)
				} else {
					target = deepestPotentialPointer
				}
			} else {
				target = reflect.New(underlyingType)
				// Preserve potential defaults set in non-pointer value.
				target.Elem().Set(value)
			}

			if u, ok := target.Interface().(encoding.TextUnmarshaler); ok {
				if err := u.UnmarshalText([]byte(envValue)); err != nil {
					return fmt.Errorf("value '%s' isn't parsable as an '%s' for field '%s'; %s: %w", envValue, underlyingType.String(), structField.Name, err, yagcl.ErrParseValue)
				}

				value.Set(convertValueToPointerIfRequired(value, reflect.Indirect(target)))
				// We are done with this field and don't need to fall back to
				// the default parsing logic.
				continue
			}
		}

		parsed, errParseValue := parseValue(structField.Name, structField.Type, envValue)
		if errParseValue != nil {
			if errParseValue != errEmbeddedStructDetected {
				return errParseValue
			}

			// If we have a non-pointer struct, it may contain default
			// values, which we want to preserve by not creating a new
			// instance of the struct.
			if deepestPotentialPointer := extractDeepestPotentialPointer(value); deepestPotentialPointer.Kind() != reflect.Pointer {
				if errParse := s.parse(parsingCompanion, lookup, joinedEnvKey, deepestPotentialPointer); errParse != nil {
					return errParse
				}
				continue
			} else
			// Non-nil Pointervalue, therefore we gotta use the existing
			// value in order to preserve potentially existing defaults.
			if !deepestPotentialPointer.IsZero() {
				if errParse := s.parse(parsingCompanion, lookup, joinedEnvKey, deepestPotentialPointer.Elem()); errParse != nil {
					return errParse
				}
				continue
			}

			underlyingType := extractNonPointerFieldType(structField.Type.Elem())
			newStruct := reflect.Indirect(reflect.New(underlyingType))
			if errParse := s.parse(parsingCompanion, lookup, joinedEnvKey, newStruct); errParse != nil {
				return errParse
			}
			parsed = newStruct
		}

		if parsed.IsZero() {
			continue
		}

		// Make sure that we have the correct alias type if necessary.
		parsed = parsed.Convert(underlyingType)
		parsed = convertValueToPointerIfRequired(value, parsed)
		value.Set(parsed)
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

	// Create as many values as we have pointers pointing to things.
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

func (s *envSourceImpl) extractEnvKey(parsingCompanion yagcl.ParsingCompanion, structField reflect.StructField) (string, error) {
	// Custom tag
	if key := structField.Tag.Get(s.KeyTag()); key != "" {
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
			// FIXME Check if map-type is supported.

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
		// FIXME These two still need to be implemented
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

func extractDeepestPotentialPointer(value reflect.Value) reflect.Value {
	if value.Kind() == reflect.Pointer {
		if value.Elem().Kind() != reflect.Pointer {
			return value
		}

		return extractDeepestPotentialPointer(value.Elem())
	}

	return value
}

func extractNonPointerFieldType(fieldType reflect.Type) reflect.Type {
	if fieldType.Kind() != reflect.Pointer {
		return fieldType
	}

	return extractNonPointerFieldType(fieldType.Elem())
}
