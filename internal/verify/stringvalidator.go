package verify

import (
	"fmt"
	"regexp"
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
)

func InstanceNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthAtLeast(3),
		stringvalidator.LengthAtMost(30),
		stringvalidator.RegexMatches(
			regexp.MustCompile("^[a-z][a-z0-9-]*$"),
			"must only lowercase letters, numbers and special characters \"-\" are allowed and must start with an alphabetic character",
		),
		stringvalidator.RegexMatches(
			regexp.MustCompile("^.*[^-_]$"),
			"must end with an alphabetic character or number",
		),
	}
}

func ValidateEmptyStringElement(i []interface{}) error {
	for _, v := range i {
		if v == nil || v == "" {
			return fmt.Errorf("empty string element found")
		}
	}
	return nil
}

type notContainValidator struct {
    target string
}

func NotContain(target string) validator.String {
    return notContainValidator{
        target: target,
    }
}

func (v notContainValidator) Description(ctx context.Context) string {
    return fmt.Sprintf("Value must not contain the string '%s'.", v.target)
}

func (v notContainValidator) MarkdownDescription(ctx context.Context) string {
    return v.Description(ctx)
}

func (v notContainValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
    if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
        return
    }

    if strings.Contains(req.ConfigValue.ValueString(), v.target) {
        resp.Diagnostics.AddAttributeError(
            req.Path,
            "Invalid Value",
            fmt.Sprintf("Value must not contain the string '%s'.", v.target),
        )
    }
}
