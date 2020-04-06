package surveyutils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/jenkins-x/jx/pkg/secreturl"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/util/secrets"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/util"

	"github.com/iancoleman/orderedmap"

	survey "gopkg.in/AlecAivazis/survey.v1"

	"gopkg.in/AlecAivazis/survey.v1/terminal"
)

var (
	generatedPasswordValueRegex = regexp.MustCompile(`^<generated:?(.*)>$`)
)

// Type represents a JSON Schema object type current to https://www.ietf.org/archive/id/draft-handrews-json-schema-validation-01.txt
type Type struct {
	Version          string      `json:"$schema,omitempty"`
	Ref              string      `json:"$ref,omitempty"`
	MultipleOf       *float64    `json:"multipleOf,omitempty"`
	Maximum          *float64    `json:"maximum,omitempty"`
	ExclusiveMaximum *float64    `json:"exclusiveMaximum,omitempty"`
	Minimum          *float64    `json:"minimum,omitempty"`
	ExclusiveMinimum *float64    `json:"exclusiveMinimum,omitempty"`
	MaxLength        *int        `json:"maxLength,omitempty"`
	MinLength        *int        `json:"minLength,omitempty"`
	Pattern          *string     `json:"pattern,omitempty"`
	AdditionalItems  *Type       `json:"additionalItems,omitempty"`
	Items            Items       `json:"items,omitempty"`
	MaxItems         *int        `json:"maxItems,omitempty"`
	MinItems         *int        `json:"minItems,omitempty"`
	UniqueItems      bool        `json:"uniqueItems,omitempty"`
	MaxProperties    *int        `json:"maxProperties,omitempty"`
	MinProperties    *int        `json:"minProperties,omitempty"`
	Required         []string    `json:"required,omitempty"`
	Properties       *Properties `json:"properties,omitempty"`
	// TODO Implement support & tests for PatternProperties
	PatternProperties map[string]*Type `json:"patternProperties,omitempty"`
	// TODO Implement support & tests for AdditionalProperties
	AdditionalProperties *Type `json:"additionalProperties,omitempty"`
	// TODO Implement support & tests for Dependencies
	Dependencies map[string]Dependency `json:"dependencies,omitempty"`
	// TODO Implement support & tests for PropertyNames
	PropertyNames *Type         `json:"propertyNames,omitempty"`
	Enum          []interface{} `json:"enum,omitempty"`
	Type          string        `json:"type,omitempty"`
	If            *Type         `json:"if,omitempty"`
	Then          *Type         `json:"then,omitempty"`
	Else          *Type         `json:"else,omitempty"`
	// TODO Implement support & tests for All
	AllOf []*Type `json:"allOf,omitempty"`
	AnyOf []*Type `json:"anyOf,omitempty"`
	// TODO Implement support & tests for OneOf
	OneOf []*Type `json:"oneOf,omitempty"`
	// TODO Implement support & tests for Not
	Not *Type `json:"not,omitempty"`
	// TODO Implement support & tests for Definitions
	Definitions Definitions `json:"definitions,omitempty"`
	// TODO Implement support & tests for Contains
	Contains         *Type        `json:"contains,omitempty"`
	Const            *interface{} `json:"const,omitempty"`
	Title            string       `json:"title,omitempty"`
	Description      string       `json:"description,omitempty"`
	Default          interface{}  `json:"default,omitempty"`
	Format           *string      `json:"format,omitempty"`
	ContentMediaType *string      `json:"contentMediaType,omitempty"`
	ContentEncoding  *string      `json:"contentEncoding,omitempty"`
}

// Definitions hold schema definitions.
type Definitions map[string]*Type

// Dependency is either a Type or an array of strings, and so requires special unmarshaling from JSON
type Dependency struct {
	Type  *Type    `json:-`
	Array []string `json:-`
}

