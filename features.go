package kibana

import (
	"context"
	"encoding/json"
	"net/http"
)

// FeaturesService handles communication with the features method of the
// Kibana API.
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-system
type FeaturesService service

// Feature is a Kibana feature, such as "dashboard_v2" or "discover_v2". Its
// ID and privilege names are what [KibanaPrivilege.Feature] grants.
//
// The alerting and cases fields of a feature are not decoded.
type Feature struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Order and Category place the feature in the role management UI.
	Order    int              `json:"order,omitempty"`
	Category *FeatureCategory `json:"category,omitempty"`

	// App and Catalogue list the applications and catalogue entries the
	// feature contains.
	App       []string `json:"app,omitempty"`
	Catalogue []string `json:"catalogue,omitempty"`

	// Management maps management sections to the entries the feature
	// contains.
	Management map[string][]string `json:"management,omitempty"`

	// MinimumLicense is the license the feature needs, if any, such as
	// "gold".
	MinimumLicense string `json:"minimumLicense,omitempty"`

	// Privileges maps the feature privileges, usually "all" and "read", to
	// what they grant. It is nil for features that only have reserved
	// privileges, such as "monitoring".
	Privileges map[string]FeaturePrivilege `json:"privileges,omitempty"`

	// SubFeatures are the sub-feature privileges that can be granted on top
	// of a feature privilege, with KibanaPrivilege.Feature too.
	SubFeatures []SubFeature `json:"subFeatures,omitempty"`

	// Reserved holds the reserved privileges, which only built-in roles
	// get, as returned by Kibana.
	Reserved json.RawMessage `json:"reserved,omitempty"`

	// Hidden features are not shown in the role management UI.
	Hidden bool `json:"hidden,omitempty"`

	// ExcludeFromBasePrivileges is true if the "all" and "read" base
	// privileges do not grant the feature.
	ExcludeFromBasePrivileges bool `json:"excludeFromBasePrivileges,omitempty"`
}

// FeatureCategory is the category a [Feature] is listed under.
type FeatureCategory struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	EUIIconType string `json:"euiIconType,omitempty"`
	Order       int    `json:"order,omitempty"`
}

// FeaturePrivilege is what a feature privilege grants.
type FeaturePrivilege struct {
	// App, Catalogue, API, UI, SavedObject and Management are what the
	// privilege grants: applications, catalogue entries, API tags, UI
	// capabilities, access to saved object types and management entries.
	App         []string                     `json:"app,omitempty"`
	Catalogue   []string                     `json:"catalogue,omitempty"`
	API         []string                     `json:"api,omitempty"`
	UI          []string                     `json:"ui,omitempty"`
	SavedObject FeatureSavedObjectPrivileges `json:"savedObject"`
	Management  map[string][]string          `json:"management,omitempty"`

	// Disabled privileges cannot be granted.
	Disabled bool `json:"disabled,omitempty"`

	// RequireAllSpaces privileges can only be granted in all spaces ("*").
	RequireAllSpaces bool `json:"requireAllSpaces,omitempty"`
}

// FeatureSavedObjectPrivileges lists the saved object types a privilege
// grants full and read-only access to.
type FeatureSavedObjectPrivileges struct {
	All  []string `json:"all"`
	Read []string `json:"read"`
}

// SubFeature is a group of sub-feature privileges of a [Feature].
type SubFeature struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// RequireAllSpaces sub-features can only be granted in all spaces.
	RequireAllSpaces bool                       `json:"requireAllSpaces,omitempty"`
	PrivilegeGroups  []SubFeaturePrivilegeGroup `json:"privilegeGroups,omitempty"`
}

// SubFeaturePrivilegeGroupType is how the privileges of a
// [SubFeaturePrivilegeGroup] can be combined.
type SubFeaturePrivilegeGroupType string

// Sub-feature privilege group types. Kibana may send others.
const (
	// SubFeaturePrivilegeGroupTypeIndependent privileges can be granted in
	// any combination.
	SubFeaturePrivilegeGroupTypeIndependent SubFeaturePrivilegeGroupType = "independent"

	// SubFeaturePrivilegeGroupTypeMutuallyExclusive privileges can be
	// granted at most one at a time.
	SubFeaturePrivilegeGroupTypeMutuallyExclusive SubFeaturePrivilegeGroupType = "mutually_exclusive"
)

// SubFeaturePrivilegeGroup is a group of sub-feature privileges.
type SubFeaturePrivilegeGroup struct {
	GroupType  SubFeaturePrivilegeGroupType `json:"groupType"`
	Privileges []SubFeaturePrivilege        `json:"privileges"`
}

// SubFeaturePrivilege is a sub-feature privilege. Its ID is granted next to
// the feature privilege, e.g. {"dashboard_v2": ["read", "url_create"]}.
type SubFeaturePrivilege struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// IncludeIn is the feature privilege that already includes this one:
	// "all", "read" or "none".
	IncludeIn string `json:"includeIn"`

	// MinimumLicense is the license the privilege needs, if any. The other
	// fields are what it grants, as in [FeaturePrivilege].
	MinimumLicense string                       `json:"minimumLicense,omitempty"`
	API            []string                     `json:"api,omitempty"`
	UI             []string                     `json:"ui,omitempty"`
	SavedObject    FeatureSavedObjectPrivileges `json:"savedObject"`
	Management     map[string][]string          `json:"management,omitempty"`
}

// ListFeaturesOptions are the options of [FeaturesService.List].
type ListFeaturesOptions struct {
	_ [0]func()

	// IgnoreValidLicenses also returns the features that the current
	// license does not allow.
	IgnoreValidLicenses bool `url:"ignoreValidLicenses,omitempty"`
}

// List returns the Kibana features. The list depends on the Kibana version,
// its configuration and the license. Feature IDs differ between versions:
// e.g. Kibana 8.19 has "dashboard" where 9.x has "dashboard_v2".
//
// The endpoint is in technical preview. It is not in the Kibana 8.19 API
// specification, but Kibana 8.19 serves it with the same shape.
//
// Kibana API: GET /api/features
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-features
func (s *FeaturesService) List(ctx context.Context, opt *ListFeaturesOptions, opts ...RequestOption) ([]*Feature, *Response, error) {
	path, err := addQuery("/api/features", opt)
	if err != nil {
		return nil, nil, err
	}
	var features []*Feature
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &features, nil, opts)
	if err != nil {
		return nil, resp, err
	}
	return features, resp, nil
}
