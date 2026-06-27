// Package valx validates the *values* of a struct's fields against rules
// declared in a `valx` struct tag — a small, zero-dependency, reflection-based
// validator in the spirit of go-playground/validator (v10), covering the
// common cases without the dependency footprint.
//
//	type SignupRequest struct {
//		Username string   `valx:"required,min=3,max=20"`
//		Role     string   `valx:"oneof=admin editor viewer"`
//		Age      int      `valx:"gt=0,lt=130"`
//		Tags     []string `valx:"max=5"`
//	}
//
//	func Handle(r SignupRequest) error {
//		if err := valx.Validate(r); err != nil {
//			return err // "valx: Username must be at least 3 characters; Role must be one of [admin editor viewer]"
//		}
//		...
//	}
//
// [Validate] walks every exported field, applies the rules in its `valx` tag,
// and recurses into nested structs (and structs reachable through pointers,
// slices, arrays, and maps) so a single call validates an entire request
// graph. All violations are collected and returned together as
// [ValidationErrors], rather than stopping at the first failure.
//
// Supported rules:
//
//	required    zero value not allowed (any kind; for a pointer, nil is the zero value)
//	oneof=a b c value's formatted form must equal one of the space-separated tokens
//	min=N       string: at least N runes; number: >= N; slice/array/map: at least N elements
//	max=N       string: at most N runes; number: <= N; slice/array/map: at most N elements
//	len=N       string: exactly N runes; number: == N; slice/array/map: exactly N elements
//	gt=N        number: strictly greater than N
//	lt=N        number: strictly less than N
//
// A rule that does not apply to a field's kind (e.g. min on a bool) is
// silently skipped, the same convention [github.com/zkrebbekx/depx] uses for
// dependency checks — a tag is written once for a type used in many contexts,
// and not every rule need apply to every kind it might see.
//
// A tag whose syntax is malformed — an unknown rule name, or a parameter that
// doesn't parse as a number where one is required — is a programmer error,
// not a data error: it can never be fixed by different input, only by
// editing the tag. Validate therefore panics immediately, the same instant
// the field is reached, so the mistake surfaces in the first test run rather
// than as a silently-skipped check.
package valx

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

// FieldError reports a single rule violation.
type FieldError struct {
	// Field is the dotted/indexed path to the field, e.g. "Address.City" or
	// "Tags[2]".
	Field string
	// Tag is the rule name that failed (e.g. "min", "oneof").
	Tag string
	// Param is the rule's parameter as written in the tag (e.g. "3", or
	// "admin editor viewer" for oneof). Empty for "required".
	Param string
}

func (e FieldError) Error() string {
	switch e.Tag {
	case "required":
		return fmt.Sprintf("%s is required", e.Field)
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", e.Field, e.Param)
	case "min":
		return fmt.Sprintf("%s must be at least %s", e.Field, e.Param)
	case "max":
		return fmt.Sprintf("%s must be at most %s", e.Field, e.Param)
	case "len":
		return fmt.Sprintf("%s must have length %s", e.Field, e.Param)
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", e.Field, e.Param)
	case "lt":
		return fmt.Sprintf("%s must be less than %s", e.Field, e.Param)
	default:
		return fmt.Sprintf("%s failed %s=%s", e.Field, e.Tag, e.Param)
	}
}

// ValidationErrors collects every [FieldError] found by [Validate], in
// field-declaration order. It implements error.
type ValidationErrors []FieldError

func (e ValidationErrors) Error() string {
	msgs := make([]string, len(e))
	for i, fe := range e {
		msgs[i] = fe.Error()
	}
	return "valx: " + strings.Join(msgs, "; ")
}

// Validate checks every exported field of s against its `valx` tag, s must
// be a struct or a non-nil pointer to one. It returns nil if every rule
// passes, or a non-nil [ValidationErrors] naming every violation found.
//
// Validate performs only reads and holds no state, so it is safe to call
// concurrently.
func Validate[T any](s T) error {
	return ValidateWith(s, "valx")
}

// ValidateWith is like [Validate] but reads rules from the struct tag named
// tagKey instead of "valx". It lets one struct carry two independent rule sets —
// for example a "valx" tag for hard constraints and another key for soft ones —
// each validated separately. An empty tagKey defaults to "valx".
func ValidateWith[T any](s T, tagKey string) error {
	if tagKey == "" {
		tagKey = "valx"
	}
	v := reflect.ValueOf(s)
	if !v.IsValid() {
		return fmt.Errorf("valx: Validate called on a nil value")
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return fmt.Errorf("valx: Validate called on a nil %T", s)
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("valx: Validate requires a struct, got %s", v.Kind())
	}

	var errs ValidationErrors
	walkStruct(v, "", tagKey, &errs)
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// MustValidate is like [Validate] but panics on any violation. Use it where
// a malformed value should stop the program rather than be reported.
func MustValidate[T any](s T) {
	if err := Validate(s); err != nil {
		panic(err)
	}
}

func walkStruct(v reflect.Value, prefix, tagKey string, errs *ValidationErrors) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get(tagKey)
		if tag == "-" {
			continue
		}

		path := f.Name
		if prefix != "" {
			path = prefix + "." + f.Name
		}

		rules := parseRules(tag, path)
		fv := v.Field(i)

		if fv.Kind() == reflect.Pointer {
			if fv.IsNil() {
				if hasRule(rules, "required") {
					*errs = append(*errs, FieldError{Field: path, Tag: "required"})
				}
				continue
			}
			fv = fv.Elem()
		}

		for _, r := range rules {
			if fe, ok := checkRule(fv, path, r); ok {
				*errs = append(*errs, fe)
			}
		}

		walkValue(fv, path, tagKey, errs)
	}
}

