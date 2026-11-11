package provider

import (
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func normalizeFQDN(s string) string {
	return strings.ToLower(strings.TrimSuffix(s, "."))
}

// Why not keep the API's form: an imported record would otherwise plan a replacement for "www.example.com." vs "www.example.com".
func fqdnValidator() validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(`^[a-z0-9_*]([a-z0-9_*-]*\.)+[a-z0-9-]*[a-z0-9]$`),
		"must be a lower-case FQDN including the domain name, without a trailing dot",
	)
}

func dnsValueEqual(a, b string) bool {
	return strings.TrimSuffix(a, ".") == strings.TrimSuffix(b, ".")
}

func dnsValue(prior types.String, actual string) types.String {
	if !prior.IsNull() && !prior.IsUnknown() && dnsValueEqual(prior.ValueString(), actual) {
		return prior
	}
	return types.StringValue(actual)
}

func nameserversEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	na := make([]string, len(a))
	nb := make([]string, len(b))
	for i := range a {
		na[i] = normalizeFQDN(a[i])
		nb[i] = normalizeFQDN(b[i])
	}
	slices.Sort(na)
	slices.Sort(nb)
	return slices.Equal(na, nb)
}
