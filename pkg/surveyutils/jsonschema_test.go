// +build unit

package surveyutils_test

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/secreturl"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/vault/fake"

	"gopkg.in/AlecAivazis/survey.v1/core"

	expect "github.com/Netflix/go-expect"
	"github.com/ghodss/yaml"

	"github.com/jenkins-x/jx/pkg/tests"

	"github.com/stretchr/testify/assert"

	"github.com/jenkins-x/jx/pkg/surveyutils"
)

// TODO Figure out how to test selects (affects arrays, enums, validation keywords for arrays)

var timeout = 5 * time.Second

const vaultBasePath = "fake"

type GeneratedSecret struct {
	Name  string
	Key   string
	Value string
}

func init() {
	// disable color output for all prompts to simplify testing
	core.DisableColor = true
}

func TestObjectType(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "objectType.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectString("Enter a value for name")
			console.SendLine("cheese")
			console.ExpectEOF()
		}, nil)
		assert.Equal(r, `nestedObject:
  anotherNestedObject:
    name: cheese
`, values)
		assert.NoError(r, err)
	})
}

func TestDescriptionAndTitle(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "descriptionAndTitle.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test explicit question
			console.ExpectString("What is your name?")
			console.SendLine("?")
			// Test explicit description
			console.ExpectString("Enter your name")
			console.SendLine("Pete")
			// Test no description
			console.ExpectString("What is your address?")
			console.SendLine("?")
			// Test no title
			console.ExpectString("Enter a value for country")
			console.SendLine("UK")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, `address: '?'
country: UK
name: Pete
`, values)
	})
}

func TestAutoAcceptDefaultValues(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "autoAcceptDefaultValues.test.schema.json", make(map[string]interface{}), false, false, true, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test explicit question
			//console.ExpectString("What is your name? John Smith [Automatically accepted default value]")
			//console.ExpectEOF()
			// TODO Fix the console test
		}, nil)
		assert.Equal(r, `name: John Smith
`, values)
		assert.NoError(r, err)
	})
}

func TestAcceptExisting(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "acceptExisting.test.schema.json", map[string]interface{}{
			"name": "John Smith",
		}, false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test explicit question
			console.ExpectString("What is your name? John Smith [Automatically accepted existing value]")
			console.ExpectEOF()
		}, nil)
		assert.Equal(r, `name: John Smith
`, values)
		assert.NoError(r, err)
	})
}

func TestAskExisting(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "askExisting.test.schema.json", map[string]interface{}{
			"name": "John Smith",
		}, true, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test explicit question
			console.ExpectString("What is your name? [? for help] (John Smith)")
			console.SendLine("")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, `name: John Smith
`, values)
	})
}