// walkValue recurses into structs reachable from fv: directly, through a
// slice/array/map of structs, or through pointers to either.
func walkValue(fv reflect.Value, path, tagKey string, errs *ValidationErrors) {
	switch fv.Kind() {
	case reflect.Struct:
		walkStruct(fv, path, tagKey, errs)
	case reflect.Slice, reflect.Array:
		for i := 0; i < fv.Len(); i++ {
			elem := indirect(fv.Index(i))
			if elem.Kind() == reflect.Struct {
				walkStruct(elem, fmt.Sprintf("%s[%d]", path, i), tagKey, errs)
			}
		}
	case reflect.Map:
		for _, k := range fv.MapKeys() {
			elem := indirect(fv.MapIndex(k))
			if elem.Kind() == reflect.Struct {
				walkStruct(elem, fmt.Sprintf("%s[%v]", path, k.Interface()), tagKey, errs)
			}
		}
	}
}

func indirect(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return v
		}
		v = v.Elem()
	}
	return v
}

// rule is one parsed clause of a `valx` tag, e.g. min=3 -> {Name: "min", Param: "3"}.
type rule struct {
	Name  string
	Param string
}

func hasRule(rules []rule, name string) bool {
	for _, r := range rules {
		if r.Name == name {
			return true
		}
	}
	return false
}

// known rule names; checked against at parse time so a typo panics
// immediately rather than being silently skipped.
var knownRules = map[string]bool{
	"required": true,
	"oneof":    true,
	"min":      true,
	"max":      true,
	"len":      true,
	"gt":       true,
	"lt":       true,
}

func parseRules(tag, path string) []rule {
	if tag == "" {
		return nil
	}
	clauses := strings.Split(tag, ",")
	rules := make([]rule, 0, len(clauses))
	for _, c := range clauses {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		name, param, _ := strings.Cut(c, "=")
		if !knownRules[name] {
			panic(fmt.Sprintf("valx: %s: unknown rule %q in tag %q", path, name, tag))
		}
		if name != "required" && param == "" {
			panic(fmt.Sprintf("valx: %s: rule %q requires a parameter in tag %q", path, name, tag))
		}
		rules = append(rules, rule{Name: name, Param: param})
	}
	return rules
}

func checkRule(fv reflect.Value, path string, r rule) (FieldError, bool) {
	switch r.Name {
	case "required":
		if fv.IsZero() {
			return FieldError{Field: path, Tag: "required"}, true
		}
	case "oneof":
		if !checkOneOf(fv, r.Param) {
			return FieldError{Field: path, Tag: "oneof", Param: r.Param}, true
		}
	case "min":
		n := parseFloatParam(path, r)
		if count, ok := countOf(fv); ok && count < n {
			return FieldError{Field: path, Tag: "min", Param: r.Param}, true
		}
	case "max":
		n := parseFloatParam(path, r)
		if count, ok := countOf(fv); ok && count > n {
			return FieldError{Field: path, Tag: "max", Param: r.Param}, true
		}
	case "len":
		n := parseFloatParam(path, r)
		if count, ok := countOf(fv); ok && count != n {
			return FieldError{Field: path, Tag: "len", Param: r.Param}, true
		}
	case "gt":
		n := parseFloatParam(path, r)
		if val, ok := numericOf(fv); ok && val <= n {
			return FieldError{Field: path, Tag: "gt", Param: r.Param}, true
		}
	case "lt":
		n := parseFloatParam(path, r)
		if val, ok := numericOf(fv); ok && val >= n {
			return FieldError{Field: path, Tag: "lt", Param: r.Param}, true
		}
	}
	return FieldError{}, false
}

func parseFloatParam(path string, r rule) float64 {
	n, err := strconv.ParseFloat(r.Param, 64)
	if err != nil {
		panic(fmt.Sprintf("valx: %s: rule %q has a non-numeric parameter %q", path, r.Name, r.Param))
	}
	return n
}

// countOf reports the "size" of fv for min/max/len: rune count for strings,
// numeric value for numbers, element count for slices/arrays/maps. ok is
// false for kinds none of those rules apply to, so the caller skips silently.
func countOf(fv reflect.Value) (float64, bool) {
	switch fv.Kind() {
	case reflect.String:
		return float64(utf8.RuneCountInString(fv.String())), true
	case reflect.Slice, reflect.Array, reflect.Map:
		return float64(fv.Len()), true
	default:
		return numericOf(fv)
	}
}

// numericOf reports fv's value as a float64 for int/uint/float kinds only.
func numericOf(fv reflect.Value) (float64, bool) {
	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(fv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(fv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return fv.Float(), true
	default:
		return 0, false
	}
}

func checkOneOf(fv reflect.Value, param string) bool {
	tokens := strings.Fields(param)
	switch fv.Kind() {
	case reflect.String:
		s := fv.String()
		for _, tok := range tokens {
			if s == tok {
				return true
			}
		}
		return false
	default:
		val, ok := numericOf(fv)
		if !ok {
			// oneof doesn't apply to this kind; treat as a no-op pass,
			// consistent with other rules skipping kinds they don't cover.
			return true
		}
		for _, tok := range tokens {
			n, err := strconv.ParseFloat(tok, 64)
			if err == nil && n == val {
				return true
			}
		}
		return false
	}
}
