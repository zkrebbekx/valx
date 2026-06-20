package valx_test

import (
	"testing"

	"github.com/zkrebbekx/valx"

	. "github.com/smartystreets/goconvey/convey"
)

type Address struct {
	City string `valx:"required,min=2"`
}

type Item struct {
	Name string `valx:"required"`
}

type Signup struct {
	Username string          `valx:"required,min=3,max=20"`
	Role     string          `valx:"oneof=admin editor viewer"`
	Age      int             `valx:"gt=0,lt=130"`
	Code     int             `valx:"oneof=1 2 3"`
	Tags     []string        `valx:"min=1,max=5"`
	PIN      string          `valx:"len=4"`
	Address  *Address        `valx:"required"`
	Items    []*Item         ``
	ByKey    map[string]Item ``
	Internal string          `valx:"-"`
	hidden   string          //nolint:unused // unexported: skipped
	Flag     bool            `valx:"min=1"` // rule doesn't apply to bool: no-op
	Optional *Address        ``             // nil, no `required`: skipped entirely
}

func valid() Signup {
	return Signup{
		Username: "alice",
		Role:     "admin",
		Age:      30,
		Code:     2,
		Tags:     []string{"go"},
		PIN:      "1234",
		Address:  &Address{City: "NYC"},
	}
}

func TestValidateAllRulesPass(t *testing.T) {
	Convey("Given a Signup satisfying every rule", t, func() {
		s := valid()

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it passes", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestRequired(t *testing.T) {
	Convey("Given a Signup with a zero-value required field", t, func() {
		s := valid()
		s.Username = ""

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it reports Username as required", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Username is required")
			})
		})
	})
}

func TestStringLengthBounds(t *testing.T) {
	Convey("Given a Signup with a username below the minimum length", t, func() {
		s := valid()
		s.Username = "ab"

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it reports the min violation", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Username must be at least 3")
			})
		})
	})

	Convey("Given a Signup with a username above the maximum length", t, func() {
		s := valid()
		s.Username = "this-username-is-way-too-long"

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it reports the max violation", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Username must be at most 20")
			})
		})
	})

	Convey("Given a multi-byte username within bounds", t, func() {
		s := valid()
		s.Username = "日本語" // 3 runes, well under the byte length

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then length is counted in runes, not bytes, so it passes", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestExactLength(t *testing.T) {
	Convey("Given a Signup with a PIN of the wrong length", t, func() {
		s := valid()
		s.PIN = "12"

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it reports the len violation", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "PIN must have length 4")
			})
		})
	})
}

func TestOneOfString(t *testing.T) {
	Convey("Given a Signup with a Role outside the allowed set", t, func() {
		s := valid()
		s.Role = "superuser"

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it reports the oneof violation listing the allowed values", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Role must be one of [admin editor viewer]")
			})
		})
	})
}

func TestOneOfNumeric(t *testing.T) {
	Convey("Given a Signup with a Code outside the allowed numeric set", t, func() {
		s := valid()
		s.Code = 9

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it reports the oneof violation", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Code must be one of [1 2 3]")
			})
		})
	})
}

func TestNumericRange(t *testing.T) {
	Convey("Given a Signup with Age at or below the gt bound", t, func() {
		s := valid()
		s.Age = 0

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it reports the gt violation", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Age must be greater than 0")
			})
		})
	})

	Convey("Given a Signup with Age at or above the lt bound", t, func() {
		s := valid()
		s.Age = 130

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it reports the lt violation", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Age must be less than 130")
			})
		})
	})
}

func TestSliceLengthBounds(t *testing.T) {
	Convey("Given a Signup with an empty Tags slice", t, func() {
		s := valid()
		s.Tags = nil

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it reports the min element-count violation", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Tags must be at least 1")
			})
		})
	})

	Convey("Given a Signup with too many Tags", t, func() {
		s := valid()
		s.Tags = []string{"a", "b", "c", "d", "e", "f"}

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it reports the max element-count violation", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Tags must be at most 5")
			})
		})
	})
}

func TestRuleSkippedForMismatchedKind(t *testing.T) {
	Convey("Given a Signup whose bool field carries a rule that doesn't apply to bool", t, func() {
		s := valid()
		s.Flag = false // zero value; min=1 has no meaning for bool

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then the inapplicable rule is silently skipped", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestNilPointerRequired(t *testing.T) {
	Convey("Given a Signup with a nil required pointer field", t, func() {
		s := valid()
		s.Address = nil

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it reports Address as required, without recursing into it", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Address is required")
			})
		})
	})
}

func TestNilOptionalPointerSkipped(t *testing.T) {
	Convey("Given a Signup with a nil, non-required pointer field", t, func() {
		s := valid()
		s.Optional = nil

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it is skipped without error or panic", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestNestedStructRecursion(t *testing.T) {
	Convey("Given a Signup whose nested Address fails its own rule", t, func() {
		s := valid()
		s.Address = &Address{City: "x"} // below min=2

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then the violation is reported with a dotted field path", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Address.City must be at least 2")
			})
		})
	})
}