func TestNoAskAndAutoAcceptDefaultsWithExisting(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "noAskAndAutoAcceptDefaultsWithExisting.test.schema.json", map[string]interface{}{
			"name":    "John Smith",
			"country": "UK",
		}, false, true, true, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test explicit question
			// TODO fix this...
			//console.ExpectString("What is your name? John Smith [Automatically accepted existing value]")
			//console.ExpectString("Enter a value for country UK [Automatically accepted default value]")
			//console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, `country: UK
name: John Smith
`, values)
	})
}

func TestIgnoreMissingValues(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "ignoreMissingValues.test.schema.json", make(map[string]interface{}), false, true, false, true, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, `{}
`, values)
	})
}

func TestErrorMissingValues(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "ignoreMissingValues.test.schema.json", make(map[string]interface{}), false, true, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectEOF()
		}, nil)
		assert.NoError(t, err)
	})
}

func TestDefaultValues(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "defaultValues.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test default value
			console.ExpectString("Enter a value for stringValue (UK)")
			console.SendLine("")
			console.ExpectString("Enter a value for booleanValue (y/N)")
			console.SendLine("")
			console.ExpectString("Enter a value for numberValue (123.4)")
			console.SendLine("")
			console.ExpectString("Enter a value for integerValue (123)")
			console.SendLine("")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, `booleanValue: false
integerValue: 123
numberValue: 123.4
stringValue: UK
`, values)
	})
}

func TestConstValues(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "constValues.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test default value
			console.ExpectString("Set stringValue to UK")
			console.ExpectString("Set booleanValue to false")
			console.ExpectString("Set numberValue to 123.4")
			console.ExpectString("Set integerValue to 123")
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, `booleanValue: false
integerValue: 123
numberValue: 123.4
stringValue: UK
`, values)
	})
}

func TestBasicTypesValidation(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "basicTypesValidation.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("abc")
			console.ExpectString("Sorry, your reply was invalid: unable to convert abc to float64")
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("123.1")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("123.1")
			console.ExpectString("Sorry, your reply was invalid: unable to convert 123.1 to int")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("123")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestBasicTypes(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "basicTypes.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for booleanValue (y/N)")
			console.SendLine("Y")
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("123.4")
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("hello")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("123")
			console.ExpectEOF()
		}, nil)
		assert.Equal(r, `booleanValue: true
integerValue: 123
nullValue: null
numberValue: 123.4
stringValue: hello
`, values)
		assert.NoError(r, err)
	})
}

func TestMultipleOf(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "multipleOf.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("11.1")
			console.ExpectString("Sorry, your reply was invalid: 11.1 cannot be divided by 10")
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("10")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("12")
			console.ExpectString("Sorry, your reply was invalid: 12 cannot be divided by 20")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("20")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestMaximum(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "maximum.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("11.1")
			console.ExpectString("Sorry, your reply was invalid: 11.1 is not less than or equal to 10.1")
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("1")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("21")
			console.ExpectString("Sorry, your reply was invalid: 21 is not less than or equal to 20")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("2")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestExclusiveMaximum(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "exclusiveMaximum.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("10.1")
			console.ExpectString("Sorry, your reply was invalid: 10.1 is not less than 10.1")
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("1")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("20")
			console.ExpectString("Sorry, your reply was invalid: 20 is not less than 20")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("2")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestMinimum(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "minimum.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("9.1")
			console.ExpectString("Sorry, your reply was invalid: 9.1 is not greater than or equal to 10.1")
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("11")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("19")
			console.ExpectString("Sorry, your reply was invalid: 19 is not greater than or equal to 20")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("21")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestExclusiveMinimum(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "exclusiveMinimum.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("10.1")
			console.ExpectString("Sorry, your reply was invalid: 10.1 is not greater than 10.1")
			console.ExpectString("Enter a value for numberValue")
			console.SendLine("11")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("20")
			console.ExpectString("Sorry, your reply was invalid: 20 is not greater than 20")
			console.ExpectString("Enter a value for integerValue")
			console.SendLine("21")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestMaxLength(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "maxLength.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("iamlongerthan10")
			console.ExpectString("Sorry, your reply was invalid: value is too long. Max length is 10")
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("short")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestMinLength(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "minLength.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("short")
			console.ExpectString("Sorry, your reply was invalid: value is too short. Min length is 10")
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("iamlongerthan10")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestPattern(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "pattern.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("HELLO")
			console.ExpectString("Sorry, your reply was invalid: HELLO does not match [0-9]")
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("123")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestRequired(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "required.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("")
			console.ExpectString("Sorry, your reply was invalid: Value is required")
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("Hello")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestIfThen(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "ifThenElse.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectString("Enter a value for enablePersistentStorage")
			console.SendLine("Y")
			console.ExpectString("Enter a value for databaseConnectionUrl")
			console.SendLine("abc")
			console.ExpectString("Enter a value for databaseUsername")
			console.SendLine("wensleydale")
			console.ExpectString("Enter a value for databasePassword")
			console.SendLine("cranberries")
			console.ExpectString(" ***********")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, fmt.Sprintf(`databaseConnectionUrl: abc
databasePassword: vault:%s:databasePassword
databaseUsername: wensleydale
enablePersistentStorage: true
`, vaultBasePath), values)
	})
}

func TestIfElse(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "ifThenElse.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectString("Enter a value for enablePersistentStorage")
			console.SendLine("N")
			console.ExpectString("Enter a value for enableInMemoryDB")
			console.SendLine("N")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, `enableInMemoryDB: false
enablePersistentStorage: false
`, values)
	})
}

func TestIfElseTrueBoolean(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "ifThenElseTrueBoolean.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectString("Enter a value for enablePersistentStorage")
			console.SendLine("N")
			console.ExpectString("Enter a value for enableInMemoryDB")
			console.SendLine("N")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, `enableInMemoryDB: false
enablePersistentStorage: false
`, values)
	})
}

func TestIfElseNested(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "ifThenElseNested.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectString("Enter a value for enablePersistentStorage")
			console.SendLine("N")
			console.ExpectString("Enter a value for enableInMemoryDB")
			console.SendLine("Y")
			console.ExpectString("Enter a value for nestedString")
			console.SendLine("Test")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, `nestedObject:
  enableInMemoryDB: true
  enablePersistentStorage: false
  nestedString: Test
`, values)
	})
}

func TestIfElseWithDefaults(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "ifThenElse.test.schema.json", make(map[string]interface{}), false, false, true, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectString("Enter a value for enablePersistentStorage")
			console.SendLine("N")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, `enableInMemoryDB: true
enablePersistentStorage: false
`, values)
	})
}

func TestAllOf(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "AllOfIf.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectString("Enter a value for enablePersistentStorage")
			console.SendLine("Y")
			console.ExpectString("Enter a value for databaseConnectionUrl")
			console.SendLine("abc")
			console.ExpectString("Enter a value for databaseUsername")
			console.SendLine("wensleydale")
			console.ExpectString("Enter a value for databasePassword")
			console.SendLine("cranberries")
			console.ExpectString(" ***********")
			console.ExpectString("Enter a value for enableCheese")
			console.SendLine("Y")
			console.ExpectString("Enter a value for cheeseType")
			console.SendLine("Stilton")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, fmt.Sprintf(`cheeseType: Stilton
databaseConnectionUrl: abc
databasePassword: vault:%s:databasePassword
databaseUsername: wensleydale
enableCheese: true
enablePersistentStorage: true
`, vaultBasePath), values)
	})
}

func TestAllOfThen(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, _, err := GenerateValuesAsYaml(r, "AllOfIf.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectString("Enter a value for enablePersistentStorage")
			console.SendLine("Y")
			console.ExpectString("Enter a value for databaseConnectionUrl")
			console.SendLine("abc")
			console.ExpectString("Enter a value for databaseUsername")
			console.SendLine("wensleydale")
			console.ExpectString("Enter a value for databasePassword")
			console.SendLine("cranberries")
			console.ExpectString(" ***********")
			console.ExpectString("Enter a value for enableCheese")
			console.SendLine("N")
			console.ExpectString("Enter a value for iDontLikeCheese")
			console.SendLine("Y")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
		assert.Equal(r, fmt.Sprintf(`databaseConnectionUrl: abc
databasePassword: vault:%s:databasePassword
databaseUsername: wensleydale
enableCheese: false
enablePersistentStorage: true
iDontLikeCheese: true
`, vaultBasePath), values)
	})
}

func TestMinProperties(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "minProperties.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("")
			console.ExpectString("Enter a value for stringValue1")
			console.SendLine("")
			console.ExpectString("Sorry, your reply was invalid: nestedObject has less than 1 items, has []")
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("abc")
			console.ExpectString("Enter a value for stringValue1")
			console.SendLine("def")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestMaxProperties(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "maxProperties.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("abc")
			console.ExpectString("Enter a value for stringValue1")
			console.SendLine("def")
			console.ExpectString("Sorry, your reply was invalid: nestedObject has more than 1 items, " +
				"has [stringValue stringValue1]")
			console.ExpectString("Enter a value for stringValue")
			console.SendLine("abc")
			console.ExpectString("Enter a value for stringValue1")
			console.SendLine("")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestDateTime(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "dateTime.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for dateTimeValue")
			console.SendLine("abc")
			console.ExpectString("Sorry, your reply was invalid: abc is not a RFC 3339 date-time formatted string, " +
				"it should be like 2006-01-02T15:04:05Z07:00")
			console.ExpectString("Enter a value for dateTimeValue")
			console.SendLine("2006-01-02T15:04:05-07:00")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestDate(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "date.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for dateValue")
			console.SendLine("abc")
			console.ExpectString("Sorry, your reply was invalid: abc is not a RFC 3339 full-date formatted string, " +
				"it should be like 2006-01-02")
			console.ExpectString("Enter a value for dateValue")
			console.SendLine("2006-01-02")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestTime(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "time.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for timeValue")
			console.SendLine("abc")
			console.ExpectString("Sorry, your reply was invalid: abc is not a RFC 3339 full-time formatted string, " +
				"it should be like 15:04:05Z07:00")
			console.ExpectString("Enter a value for timeValue")
			console.SendLine("15:04:05-07:00")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestPassword(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, vaultClient, err := GenerateValuesAsYaml(r, "password.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for passwordValue")
			console.SendLine("abc")
			console.ExpectEOF()
		}, nil)
		assert.Equal(r, fmt.Sprintf(`passwordValue: vault:%s:passwordValue
`, vaultBasePath), values)
		secrets, err := vaultClient.Read(vaultBasePath)
		assert.NoError(t, err)
		assert.Equal(r, "abc", secrets["passwordValue"])
		assert.NoError(r, err)
	})
}

func TestExistingPassword(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 1, time.Second*10, func(r *tests.R) {
		values, vaultClient, err := GenerateValuesAsYaml(r, "password.test.schema.json", map[string]interface{}{
			"passwordValue": map[string]string{
				"password": "vault:/foo/bar",
			},
		}, false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			console.ExpectString("Enter a value for passwordValue")
			console.SendLine("abc")
			console.ExpectEOF()
		}, nil)
		assert.Equal(r, fmt.Sprintf(`passwordValue: vault:%s:passwordValue
`, vaultBasePath), values)
		secrets, err := vaultClient.Read(vaultBasePath)
		assert.NoError(t, err)
		assert.Equal(r, "abc", secrets["passwordValue"])
		assert.NoError(r, err)
	})
}

func TestToken(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		values, vaultClient, err := GenerateValuesAsYaml(r, "token.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for tokenValue")
			console.SendLine("abc")
			console.ExpectEOF()
		}, nil)
		assert.Equal(r, fmt.Sprintf(`tokenValue: vault:%s:tokenValue
`, vaultBasePath), values)
		secrets, err := vaultClient.Read(vaultBasePath)
		assert.NoError(t, err)
		assert.Equal(r, "abc", secrets["tokenValue"])
		assert.NoError(r, err)
	})
}

func TestGeneratedToken(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 1, time.Second*10, func(r *tests.R) {
		values, vaultClient, err := GenerateValuesAsYaml(r, "generatedToken.test.schema.json", make(map[string]interface{}), false,
			false,
			false, false,
			func(console *tests.ConsoleWrapper, donec chan struct{}) {
				defer close(donec)
				// Test boolean type
				console.ExpectString("Enter a value for tokenValue")
				console.SendLine("")
				console.ExpectEOF()
			}, nil)
		assert.Equal(r, fmt.Sprintf(`tokenValue: vault:%s:tokenValue
`, vaultBasePath), values)
		secrets, err := vaultClient.Read(vaultBasePath)
		assert.NoError(t, err)
		assert.Len(t, secrets["tokenValue"], 20)
		assert.NoError(r, err)
	})
}

func TestGeneratedHmacToken(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 1, time.Second*10, func(r *tests.R) {
		values, vaultClient, err := GenerateValuesAsYaml(r, "generatedHmacToken.test.schema.json", make(map[string]interface{}), false,
			false,
			false, false,
			func(console *tests.ConsoleWrapper, donec chan struct{}) {
				defer close(donec)
				// Test boolean type
				console.ExpectString("Enter a value for tokenValue")
				console.SendLine("")
				console.ExpectEOF()
			}, nil)
		assert.Equal(r, fmt.Sprintf(`tokenValue: vault:%s:tokenValue
`, vaultBasePath), values)
		secrets, err := vaultClient.Read(vaultBasePath)
		assert.NoError(t, err)
		value := secrets["tokenValue"]
		valueStr, err := util.AsString(value)
		assert.NoError(t, err)
		assert.Len(t, valueStr, 41)
		hexRegex := regexp.MustCompile(`^(0x|0X)?[a-fA-F0-9]+$`)
		assert.True(t, hexRegex.MatchString(valueStr), "%s is not a hexadecimal string")
		assert.NoError(r, err)
	})
}

func TestExistingToken(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 1, time.Second*10, func(r *tests.R) {
		vaultClient := fake.NewFakeVaultClient()
		vaultClient.Write(vaultBasePath, map[string]interface{}{
			"tokenValue": "abc",
		})
		values, _, err := GenerateValuesAsYaml(r, "token.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for tokenValue *** [Automatically accepted existing value]")
			console.ExpectEOF()
		}, vaultClient)

		assert.Equal(r, fmt.Sprintf(`tokenValue: vault:%s:tokenValue
`, vaultBasePath), values)
		secrets, err := vaultClient.Read(vaultBasePath)
		assert.NoError(t, err)
		assert.Equal(r, "abc", secrets["tokenValue"])
		assert.NoError(r, err)
	})
}

func TestEmail(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "email.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for emailValue")
			console.SendLine("abc")
			console.ExpectString("Sorry, your reply was invalid: abc is not a RFC 5322 address, " +
				"it should be like Barry Gibb <bg@example.com>")
			console.ExpectString("Enter a value for emailValue")
			console.SendLine("Maurice Gibb <mg@example.com>")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestIdnEmail(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "idnemail.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for emailValue")
			console.SendLine("abc")
			console.ExpectString("Sorry, your reply was invalid: abc is not a RFC 5322 address, " +
				"it should be like Barry Gibb <bg@example.com>")
			console.ExpectString("Enter a value for emailValue")
			console.SendLine("Maurice Gibb <mg@example.com>")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestHostname(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "hostname.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for hostnameValue")
			console.SendLine("*****")
			console.ExpectString("Sorry, your reply was invalid: ***** is not a RFC 1034 hostname, " +
				"it should be like example.com")
			console.ExpectString("Enter a value for hostnameValue")
			console.SendLine("example.com")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestIdnHostname(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "idnhostname.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for hostnameValue")
			console.SendLine("*****")
			console.ExpectString("Sorry, your reply was invalid: ***** is not a RFC 1034 hostname, " +
				"it should be like example.com")
			console.ExpectString("Enter a value for hostnameValue")
			console.SendLine("example.com")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestIpv4(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "ipv4.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for ipv4Value")
			console.SendLine("abc")
			console.ExpectString("Sorry, your reply was invalid: abc is not a RFC 2673 IPv4 Address, " +
				"it should be like 127.0.0.1")
			console.ExpectString("Enter a value for ipv4Value")
			console.SendLine("127.0.0.1")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestIpv6(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "ipv6.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for ipv6Value")
			console.SendLine("abc")
			console.ExpectString("Sorry, your reply was invalid: abc is not a RFC 4291 IPv6 address, " +
				"it should be like ::1")
			console.ExpectString("Enter a value for ipv6Value")
			console.SendLine("::1")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestUri(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "uri.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for uriValue")
			console.SendLine("*****")
			console.ExpectString("Sorry, your reply was invalid: ***** is not a RFC 3986 URI")
			console.ExpectString("Enter a value for uriValue")
			console.SendLine("https://example.com")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestUriReference(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "uriReference.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for uriReferenceValue")
			console.SendLine("http$$://foo")
			console.ExpectString("Sorry, your reply was invalid: http$$://foo is not a RFC 3986 URI reference")
			console.ExpectString("Enter a value for uriReferenceValue")
			console.SendLine("../resource.txt")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)
	})
}

func TestJSONPointer(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		_, _, err := GenerateValuesAsYaml(r, "jsonPointer.test.schema.json", make(map[string]interface{}), false, false, false, false, func(console *tests.ConsoleWrapper, donec chan struct{}) {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for jsonPointerValue")
			console.SendLine("~")
			console.ExpectString("Sorry, your reply was invalid: ~ is not a RFC 6901 JSON pointer")
			console.ExpectString("Enter a value for jsonPointerValue")
			console.SendLine("/abc")
			console.ExpectEOF()
		}, nil)
		assert.NoError(r, err)

	})
}

func GenerateValuesAsYaml(r *tests.R, schemaName string, existingValues map[string]interface{}, askExisting bool, noAsk bool, autoAcceptDefaults bool, ignoreMissingValues bool, answerQuestions func(
	console *tests.ConsoleWrapper, donec chan struct{}), vaultClient secreturl.Client) (string, secreturl.Client, error) {

	//t.Parallel()
	console := tests.NewTerminal(r, &timeout)
	defer console.Cleanup()
	if vaultClient == nil {
		vaultClient = fake.NewFakeVaultClient()
	}

	options := surveyutils.JSONSchemaOptions{
		Out:                 console.Out,
		In:                  console.In,
		OutErr:              console.Err,
		AskExisting:         askExisting,
		AutoAcceptDefaults:  autoAcceptDefaults,
		NoAsk:               noAsk,
		IgnoreMissingValues: ignoreMissingValues,
		VaultClient:         vaultClient,
		VaultBasePath:       vaultBasePath,
		VaultScheme:         "vault",
	}
	data, err := ioutil.ReadFile(filepath.Join("test_data", schemaName))
	assert.NoError(r, err)

	// Test interactive IO
	donec := make(chan struct{})
	go answerQuestions(console, donec)
	assert.NoError(r, err)
	result, runErr := options.GenerateValues(
		data,
		existingValues)
	console.Close()
	<-donec
	yaml, err := yaml.JSONToYAML(result)
	consoleOut := expect.StripTrailingEmptyLines(console.CurrentState())
	r.Logf(consoleOut)
	assert.NoError(r, err)
	return string(yaml), vaultClient, runErr
}
