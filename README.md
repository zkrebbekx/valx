# valx

A small, zero-dependency struct field validator for Go — `oneof`, `min`/`max`/`len`
for strings, numbers, and array/slice/map element counts, `gt`/`lt` for numeric
ranges, and `required` — in the spirit of
[go-playground/validator](https://github.com/go-playground/validator) (v10),
without the dependency.

Library is zero-dependency. Go 1.23+.

```go
import "github.com/zkrebbekx/valx"
```

## Usage

```go
type SignupRequest struct {
	Username string   `valx:"required,min=3,max=20"`
	Role     string   `valx:"oneof=admin editor viewer"`
	Age      int      `valx:"gt=0,lt=130"`
	Tags     []string `valx:"max=5"`
}

func Handle(r SignupRequest) error {
	if err := valx.Validate(r); err != nil {
		return err
		// "valx: Username is required; Role must be one of [admin editor viewer]"
	}
	...
}
```

`Validate` walks every exported field, applies the rules in its `valx` tag,
and **recurses into nested structs** — directly, through a pointer, or through
a slice/array/map of either — so one call validates an entire request graph,
with field paths like `Address.City` or `Items[2].Name`.

Every violation is collected and returned together as `valx.ValidationErrors`
(a `[]FieldError`), not just the first one found.

## Rules

| tag | applies to | check |
|---|---|---|
| `required` | any kind | not the zero value (nil for a pointer) |
| `oneof=a b c` | string, int\*, uint\*, float\* | value equals one of the space-separated tokens |
| `min=N` | string / number / slice, array, map | string: ≥N runes · number: ≥N · collection: ≥N elements |
| `max=N` | string / number / slice, array, map | string: ≤N runes · number: ≤N · collection: ≤N elements |
| `len=N` | string / number / slice, array, map | exactly N (runes / value / elements) |
| `gt=N` | int\*, uint\*, float\* | strictly greater than N |
| `lt=N` | int\*, uint\*, float\* | strictly less than N |

Stack multiple rules on one field, comma-separated: `valx:"required,min=1,max=5"`.

- A rule that doesn't apply to a field's kind (e.g. `min` on a `bool`) is
  silently skipped — the same convention
  [`depx`](https://github.com/zkrebbekx/depx) uses for dependency checks. A
  tag is written once for a type that may be reused in different contexts;
  not every rule needs to apply to every kind it might see.
- `valx:"-"` excludes a field entirely — no rules, no recursion.
- Unexported fields are always skipped.
- A nil pointer skips every rule except `required` (there's nothing to check
  `min`/`max`/etc. against), and is not recursed into.
- string length is counted in **runes**, not bytes.

A malformed tag — an unknown rule name, or a parameter that isn't a valid
number where one is required — is a programmer error, not a data error: no
input will ever fix it. `Validate` panics on it immediately, so a typo is
caught by the first test run instead of being silently skipped forever.

## Error shape

```go
err := valx.Validate(r)
var ve valx.ValidationErrors
if errors.As(err, &ve) {
	for _, fe := range ve {
		fmt.Println(fe.Field, fe.Tag, fe.Param) // e.g. "Username" "min" "3"
	}
}
```

`valx.MustValidate(r)` panics instead, for places where an invalid value
should stop the program (e.g. validating loaded config at start-up).

## Develop

```sh
make test
make lint
```

## License

MIT