func TestSliceOfStructRecursion(t *testing.T) {
	Convey("Given a Signup with an invalid struct inside a slice", t, func() {
		s := valid()
		s.Items = []*Item{{Name: "ok"}, {Name: ""}}

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then the violation is reported with an indexed field path", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "Items[1].Name is required")
			})
		})
	})
}

func TestMapOfStructRecursion(t *testing.T) {
	Convey("Given a Signup with an invalid struct inside a map", t, func() {
		s := valid()
		s.ByKey = map[string]Item{"a": {Name: ""}}

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then the violation is reported with a keyed field path", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "ByKey[a].Name is required")
			})
		})
	})
}

func TestExcludedFieldSkipped(t *testing.T) {
	Convey("Given a Signup whose excluded field is left empty", t, func() {
		s := valid()
		s.Internal = ""

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then it does not count as a violation", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestMultipleViolationsAggregated(t *testing.T) {
	Convey("Given a Signup violating more than one rule", t, func() {
		s := valid()
		s.Role = "superuser" // oneof
		s.Age = 0            // gt

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then both violations are reported together", func() {
				So(err, ShouldNotBeNil)
				ve, ok := err.(valx.ValidationErrors)
				So(ok, ShouldBeTrue)
				So(len(ve), ShouldEqual, 2)
				So(ve[0].Field, ShouldEqual, "Role")
				So(ve[1].Field, ShouldEqual, "Age")
			})
		})
	})
}

func TestCompoundRuleViolations(t *testing.T) {
	Convey("Given a field whose value fails two of its own rules at once", t, func() {
		s := valid()
		s.Username = "" // fails both required and min=3

		Convey("When validated", func() {
			err := valx.Validate(s)

			Convey("Then every failing rule for that field is reported, not just the first", func() {
				So(err, ShouldNotBeNil)
				ve, ok := err.(valx.ValidationErrors)
				So(ok, ShouldBeTrue)
				So(len(ve), ShouldEqual, 2)
				So(ve[0].Tag, ShouldEqual, "required")
				So(ve[1].Tag, ShouldEqual, "min")
			})
		})
	})
}

func TestValidateAcceptsPointer(t *testing.T) {
	Convey("Given a pointer to a valid Signup", t, func() {
		s := valid()

		Convey("When validated by pointer", func() {
			err := valx.Validate(&s)

			Convey("Then it dereferences and passes", func() {
				So(err, ShouldBeNil)
			})
		})
	})

	Convey("Given a typed nil pointer", t, func() {
		Convey("When validated", func() {
			err := valx.Validate((*Signup)(nil))

			Convey("Then it errors clearly rather than panicking", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "nil")
			})
		})
	})
}

func TestValidateRejectsUntypedNil(t *testing.T) {
	Convey("Given an untyped nil", t, func() {
		Convey("When validated", func() {
			var s any
			err := valx.Validate(s)

			Convey("Then it reports the nil rather than panicking", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "nil")
			})
		})
	})
}

func TestValidateRejectsNonStruct(t *testing.T) {
	Convey("Given a non-struct value", t, func() {
		Convey("When validated", func() {
			err := valx.Validate(42)

			Convey("Then it reports the misuse", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "requires a struct")
			})
		})
	})
}

func TestMustValidatePanics(t *testing.T) {
	Convey("Given an invalid Signup", t, func() {
		s := valid()
		s.Username = ""

		Convey("When MustValidate is called", func() {
			Convey("Then it panics", func() {
				So(func() { valx.MustValidate(s) }, ShouldPanic)
			})
		})

		Convey("When MustValidate is called on a valid Signup", func() {
			s := valid()

			Convey("Then it does not panic", func() {
				So(func() { valx.MustValidate(s) }, ShouldNotPanic)
			})
		})
	})
}

type badRuleName struct {
	Field string `valx:"bogus"`
}

type badRuleParam struct {
	Field string `valx:"min=notanumber"`
}

type missingParam struct {
	Field string `valx:"min"`
}

func TestMalformedTagPanics(t *testing.T) {
	Convey("Given a struct with an unknown rule name", t, func() {
		v := badRuleName{Field: "x"}

		Convey("When validated", func() {
			Convey("Then it panics rather than silently skipping the typo", func() {
				So(func() { _ = valx.Validate(v) }, ShouldPanic)
			})
		})
	})

	Convey("Given a struct whose numeric rule has a non-numeric parameter", t, func() {
		v := badRuleParam{Field: "x"}

		Convey("When validated", func() {
			Convey("Then it panics", func() {
				So(func() { _ = valx.Validate(v) }, ShouldPanic)
			})
		})
	})

	Convey("Given a struct whose rule is missing its required parameter", t, func() {
		v := missingParam{Field: "x"}

		Convey("When validated", func() {
			Convey("Then it panics", func() {
				So(func() { _ = valx.Validate(v) }, ShouldPanic)
			})
		})
	})
}