// UnmarshalJSON performs unmarshals Dependency from JSON, required as the json field can be one of two types
func (d *Dependency) UnmarshalJSON(b []byte) error {
	if b[0] == '[' {
		return json.Unmarshal(b, &d.Array)
	}
	return json.Unmarshal(b, d.Type)
}

// Items is a either a Type or a array of types, and so requires special unmarshaling from JSON
type Items struct {
	Types []*Type `json:-`
	Type  *Type   `json:-`
}

// Properties is a set of ordered key-value pairs, as it is ordered it requires special marshaling to/from JSON
type Properties struct {
	*orderedmap.OrderedMap
}

// UnmarshalJSON performs custom Unmarshaling for Properties allowing us to preserve order,
// which is not a standard JSON feature
func (p *Properties) UnmarshalJSON(b []byte) error {
	m := orderedmap.New()
	err := json.Unmarshal(b, &m)
	if err != nil {
		return err
	}
	if p.OrderedMap == nil {
		p.OrderedMap = orderedmap.New()
	}
	for _, k := range m.Keys() {
		v, _ := m.Get(k)
		t := Type{}
		om, ok := v.(orderedmap.OrderedMap)
		if !ok {
			return fmt.Errorf("Unable to cast nested data structure to OrderedMap")
		}
		values := make(map[string]interface{}, 0)
		for _, k1 := range om.Keys() {
			v1, _ := om.Get(k1)
			values[k1] = v1
		}
		err := util.ToStructFromMapStringInterface(values, &t)
		if err != nil {
			return err
		}
		p.Set(k, &t)
	}
	return nil
}

// UnmarshalJSON performs unmarshals Items from JSON, required as the json field can be one of two types
func (t *Items) UnmarshalJSON(b []byte) error {
	if b[0] == '[' {
		return json.Unmarshal(b, &t.Types)
	}
	err := json.Unmarshal(b, &t.Type)
	if err != nil {
		return err
	}
	return nil
}

// JSONSchemaOptions are options for generating values from a schema
type JSONSchemaOptions struct {
	VaultClient         secreturl.Client
	VaultBasePath       string
	VaultScheme         string
	AskExisting         bool
	AutoAcceptDefaults  bool
	NoAsk               bool
	IgnoreMissingValues bool
	In                  terminal.FileReader
	Out                 terminal.FileWriter
	OutErr              io.Writer
}

// GenerateValues examines the schema in schemaBytes, asks a series of questions using in, out and outErr,
// applying validators, returning a generated json file.
// If there are existingValues then those questions will be ignored and the existing value used unless askExisting is
// true. If autoAcceptDefaults is true, then default values will be used automatically.
// If ignoreMissingValues is false then any values which don't have an existing value (
// or a default value if autoAcceptDefaults is true) will cause an error
func (o *JSONSchemaOptions) GenerateValues(schemaBytes []byte, existingValues map[string]interface{}) ([]byte, error) {
	t := Type{}
	err := json.Unmarshal(schemaBytes, &t)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshaling schema %s", schemaBytes)
	}
	output := orderedmap.New()
	err = o.recurse("", make([]string, 0), make([]string, 0), &t, nil, output, make([]survey.Validator, 0), existingValues)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// move the output up a level
	if root, ok := output.Get(""); ok {
		bytes, err := json.Marshal(root)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return bytes, nil
	}
	return make([]byte, 0), fmt.Errorf("unable to find root element in %v", output)

}

