package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal/apiresponse"

	"github.com/go-playground/validator/v10"
)

// V is the shared validator (struct tags like validate:"required,email").
var V = validator.New()

// DecodeJSON reads the body into dst then runs validator.Struct(dst).
func DecodeJSON(r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := V.Struct(dst); err != nil {
		return err
	}
	return nil
}

// Var runs validation rules on a single value (e.g. path/query params).
func Var(field interface{}, tag string) error {
	return V.Var(field, tag)
}

// WriteError sends RFC 7807 problem+json. Validation → 422; JSON syntax / type mismatch → 400.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		errs := make([]apiresponse.FieldProblem, 0, len(verrs))
		for _, fe := range verrs {
			errs = append(errs, apiresponse.FieldProblem{
				Field:   jsonFieldName(fe),
				Message: humanize(fe),
				Code:    tagToCode(fe.Tag()),
			})
		}
		apiresponse.WriteProblem(w, r, http.StatusUnprocessableEntity,
			apiresponse.ProblemTypeURI(r, "validation"),
			"Validation Failed",
			"One or more fields failed validation.",
			errs,
		)
		return
	}

	var syn *json.SyntaxError
	var ut *json.UnmarshalTypeError
	if errors.As(err, &syn) || errors.As(err, &ut) {
		apiresponse.WriteProblem(w, r, http.StatusBadRequest,
			apiresponse.ProblemTypeURI(r, "invalid-json"),
			"Invalid JSON",
			"The request body must be valid JSON.",
			nil,
		)
		return
	}

	apiresponse.WriteProblem(w, r, http.StatusBadRequest,
		apiresponse.ProblemTypeURI(r, "bad-request"),
		"Bad Request",
		"The request could not be understood.",
		nil,
	)
}

func jsonFieldName(fe validator.FieldError) string {
	n := fe.Namespace()
	if i := strings.IndexByte(n, '.'); i >= 0 {
		return n[i+1:]
	}
	return fe.Field()
}

func tagToCode(tag string) string {
	switch tag {
	case "required":
		return "REQUIRED"
	case "email", "uuid", "datetime":
		return "INVALID_FORMAT"
	case "min", "max", "gte", "lte", "len":
		return "OUT_OF_RANGE"
	case "oneof":
		return "INVALID_VALUE"
	default:
		return "INVALID"
	}
}

func humanize(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "required"
	case "email":
		return "must be a valid email address"
	case "uuid":
		return "must be a valid UUID"
	case "min":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "gte":
		return fmt.Sprintf("must be greater than or equal to %s", fe.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", strings.ReplaceAll(fe.Param(), " ", ", "))
	case "startswith":
		return fmt.Sprintf("must start with %q", fe.Param())
	default:
		return fmt.Sprintf("failed on %s", fe.Tag())
	}
}
