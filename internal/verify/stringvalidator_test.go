package verify

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/terraform-providers/terraform-provider-ncloud/internal/verify"
)

func TestContainValidator(t *testing.T) {
	t.Parallel()

	type testCase struct {
		in        types.String
		validator validator.String
		expErrors int
	}

	testCases := map[string]testCase{
		"contains-substring": {
			in: types.StringValue("hello123"),
			validator: verify.NotContain("hello"),
			expErrors: 1,
		},
		"does-not-contain-substring": {
			in: types.StringValue("goodbye"),
			validator: verify.NotContain("hello"),
			expErrors: 0,
		},
		"confirm-other-attr": {
			in: types.StringValue("gooyehi99993"),
			validator: verify.NotContain(path.MatchRoot("does-not-contain-substring").String()),
			expErrors: 0,
		},
		"skip-validation-on-null": {
			in: types.StringNull(),
			validator: verify.NotContain("hello"),
			expErrors: 0,
		},
		"skip-validation-on-unknown": {
			in: types.StringUnknown(),
			validator: verify.NotContain("hello"),
			expErrors: 0,
		},
	}

	for name, test := range testCases {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := validator.StringRequest{
				ConfigValue: test.in,
			}
			res := validator.StringResponse{}
			test.validator.ValidateString(context.TODO(), req, &res)

			if test.expErrors > 0 && !res.Diagnostics.HasError() {
				t.Fatalf("expected %d error(s), got none", test.expErrors)
			}

			if test.expErrors > 0 && test.expErrors != res.Diagnostics.ErrorsCount() {
				t.Fatalf("expected %d error(s), got %d: %v", test.expErrors, res.Diagnostics.ErrorsCount(), res.Diagnostics)
			}

			if test.expErrors == 0 && res.Diagnostics.HasError() {
				t.Fatalf("expected no error(s), got %d: %v", res.Diagnostics.ErrorsCount(), res.Diagnostics)
			}
		})
	}
}

func TestContainValidator_Description(t *testing.T) {
	t.Parallel()

	type testCase struct {
		target   string
		expected string
	}

	testCases := map[string]testCase{
		"simple-description": {
			target:   "foo",
			expected: "Value must not contain the string 'foo'.",
		},
	}

	for name, test := range testCases {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			v := verify.NotContain(test.target)

			got := v.MarkdownDescription(context.Background())

			if diff := cmp.Diff(got, test.expected); diff != "" {
				t.Errorf("unexpected difference: %s", diff)
			}
		})
	}
}