func (o *JSONSchemaOptions) handleConditionals(prefixes []string, requiredFields []string, property string, t *Type, parentType *Type, output *orderedmap.OrderedMap, existingValues map[string]interface{}) error {
	if parentType != nil {
		err := o.handleIf(prefixes, requiredFields, property, t, parentType, output, existingValues)
		if err != nil {
			return errors.WithStack(err)
		}
		err = o.handleAllOf(prefixes, requiredFields, property, t, parentType, output, existingValues)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (o *JSONSchemaOptions) handleAllOf(prefixes []string, requiredFields []string, property string, t *Type, parentType *Type, output *orderedmap.OrderedMap, existingValues map[string]interface{}) error {
	if parentType.AllOf != nil && len(parentType.AllOf) > 0 {
		for _, allType := range parentType.AllOf {
			err := o.handleIf(prefixes, requiredFields, property, t, allType, output, existingValues)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (o *JSONSchemaOptions) handleIf(prefixes []string, requiredFields []string, propertyName string, t *Type, parentType *Type, output *orderedmap.OrderedMap, existingValues map[string]interface{}) error {
	if parentType.If != nil {
		if len(parentType.If.Properties.Keys()) > 1 {
			return fmt.Errorf("Please specify a single property condition when using If in your schema")
		}
		detypedCondition, conditionFound := parentType.If.Properties.Get(propertyName)
		selectedValue, selectedValueFound := output.Get(propertyName)
		if conditionFound && selectedValueFound {
			desiredState := true
			if detypedCondition != nil {
				condition := detypedCondition.(*Type)
				if condition.Const != nil {

					switch t.Type {
					case "boolean":
						tConst, err := util.AsBool(*condition.Const)
						if err != nil {
							return err
						}
						if tConst != selectedValue {
							desiredState = false
						}
					default:
						stringConst, err := util.AsString(*condition.Const)
						if err != nil {
							return errors.Wrapf(err, "converting %s to string", condition.Type)
						}
						typedConst, err := convertAnswer(stringConst, t.Type)
						if typedConst != selectedValue {
							desiredState = false
						}
					}
				}
			}
			result := orderedmap.New()
			if desiredState {
				if parentType.Then != nil {
					parentType.Then.Type = "object"
					err := o.processThenElse(result, output, requiredFields, parentType.Then, parentType, existingValues)
					if err != nil {
						return err
					}
				}
			} else {
				if parentType.Else != nil {
					parentType.Else.Type = "object"
					err := o.processThenElse(result, output, requiredFields, parentType.Else, parentType, existingValues)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (o *JSONSchemaOptions) processThenElse(result *orderedmap.OrderedMap, output *orderedmap.OrderedMap, requiredFields []string, conditionalType *Type, parentType *Type, existingValues map[string]interface{}) error {
	err := o.recurse("", make([]string, 0), requiredFields, conditionalType, parentType, result, make([]survey.Validator, 0), existingValues)
	if err != nil {
		return err
	}
	resultSet, found := result.Get("")
	if found {
		resultMap := resultSet.(*orderedmap.OrderedMap)
		for _, key := range resultMap.Keys() {
			value, foundValue := resultMap.Get(key)
			if foundValue {
				output.Set(key, value)
			}
		}
	}
	return nil
}

func (o *JSONSchemaOptions) recurse(name string, prefixes []string, requiredFields []string, t *Type, parentType *Type, output *orderedmap.OrderedMap,
	additionalValidators []survey.Validator, existingValues map[string]interface{}) error {
	required := util.Contains(requiredFields, name)
	if name != "" {
		prefixes = append(prefixes, name)
	}
	if t.ContentEncoding != nil {
		return fmt.Errorf("contentEncoding is not supported for %s", name)
		// TODO support contentEncoding
	}
	if t.ContentMediaType != nil {
		return fmt.Errorf("contentMediaType is not supported for %s", name)
		// TODO support contentMediaType
	}

	switch t.Type {
	case "null":
		output.Set(name, nil)
	case "boolean":
		validators := []survey.Validator{
			EnumValidator(t.Enum),
			RequiredValidator(required),
			BoolValidator(),
		}
		validators = append(validators, additionalValidators...)
		err := o.handleBasicProperty(name, prefixes, validators, t, output, existingValues, required)
		if err != nil {
			return err
		}
	case "object":
		if t.AdditionalProperties != nil {
			return fmt.Errorf("additionalProperties is not supported for %s", name)
		}
		if len(t.PatternProperties) > 0 {
			return fmt.Errorf("patternProperties is not supported for %s", name)
		}
		if len(t.Dependencies) > 0 {
			return fmt.Errorf("dependencies is not supported for %s", name)
		}
		if t.PropertyNames != nil {
			return fmt.Errorf("propertyNames is not supported for %s", name)
		}
		if t.Const != nil {
			return fmt.Errorf("const is not supported for %s", name)
			// TODO support const
		}
		if t.Properties != nil {
			for valid := false; !valid; {
				result := orderedmap.New()
				duringValidators := make([]survey.Validator, 0)
				postValidators := []survey.Validator{
					// These validators are run after the processing of the properties
					MinPropertiesValidator(t.MinProperties, result, name),
					EnumValidator(t.Enum),
					MaxPropertiesValidator(t.MaxProperties, result, name),
				}
				for _, n := range t.Properties.Keys() {
					v, _ := t.Properties.Get(n)
					property := v.(*Type)
					var nestedExistingValues map[string]interface{}
					if name == "" {
						// This is the root element
						nestedExistingValues = existingValues
					} else if v, ok := existingValues[name]; ok {
						var err error
						nestedExistingValues, err = util.AsMapOfStringsIntefaces(v)
						if err != nil {
							return errors.Wrapf(err, "converting key %s from %v to map[string]interface{}", name, existingValues)
						}
					}
					err := o.recurse(n, prefixes, t.Required, property, t, result, duringValidators, nestedExistingValues)
					if err != nil {
						return err
					}
				}
				valid = true
				for _, v := range postValidators {
					err := v(result)
					if err != nil {
						str := fmt.Sprintf("Sorry, your reply was invalid: %s", err.Error())
						_, err1 := o.Out.Write([]byte(str))
						if err1 != nil {
							return err1
						}
						valid = false
					}
				}
				if valid {
					output.Set(name, result)
				}
			}
		}
	case "array":
		if t.Const != nil {
			return fmt.Errorf("const is not supported for %s", name)
			// TODO support const
		}
		if t.Contains != nil {
			return fmt.Errorf("contains is not supported for %s", name)
			// TODO support contains
		}
		if t.AdditionalItems != nil {
			return fmt.Errorf("additionalItems is not supported for %s", name)
			// TODO support additonalItems
		}
		err := o.handleArrayProperty(name, t, output, existingValues)
		if err != nil {
			return err
		}
	case "number":
		validators := additionalValidators
		validators = append(validators, FloatValidator())
		err := o.handleBasicProperty(name, prefixes, numberValidator(required, validators, t), t, output, existingValues, required)
		if err != nil {
			return err
		}
	case "string":
		validators := []survey.Validator{
			EnumValidator(t.Enum),
			MinLengthValidator(t.MinLength),
			MaxLengthValidator(t.MaxLength),
			RequiredValidator(required),
			PatternValidator(t.Pattern),
		}
		// Defined Format validation
		if t.Format != nil {
			format := util.DereferenceString(t.Format)
			switch format {
			case "date-time":
				validators = append(validators, DateTimeValidator())
			case "date":
				validators = append(validators, DateValidator())
			case "time":
				validators = append(validators, TimeValidator())
			case "email", "idn-email":
				validators = append(validators, EmailValidator())
			case "hostname", "idn-hostname":
				validators = append(validators, HostnameValidator())
			case "ipv4":
				validators = append(validators, Ipv4Validator())
			case "ipv6":
				validators = append(validators, Ipv6Validator())
			case "uri":
				validators = append(validators, URIValidator())
			case "uri-reference":
				validators = append(validators, URIReferenceValidator())
			case "iri":
				return fmt.Errorf("iri defined format not supported")
			case "iri-reference":
				return fmt.Errorf("iri-reference defined format not supported")
			case "uri-template":
				return fmt.Errorf("uri-template defined format not supported")
			case "json-pointer":
				validators = append(validators, JSONPointerValidator())
			case "relative-json-pointer":
				return fmt.Errorf("relative-json-pointer defined format not supported")
			case "regex":
				return fmt.Errorf("regex defined format not supported, use pattern keyword")
			}
		}
		validators = append(validators, additionalValidators...)
		err := o.handleBasicProperty(name, prefixes, validators, t, output, existingValues, required)
		if err != nil {
			return err
		}
	case "integer":
		validators := additionalValidators
		validators = append(validators, IntegerValidator())
		err := o.handleBasicProperty(name, prefixes, numberValidator(required, validators, t), t, output,
			existingValues, required)
		if err != nil {
			return err
		}
	}
	err := o.handleConditionals(prefixes, t.Required, name, t, parentType, output, existingValues)
	return err
}

// According to the spec, "An instance validates successfully against this keyword if its value
// is equal to the value of the keyword." which we interpret for questions as "this is the value of this keyword"
func (o *JSONSchemaOptions) handleConst(name string, validators []survey.Validator, t *Type, output *orderedmap.OrderedMap) error {
	message := fmt.Sprintf("Set %s to %v", name, *t.Const)
	if t.Title != "" {
		message = t.Title
	}
	// These are console output, not logging - DO NOT CHANGE THEM TO log statements
	fmt.Fprint(o.Out, message)
	if t.Description != "" {
		fmt.Fprint(o.Out, t.Description)
	}

	switch t.Type {
	case "boolean":
		tConst, err := util.AsBool(*t.Const)
		if err != nil {
			return err
		}
		output.Set(name, tConst)
	default:
		stringConst, err := util.AsString(*t.Const)
		if err != nil {
			return errors.Wrapf(err, "converting %s to string", *t.Const)
		}
		typedConst, err := convertAnswer(stringConst, t.Type)
		output.Set(name, typedConst)
	}

	return nil
}

func (o *JSONSchemaOptions) handleArrayProperty(name string, t *Type, output *orderedmap.OrderedMap,
	existingValues map[string]interface{}) error {
	results := make([]interface{}, 0)

	validators := []survey.Validator{
		MaxItemsValidator(t.MaxItems, results),
		UniqueItemsValidator(results),
		MinItemsValidator(t.MinItems, results),
		EnumValidator(t.Enum),
	}
	if t.Items.Type != nil && t.Items.Type.Enum != nil {
		// Arrays can used to create a multi-select list
		// Note that this only supports basic types at the moment
		if t.Items.Type.Type == "null" {
			output.Set(name, nil)
			return nil
		} else if !util.Contains([]string{"string", "boolean", "number", "integer"}, t.Items.Type.Type) {
			return fmt.Errorf("type %s is not supported for array %s", t.Items.Type.Type, name)
			// TODO support other types
		}
		message := fmt.Sprintf("Select values for %s", name)
		help := ""
		ask := true
		var defaultValue []string
		autoAcceptMessage := ""
		if value, ok := existingValues[name]; ok {
			if !o.AskExisting {
				ask = false
			}
			existingString, err := util.AsString(value)
			existingArray, err1 := util.AsSliceOfStrings(value)
			if err != nil && err1 != nil {
				v := reflect.ValueOf(value)
				v = reflect.Indirect(v)
				return fmt.Errorf("Cannot convert %v (%v) to string or []string", v.Type(), value)
			}
			if existingString != "" {
				defaultValue = []string{
					existingString,
				}
			} else {
				defaultValue = existingArray
			}
			autoAcceptMessage = "Automatically accepted existing value"
		} else if t.Default != nil {
			if o.AutoAcceptDefaults {
				ask = false
				autoAcceptMessage = "Automatically accepted default value"
			}
			defaultString, err := util.AsString(t.Default)
			defaultArray, err1 := util.AsSliceOfStrings(t.Default)
			if err != nil && err1 != nil {
				v := reflect.ValueOf(t.Default)
				v = reflect.Indirect(v)
				return fmt.Errorf("Cannot convert %value (%value) to string or []string", v.Type(), t.Default)
			}
			if defaultString != "" {
				defaultValue = []string{
					defaultString,
				}
			} else {
				defaultValue = defaultArray
			}
		}
		if o.NoAsk {
			ask = false
		}

		options := make([]string, 0)
		if t.Title != "" {
			message = t.Title
		}
		if t.Description != "" {
			help = t.Description
		}
		for _, e := range t.Items.Type.Enum {
			options = append(options, fmt.Sprintf("%v", e))
		}

		answer := make([]string, 0)
		surveyOpts := survey.WithStdio(o.In, o.Out, o.OutErr)
		// TODO^^^ Apply correct validators for type
		validator := survey.ComposeValidators(validators...)

		if ask {
			prompt := &survey.MultiSelect{
				Default: defaultValue,
				Help:    help,
				Message: message,
				Options: options,
			}
			err := survey.AskOne(prompt, &answer, validator, surveyOpts)
			if err != nil {
				return err
			}
		} else {
			answer = defaultValue
			msg := fmt.Sprintf("%s %s [%s]\n", message, util.ColorInfo(answer), autoAcceptMessage)
			_, err := fmt.Fprint(terminal.NewAnsiStdout(o.Out), msg)
			if err != nil {
				return errors.Wrapf(err, "writing %s to console", msg)
			}
		}

		for _, a := range answer {
			v, err := convertAnswer(a, t.Items.Type.Type)
			// An error is a genuine error as we've already done type validation
			if err != nil {
				return err
			}
			results = append(results, v)
		}
	}

	output.Set(name, results)
	return nil
}

func convertAnswer(answer string, t string) (interface{}, error) {
	if t == "number" {
		return strconv.ParseFloat(answer, 64)
	} else if t == "integer" {
		return strconv.Atoi(answer)
	} else if t == "boolean" {
		return strconv.ParseBool(answer)
	} else {
		return answer, nil
	}
}

func (o *JSONSchemaOptions) handleBasicProperty(name string, prefixes []string, validators []survey.Validator, t *Type,
	output *orderedmap.OrderedMap, existingValues map[string]interface{}, required bool) error {
	if t.Const != nil {
		return o.handleConst(name, validators, t, output)
	}

	// lets use the last vaultPath as the vaultKey. We don't need to use the format
	// in the vaultKey or vaultPath
	lastIdx := len(prefixes) - 1
	vaultKey := prefixes[lastIdx]
	pathPrefixes := prefixes[0:lastIdx]
	vaultPath := o.VaultBasePath
	vaultDir := strings.Join(pathPrefixes, "-")
	if vaultDir != "" {
		vaultPath = strings.Join([]string{vaultPath, vaultDir}, "/")
	}
	ask := true
	defaultValue := ""
	autoAcceptMessage := ""
	if v, ok := existingValues[name]; ok {
		if !o.AskExisting {
			ask = false
		}
		defaultValue = fmt.Sprintf("%v", v)
		autoAcceptMessage = "Automatically accepted existing value"
	} else if t.Default != nil {
		if o.AutoAcceptDefaults {
			ask = false
			autoAcceptMessage = "Automatically accepted default value"
		}
		defaultValue = fmt.Sprintf("%v", t.Default)
	}
	if o.NoAsk {
		ask = false
	}

	var result interface{}
	message := fmt.Sprintf("Enter a value for %s", name)
	help := ""
	if t.Title != "" {
		message = t.Title
	}
	if t.Description != "" {
		help = t.Description
	}

	if !ask {
		envVar := strings.ToUpper("JX_VALUE_" + strings.Join(prefixes, "_"))
		if defaultValue == "" {
			defaultValue = os.Getenv(envVar)
			if defaultValue != "" {
				log.Logger().Infof("defaulting value from $%s\n", envVar)
			}
		}
		if !o.IgnoreMissingValues && defaultValue == "" {
			// lets not fail if in batch mode for non-required fields
			if !o.NoAsk || required {
				return fmt.Errorf("no existing or default value in answer to question %s and no value for $%s", message, envVar)
			}
		}
	}

	surveyOpts := survey.WithStdio(o.In, o.Out, o.OutErr)
	validator := survey.ComposeValidators(validators...)
	// Ask the question
	// Custom format support for passwords
	dereferencedFormat := strings.TrimSuffix(util.DereferenceString(t.Format), "-passthrough")
	if dereferencedFormat == "password" || dereferencedFormat == "token" {
		// the default value for a password is just the path, so clear those values
		if _, ok := existingValues[name]; ok {
			defaultValue = ""
			ask = true
		}
		if o.VaultClient != nil {
			// the standard existing logic is not used in this case
			secret, err := o.VaultClient.Read(vaultPath)
			if err == nil {
				if value, ok := secret[vaultKey]; ok {
					if !o.AskExisting {
						ask = false
					}
					defaultValue = fmt.Sprintf("%v", value)
					autoAcceptMessage = "Automatically accepted existing value"
				}
			} else {
				// If there is an error, just continue
				log.Logger().Debugf("Error reading %s from vault %v", vaultPath, err)
			}
		}

		secret, err := handlePasswordProperty(message, help, dereferencedFormat, ask, validator, surveyOpts, defaultValue,
			autoAcceptMessage, o.Out, t.Type)
		if err != nil {
			return errors.WithStack(err)
		}
		if secret != nil {
			value, err := util.AsString(secret)
			if err != nil {
				return err
			}
			if o.VaultClient != nil {

				secretReference := secreturl.ToURI(vaultPath, vaultKey, o.VaultScheme)
				output.Set(name, secretReference)

				// lets upsert the vaultKey
				data, _ := o.VaultClient.Read(vaultPath)
				if data == nil {
					data = map[string]interface{}{}
				}
				data[vaultKey] = value
				_, err = o.VaultClient.Write(vaultPath, data)
				if err != nil {
					return errors.Wrapf(err, "failed to write to vaultPath %s", vaultPath)
				}
			} else {
				log.Logger().Warnf("Need to store a secret for %s but no secret store configured", name)
			}
		}
	} else if t.Enum != nil {
		var enumResult string
		// Support for selects
		names := make([]string, 0)
		for _, e := range t.Enum {
			names = append(names, fmt.Sprintf("%v", e))
		}
		prompt := &survey.Select{
			Message: message,
			Options: names,
			Default: defaultValue,
			Help:    help,
		}
		if ask {
			err := survey.AskOne(prompt, &enumResult, validator, surveyOpts)
			if err != nil {
				return err
			}
			result = enumResult
		} else {
			result = defaultValue
			msg := fmt.Sprintf("%s %s [%s]\n", message, util.ColorInfo(result), autoAcceptMessage)
			_, err := fmt.Fprint(terminal.NewAnsiStdout(o.Out), msg)
			if err != nil {
				return errors.Wrapf(err, "writing %s to console", msg)
			}
		}
	} else if t.Type == "boolean" {
		// Confirm dialog
		var d bool
		var err error
		if defaultValue != "" {
			d, err = strconv.ParseBool(defaultValue)
			if err != nil {
				return err
			}
		}

		var answer bool
		prompt := &survey.Confirm{
			Message: message,
			Help:    help,
			Default: d,
		}

		if ask {
			err = survey.AskOne(prompt, &answer, validator, surveyOpts)
			if err != nil {
				return errors.Wrapf(err, "error asking user %s using validators %v", message, validators)
			}
		} else {
			answer = d
			msg := fmt.Sprintf("%s %s [%s]\n", message, util.ColorInfo(answer), autoAcceptMessage)
			_, err := fmt.Fprint(terminal.NewAnsiStdout(o.Out), msg)
			if err != nil {
				return errors.Wrapf(err, "writing %s to console", msg)
			}
		}
		result = answer
	} else {
		// Basic input
		prompt := &survey.Input{
			Message: message,
			Default: defaultValue,
			Help:    help,
		}
		var answer string
		var err error
		if ask {
			err = survey.AskOne(prompt, &answer, validator, surveyOpts)
			if err != nil {
				return errors.Wrapf(err, "error asking user %s using validators %v", message, validators)
			}
		} else {
			answer = defaultValue
			msg := fmt.Sprintf("%s %s [%s]\n", message, util.ColorInfo(answer), autoAcceptMessage)
			_, err := fmt.Fprint(terminal.NewAnsiStdout(o.Out), msg)
			if err != nil {
				return errors.Wrapf(err, "writing %s to console", msg)
			}
		}
		if answer != "" {
			result, err = convertAnswer(answer, t.Type)
		}
		if err != nil {
			return errors.Wrapf(err, "error converting result %s to type %s", answer, t.Type)
		}
	}

	if result != nil {
		// Write the value to the output
		output.Set(name, result)
	}
	return nil
}

func handlePasswordProperty(message string, help string, kind string, ask bool, validator survey.Validator,
	surveyOpts survey.AskOpt, defaultValue string, autoAcceptMessage string, out terminal.FileWriter,
	t string) (interface{}, error) {
	// Secret input
	prompt := &survey.Password{
		Message: message,
		Help:    help,
	}

	generated := false
	if submatches := generatedPasswordValueRegex.FindStringSubmatch(defaultValue); len(submatches) > 0 {
		var secret string
		var err error
		if len(submatches) == 2 {
			format := submatches[1]
			switch format {
			case "hmac":
				secret, err = util.RandStringBytesMaskImprSrc(41)
				if err != nil {
					return nil, errors.Wrapf(err, "generating hmac")
				}
			case "":
				secret, err = secrets.DefaultGenerateSecret()
				if err != nil {
					return nil, errors.WithStack(err)
				}
			default:
				return nil, errors.Errorf("%s is not a known format to generate", format)
			}
		} else {
			secret, err = secrets.DefaultGenerateSecret()
			if err != nil {
				return nil, errors.WithStack(err)
			}
		}

		defaultValue = secret
		fmt.Fprintf(terminal.NewAnsiStdout(out), "Generated %s %s, to use it press enter.\nThis is the only time you will be shown it so remember to save it\n", kind, util.ColorInfo(secret))
		generated = true
	}

	var answer string
	if ask {
		err := survey.AskOne(prompt, &answer, validator, surveyOpts)
		if err != nil {
			return nil, err
		}
	} else {
		answer = defaultValue
		msg := fmt.Sprintf("%s *** [%s]\n", message, autoAcceptMessage)
		_, err := fmt.Fprint(terminal.NewAnsiStdout(out), msg)
		if err != nil {
			return nil, errors.Wrapf(err, "writing %s to console", msg)
		}
	}
	if answer == "" && generated {
		answer = defaultValue
	}
	if answer != "" {
		result, err := convertAnswer(answer, t)
		if err != nil {
			return nil, errors.Wrapf(err, "error converting answer %s to type %s", answer, t)
		}
		return result, nil
	}
	return nil, nil
}

func numberValidator(required bool, additonalValidators []survey.Validator, t *Type) []survey.Validator {
	validators := []survey.Validator{EnumValidator(t.Enum),
		MultipleOfValidator(t.MultipleOf),
		RequiredValidator(required),
		MinValidator(t.Minimum, false),
		MinValidator(t.ExclusiveMinimum, true),
		MaxValidator(t.Maximum, false),
		MaxValidator(t.ExclusiveMaximum, true),
	}
	return append(validators, additonalValidators...)
}
