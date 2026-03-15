package testengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchUsers(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		ctx        *SignInContext
		want       bool
	}{
		{
			name:       "no user conditions matches all",
			conditions: map[string]interface{}{},
			ctx:        &SignInContext{User: "user-1"},
			want:       true,
		},
		{
			name: "All includes matches any user",
			conditions: map[string]interface{}{
				"users": map[string]interface{}{
					"includeUsers": []interface{}{"All"},
				},
			},
			ctx:  &SignInContext{User: "user-1"},
			want: true,
		},
		{
			name: "specific user included",
			conditions: map[string]interface{}{
				"users": map[string]interface{}{
					"includeUsers": []interface{}{"user-1", "user-2"},
				},
			},
			ctx:  &SignInContext{User: "user-1"},
			want: true,
		},
		{
			name: "user not in include list",
			conditions: map[string]interface{}{
				"users": map[string]interface{}{
					"includeUsers": []interface{}{"user-1"},
				},
			},
			ctx:  &SignInContext{User: "user-99"},
			want: false,
		},
		{
			name: "user included but excluded overrides",
			conditions: map[string]interface{}{
				"users": map[string]interface{}{
					"includeUsers": []interface{}{"All"},
					"excludeUsers": []interface{}{"user-1"},
				},
			},
			ctx:  &SignInContext{User: "user-1"},
			want: false,
		},
		{
			name: "user included via group",
			conditions: map[string]interface{}{
				"users": map[string]interface{}{
					"includeGroups": []interface{}{"group-a"},
				},
			},
			ctx:  &SignInContext{User: "user-1", Groups: []string{"group-a", "group-b"}},
			want: true,
		},
		{
			name: "user included via role",
			conditions: map[string]interface{}{
				"users": map[string]interface{}{
					"includeRoles": []interface{}{"role-admin"},
				},
			},
			ctx:  &SignInContext{User: "user-1", Roles: []string{"role-admin"}},
			want: true,
		},
		{
			name: "group excluded overrides user include",
			conditions: map[string]interface{}{
				"users": map[string]interface{}{
					"includeUsers":  []interface{}{"All"},
					"excludeGroups": []interface{}{"break-glass-group"},
				},
			},
			ctx:  &SignInContext{User: "user-1", Groups: []string{"break-glass-group"}},
			want: false,
		},
		{
			name: "GuestsOrExternalUsers matches guest",
			conditions: map[string]interface{}{
				"users": map[string]interface{}{
					"includeUsers": []interface{}{"GuestsOrExternalUsers"},
				},
			},
			ctx:  &SignInContext{User: "guest"},
			want: true,
		},
		{
			name: "GuestsOrExternalUsers does not match member",
			conditions: map[string]interface{}{
				"users": map[string]interface{}{
					"includeUsers": []interface{}{"GuestsOrExternalUsers"},
				},
			},
			ctx:  &SignInContext{User: "user-1"},
			want: false,
		},
		{
			name: "empty include lists matches nobody",
			conditions: map[string]interface{}{
				"users": map[string]interface{}{},
			},
			ctx:  &SignInContext{User: "user-1"},
			want: false,
		},
		{
			name: "GuestsOrExternalUsers in exclude blocks guest",
			conditions: map[string]interface{}{
				"users": map[string]interface{}{
					"includeUsers": []interface{}{"All"},
					"excludeUsers": []interface{}{"GuestsOrExternalUsers"},
				},
			},
			ctx:  &SignInContext{User: "guest"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchUsers(tt.conditions, tt.ctx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchApplications(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		ctx        *SignInContext
		want       bool
	}{
		{
			name:       "no application conditions matches all",
			conditions: map[string]interface{}{},
			ctx:        &SignInContext{Application: "app-1"},
			want:       true,
		},
		{
			name: "All includes matches any app",
			conditions: map[string]interface{}{
				"applications": map[string]interface{}{
					"includeApplications": []interface{}{"All"},
				},
			},
			ctx:  &SignInContext{Application: "app-1"},
			want: true,
		},
		{
			name: "specific app matches",
			conditions: map[string]interface{}{
				"applications": map[string]interface{}{
					"includeApplications": []interface{}{"app-1"},
				},
			},
			ctx:  &SignInContext{Application: "app-1"},
			want: true,
		},
		{
			name: "app not in include list",
			conditions: map[string]interface{}{
				"applications": map[string]interface{}{
					"includeApplications": []interface{}{"app-1"},
				},
			},
			ctx:  &SignInContext{Application: "app-99"},
			want: false,
		},
		{
			name: "app included but excluded",
			conditions: map[string]interface{}{
				"applications": map[string]interface{}{
					"includeApplications": []interface{}{"All"},
					"excludeApplications": []interface{}{"app-1"},
				},
			},
			ctx:  &SignInContext{Application: "app-1"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchApplications(tt.conditions, tt.ctx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchClientAppTypes(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		ctx        *SignInContext
		want       bool
	}{
		{
			name:       "no clientAppTypes matches all",
			conditions: map[string]interface{}{},
			ctx:        &SignInContext{ClientAppType: "browser"},
			want:       true,
		},
		{
			name: "all keyword matches any type",
			conditions: map[string]interface{}{
				"clientAppTypes": []interface{}{"all"},
			},
			ctx:  &SignInContext{ClientAppType: "exchangeActiveSync"},
			want: true,
		},
		{
			name: "specific type matches",
			conditions: map[string]interface{}{
				"clientAppTypes": []interface{}{"browser", "mobileAppsAndDesktopClients"},
			},
			ctx:  &SignInContext{ClientAppType: "browser"},
			want: true,
		},
		{
			name: "type not in list",
			conditions: map[string]interface{}{
				"clientAppTypes": []interface{}{"browser"},
			},
			ctx:  &SignInContext{ClientAppType: "exchangeActiveSync"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchClientAppTypes(tt.conditions, tt.ctx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchPlatforms(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		ctx        *SignInContext
		want       bool
	}{
		{
			name:       "no platforms block matches all",
			conditions: map[string]interface{}{},
			ctx:        &SignInContext{Platform: "windows"},
			want:       true,
		},
		{
			name: "all includes matches any platform",
			conditions: map[string]interface{}{
				"platforms": map[string]interface{}{
					"includePlatforms": []interface{}{"all"},
				},
			},
			ctx:  &SignInContext{Platform: "iOS"},
			want: true,
		},
		{
			name: "specific platform matches",
			conditions: map[string]interface{}{
				"platforms": map[string]interface{}{
					"includePlatforms": []interface{}{"windows", "macOS"},
				},
			},
			ctx:  &SignInContext{Platform: "windows"},
			want: true,
		},
		{
			name: "platform not in list",
			conditions: map[string]interface{}{
				"platforms": map[string]interface{}{
					"includePlatforms": []interface{}{"iOS"},
				},
			},
			ctx:  &SignInContext{Platform: "android"},
			want: false,
		},
		{
			name: "platform excluded overrides include",
			conditions: map[string]interface{}{
				"platforms": map[string]interface{}{
					"includePlatforms": []interface{}{"all"},
					"excludePlatforms": []interface{}{"android"},
				},
			},
			ctx:  &SignInContext{Platform: "android"},
			want: false,
		},
		{
			name: "empty platform in context matches",
			conditions: map[string]interface{}{
				"platforms": map[string]interface{}{
					"includePlatforms": []interface{}{"windows"},
				},
			},
			ctx:  &SignInContext{Platform: ""},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPlatforms(tt.conditions, tt.ctx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchLocations(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		ctx        *SignInContext
		want       bool
	}{
		{
			name:       "no locations block matches all",
			conditions: map[string]interface{}{},
			ctx:        &SignInContext{Location: "trusted"},
			want:       true,
		},
		{
			name: "All includes matches any location",
			conditions: map[string]interface{}{
				"locations": map[string]interface{}{
					"includeLocations": []interface{}{"All"},
				},
			},
			ctx:  &SignInContext{Location: "some-guid"},
			want: true,
		},
		{
			name: "AllTrusted matches trusted location",
			conditions: map[string]interface{}{
				"locations": map[string]interface{}{
					"includeLocations": []interface{}{"AllTrusted"},
				},
			},
			ctx:  &SignInContext{Location: "trusted"},
			want: true,
		},
		{
			name: "AllTrusted does not match untrusted",
			conditions: map[string]interface{}{
				"locations": map[string]interface{}{
					"includeLocations": []interface{}{"AllTrusted"},
				},
			},
			ctx:  &SignInContext{Location: "untrusted"},
			want: false,
		},
		{
			name: "specific location GUID matches",
			conditions: map[string]interface{}{
				"locations": map[string]interface{}{
					"includeLocations": []interface{}{"loc-guid-1"},
				},
			},
			ctx:  &SignInContext{Location: "loc-guid-1"},
			want: true,
		},
		{
			name: "location excluded overrides include",
			conditions: map[string]interface{}{
				"locations": map[string]interface{}{
					"includeLocations": []interface{}{"All"},
					"excludeLocations": []interface{}{"loc-guid-1"},
				},
			},
			ctx:  &SignInContext{Location: "loc-guid-1"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchLocations(tt.conditions, tt.ctx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchSignInRiskLevels(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		ctx        *SignInContext
		want       bool
	}{
		{
			name:       "empty risk levels matches all",
			conditions: map[string]interface{}{},
			ctx:        &SignInContext{SignInRiskLevel: "high"},
			want:       true,
		},
		{
			name: "risk level in list matches",
			conditions: map[string]interface{}{
				"signInRiskLevels": []interface{}{"medium", "high"},
			},
			ctx:  &SignInContext{SignInRiskLevel: "high"},
			want: true,
		},
		{
			name: "risk level not in list",
			conditions: map[string]interface{}{
				"signInRiskLevels": []interface{}{"medium", "high"},
			},
			ctx:  &SignInContext{SignInRiskLevel: "low"},
			want: false,
		},
		{
			name: "none risk level matches none filter",
			conditions: map[string]interface{}{
				"signInRiskLevels": []interface{}{"none"},
			},
			ctx:  &SignInContext{SignInRiskLevel: "none"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchSignInRiskLevels(tt.conditions, tt.ctx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchUserRiskLevels(t *testing.T) {
	tests := []struct {
		name       string
		conditions map[string]interface{}
		ctx        *SignInContext
		want       bool
	}{
		{
			name:       "empty user risk levels matches all",
			conditions: map[string]interface{}{},
			ctx:        &SignInContext{UserRiskLevel: "high"},
			want:       true,
		},
		{
			name: "user risk level in list matches",
			conditions: map[string]interface{}{
				"userRiskLevels": []interface{}{"high"},
			},
			ctx:  &SignInContext{UserRiskLevel: "high"},
			want: true,
		},
		{
			name: "user risk level not in list",
			conditions: map[string]interface{}{
				"userRiskLevels": []interface{}{"high"},
			},
			ctx:  &SignInContext{UserRiskLevel: "low"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchUserRiskLevels(tt.conditions, tt.ctx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchStringList(t *testing.T) {
	tests := []struct {
		name    string
		include []string
		exclude []string
		values  []string
		want    bool
	}{
		{
			name:    "All includes matches anything",
			include: []string{"All"},
			exclude: nil,
			values:  []string{"anything"},
			want:    true,
		},
		{
			name:    "specific match",
			include: []string{"a", "b"},
			exclude: nil,
			values:  []string{"b"},
			want:    true,
		},
		{
			name:    "no match",
			include: []string{"a"},
			exclude: nil,
			values:  []string{"b"},
			want:    false,
		},
		{
			name:    "exclude overrides include",
			include: []string{"All"},
			exclude: []string{"a"},
			values:  []string{"a"},
			want:    false,
		},
		{
			name:    "empty include returns false",
			include: nil,
			exclude: nil,
			values:  []string{"a"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchStringList(tt.include, tt.exclude, tt.values)
			assert.Equal(t, tt.want, got)
		})
	}
}
