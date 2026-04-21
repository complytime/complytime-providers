// SPDX-License-Identifier: Apache-2.0

package xccdf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antchfx/xmlquery"

	xccdf "github.com/complytime/complytime-providers/cmd/openscap-provider/xccdftype"
)

var testDataDir = filepath.Join("..", "..", "..", "internal", "complytime", "testdata", "openscap")

// Helper function to load Datastream XML file. It is used by multiple tests in xccdf package.
func LoadDsTest(t *testing.T, dsTestFile string) (*xmlquery.Node, error) {
	dsTestFilePath := filepath.Join(testDataDir, dsTestFile)
	file, err := os.Open(dsTestFilePath)
	if err != nil {
		t.Fatalf("error opening datastream file: %v", err)
		return nil, err
	}
	defer file.Close()

	dsDom, err := xmlquery.Parse(file)
	if err != nil {
		t.Fatalf("error parsing datastream file: %v", err)
		return nil, err
	}

	return dsDom, nil
}

// TestLoadDataStream tests the loadDataStream function.
// Errors are expected for absent and invalid XML files that cannot be parsed.
func TestLoadDataStream(t *testing.T) {
	tests := []struct {
		dsPath  string
		wantErr bool
	}{
		{filepath.Join(testDataDir, "ssg-rhel-ds.xml"), false},
		{filepath.Join(testDataDir, "absent.xml"), true},
		{filepath.Join(testDataDir, "invalid.xml"), true},
	}

	for _, tt := range tests {
		t.Run(tt.dsPath, func(t *testing.T) {
			_, err := loadDataStream(tt.dsPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadDataStream() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetDsProfileID tests the getDsProfileID function.
func TestGetDsProfileID(t *testing.T) {
	tests := []struct {
		profileId string
		expected  string
	}{
		{"test", "xccdf_org.ssgproject.content_profile_test"},
		{"profile1", "xccdf_org.ssgproject.content_profile_profile1"},
		{"", "xccdf_org.ssgproject.content_profile_"},
	}

	for _, tt := range tests {
		t.Run(tt.profileId, func(t *testing.T) {
			result := getDsProfileID(tt.profileId)
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestGetDsRuleID tests the getDsRuleID function.
func TestGetDsRuleID(t *testing.T) {
	tests := []struct {
		ruleId   string
		expected string
	}{
		{"test_rule", "xccdf_org.ssgproject.content_rule_test_rule"},
		{"rule1", "xccdf_org.ssgproject.content_rule_rule1"},
		{"", "xccdf_org.ssgproject.content_rule_"},
	}

	for _, tt := range tests {
		t.Run(tt.ruleId, func(t *testing.T) {
			result := getDsRuleID(tt.ruleId)
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestGetDsVarID tests the getDsVarID function.
func TestGetDsVarID(t *testing.T) {
	tests := []struct {
		varId    string
		expected string
	}{
		{"test_var", "xccdf_org.ssgproject.content_value_test_var"},
		{"var1", "xccdf_org.ssgproject.content_value_var1"},
		{"", "xccdf_org.ssgproject.content_value_"},
	}

	for _, tt := range tests {
		t.Run(tt.varId, func(t *testing.T) {
			result := getDsVarID(tt.varId)
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestGetDsElement tests the getDsElement function with the Profile "description" element.
func TestGetDsElement(t *testing.T) {
	doc, _ := LoadDsTest(t, "ssg-rhel-ds.xml")

	tests := []struct {
		dsElement string
		expected  string
		wantErr   bool
	}{
		{"//xccdf-1.2:Profile[@id='xccdf_org.ssgproject.content_profile_test_profile']", "This profile is only used for Unit Tests", false},
		{"//xccdf-1.2:Profile[@id='xccdf_org.ssgproject.content_profile_absent']", "", false},
		{"//invalid:Profile[@id='xccdf_org.ssgproject.content_profile_test_profile']", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.dsElement, func(t *testing.T) {
			result, err := getDsElement(doc, tt.dsElement)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDsElement() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != nil && err == nil {
				profileDescription := result.SelectElement("xccdf-1.2:description")
				if profileDescription.InnerText() != tt.expected {
					t.Errorf("got %s, want %s", profileDescription.InnerText(), tt.expected)
				}
			}
		})
	}
}

// TestGetDsElementAttrValue tests the getDsElementAttrValue function.
func TestGetDsElementAttrValue(t *testing.T) {
	tests := []struct {
		xmlContent    string
		attributeName string
		expected      string
		wantErr       bool
	}{
		{
			xmlContent:    `<Element id="xccdf_org.ssgproject.content_profile_test_profile"/>`,
			attributeName: "id",
			expected:      "xccdf_org.ssgproject.content_profile_test_profile",
			wantErr:       false,
		},
		{
			xmlContent:    `<Element id="test_id" name="test_name"/>`,
			attributeName: "name",
			expected:      "test_name",
			wantErr:       false,
		},
		{
			xmlContent:    `<Element id="test_id" name="test_name"/>`,
			attributeName: "nonexistent",
			expected:      "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.attributeName, func(t *testing.T) {
			doc, err := xmlquery.Parse(strings.NewReader(tt.xmlContent))
			if err != nil {
				t.Fatalf("failed to parse XML: %v", err)
			}

			element := doc.SelectElement("Element")
			if element == nil {
				t.Fatalf("failed to select element")
			}

			result, err := getDsElementAttrValue(element, tt.attributeName)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDsElementAttrValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestGetDsOptionalAttrValue tests the getDsOptionalAttrValue function.
// The getDsOptionalAttrValue function depends on getDsElementAttrValue and most of this test
// seems redundant but still has value as it tests the function in isolation.
func TestGetDsOptionalAttrValue(t *testing.T) {
	tests := []struct {
		xmlContent    string
		attributeName string
		expected      string
	}{
		{
			xmlContent:    `<Element id="xccdf_org.ssgproject.content_profile_test_profile"/>`,
			attributeName: "id",
			expected:      "xccdf_org.ssgproject.content_profile_test_profile",
		},
		{
			xmlContent:    `<Element id="test_id" name="test_name"/>`,
			attributeName: "name",
			expected:      "test_name",
		},
		{
			xmlContent:    `<Element id="test_id" name="test_name"/>`,
			attributeName: "nonexistent",
			expected:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.attributeName, func(t *testing.T) {
			doc, err := xmlquery.Parse(strings.NewReader(tt.xmlContent))
			if err != nil {
				t.Fatalf("failed to parse XML: %v", err)
			}

			element := doc.SelectElement("Element")
			if element == nil {
				t.Fatalf("failed to select element")
			}

			result := getDsOptionalAttrValue(element, tt.attributeName)
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestGetDsElements tests the getDsElements function.
func TestGetDsElements(t *testing.T) {
	doc, _ := LoadDsTest(t, "ssg-rhel-ds.xml")
	tests := []struct {
		dsElement string
		expected  int
		wantErr   bool
	}{
		{"//xccdf-1.2:Profile", 4, false},
		{"//xccdf-1.2:Value", 71, false},
		{"//invalid:Element", 0, false},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.dsElement, func(t *testing.T) {
			result, err := getDsElements(doc, tt.dsElement)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDsElements() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(result) != tt.expected {
				t.Errorf("got %d elements, want %d", len(result), tt.expected)
			}
		})
	}
}

// TestGetDsProfileInternal tests the getDsProfile function.
func TestGetDsProfileInternal(t *testing.T) {
	doc, _ := LoadDsTest(t, "ssg-rhel-ds.xml")
	tests := []struct {
		dsProfileID string
		expected    string
		wantErr     bool
	}{
		{"xccdf_org.ssgproject.content_profile_test_profile", "This profile is only used for Unit Tests", false},
		{"xccdf_org.ssgproject.content_profile_absent", "", false},
		{"invalid_profile", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.dsProfileID, func(t *testing.T) {
			result, err := getDsProfile(doc, tt.dsProfileID)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDsProfile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != nil && err == nil {
				profileDescription := result.SelectElement("xccdf-1.2:description")
				if profileDescription.InnerText() != tt.expected {
					t.Errorf("got %s, want %s", profileDescription.InnerText(), tt.expected)
				}
			}
		})
	}
}

// TestGetDsElementTitleNode tests the getDsElementTitle function.
func TestGetDsElementTitleNode(t *testing.T) {
	doc, _ := LoadDsTest(t, "ssg-rhel-ds.xml")
	tests := []struct {
		dsProfileID string
		expected    string
		wantErr     bool
	}{
		{"xccdf_org.ssgproject.content_profile_test_profile", "Test Profile", false},
		{"xccdf_org.ssgproject.content_profile_test_profile_no_title", "", false},
		{"xccdf_org.ssgproject.content_profile_absent", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.dsProfileID, func(t *testing.T) {
			dsProfile, err := getDsProfile(doc, tt.dsProfileID)
			if err != nil {
				t.Fatalf("failed to get profile: %v", err)
			}

			result, err := getDsElementTitle(dsProfile)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDsElementTitle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != nil && err == nil {
				if result.InnerText() != tt.expected {
					t.Errorf("got %s, want %s", result.InnerText(), tt.expected)
				}
			}
		})
	}
}

// TestGetDsElementDescriptionNode tests the getDsElementDescription function.
func TestGetDsElementDescriptionNode(t *testing.T) {
	doc, _ := LoadDsTest(t, "ssg-rhel-ds.xml")
	tests := []struct {
		dsProfileID string
		expected    string
		wantErr     bool
	}{
		{"xccdf_org.ssgproject.content_profile_test_profile", "This profile is only used for Unit Tests", false},
		{"xccdf_org.ssgproject.content_profile_test_profile_no_description", "", false},
		{"xccdf_org.ssgproject.content_profile_absent", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.dsProfileID, func(t *testing.T) {
			dsProfile, err := getDsProfile(doc, tt.dsProfileID)
			if err != nil {
				t.Fatalf("failed to get profile: %v", err)
			}

			result, err := getDsElementDescription(dsProfile)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDsElementDescription() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != nil && err == nil {
				if result.InnerText() != tt.expected {
					t.Errorf("got %s, want %s", result.InnerText(), tt.expected)
				}
			}
		})
	}
}

// TestPopulateProfileInfo tests the populateProfileInfo function.
func TestPopulateProfileInfo(t *testing.T) {
	doc, _ := LoadDsTest(t, "ssg-rhel-ds.xml")
	tests := []struct {
		dsProfileID         string
		expectedTitle       string
		expectedDescription string
		wantErr             bool
	}{
		{"xccdf_org.ssgproject.content_profile_test_profile", "Test Profile", "This profile is only used for Unit Tests", false},
		{"xccdf_org.ssgproject.content_profile_test_profile_no_title", "", "This profile is only used for Unit Tests", false},
		{"xccdf_org.ssgproject.content_profile_test_profile_no_description", "Test Profile No Description", "", false},
		{"xccdf_org.ssgproject.content_profile_absent", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.dsProfileID, func(t *testing.T) {
			dsProfile, err := getDsProfile(doc, tt.dsProfileID)
			if err != nil {
				t.Fatalf("failed to get profile: %v", err)
			}

			parsedProfile := &xccdf.ProfileElement{}
			result, err := populateProfileInfo(dsProfile, parsedProfile)
			if (err != nil) != tt.wantErr {
				t.Errorf("populateProfileInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result.Title != nil && result.Title.Value != tt.expectedTitle {
				t.Errorf("got title %s, want %s", result.Title.Value, tt.expectedTitle)
			}
			if result.Description != nil && result.Description.Value != tt.expectedDescription {
				t.Errorf("got description %s, want %s", result.Description.Value, tt.expectedDescription)
			}
		})
	}
}

// TestPopulateProfileVariables tests the populateProfileVariables function.
func TestPopulateProfileVariables(t *testing.T) {
	doc, _ := LoadDsTest(t, "ssg-rhel-ds.xml")
	tests := []struct {
		dsProfileID string
		wantErr     bool
	}{
		{"xccdf_org.ssgproject.content_profile_test_profile", false},
		{"xccdf_org.ssgproject.content_profile_absent", true},
	}

	for _, tt := range tests {
		t.Run(tt.dsProfileID, func(t *testing.T) {
			dsProfile, err := getDsProfile(doc, tt.dsProfileID)
			if err != nil {
				t.Fatalf("failed to get profile: %v", err)
			}

			parsedProfile := &xccdf.ProfileElement{}
			result, err := populateProfileVariables(dsProfile, parsedProfile)
			if (err != nil) != tt.wantErr {
				t.Errorf("populateProfileVariables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != nil && result.Values == nil {
				t.Errorf("got nil values, want non-nil")
			}
		})
	}
}

// TestPopulateProfileRules tests the populateProfileVariables function.
func TestPopulateProfileRules(t *testing.T) {
	doc, _ := LoadDsTest(t, "ssg-rhel-ds.xml")
	tests := []struct {
		dsProfileID string
		wantErr     bool
	}{
		{"xccdf_org.ssgproject.content_profile_test_profile", false},
		{"xccdf_org.ssgproject.content_profile_absent", true},
	}

	for _, tt := range tests {
		t.Run(tt.dsProfileID, func(t *testing.T) {
			dsProfile, err := getDsProfile(doc, tt.dsProfileID)
			if err != nil {
				t.Fatalf("failed to get profile: %v", err)
			}

			parsedProfile := &xccdf.ProfileElement{}
			result, err := populateProfileRules(dsProfile, parsedProfile)
			if (err != nil) != tt.wantErr {
				t.Errorf("populateProfileRules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != nil && result.Selections == nil {
				t.Errorf("got nil rules, want non-nil")
			}
		})
	}
}

// TestInitProfile tests the initProfile function.
func TestInitProfile(t *testing.T) {
	doc, _ := LoadDsTest(t, "ssg-rhel-ds.xml")
	tests := []struct {
		dsProfileID         string
		expectedTitle       string
		expectedDescription string
		wantErr             bool
	}{
		{"xccdf_org.ssgproject.content_profile_test_profile", "Test Profile", "This profile is only used for Unit Tests", false},
		{"xccdf_org.ssgproject.content_profile_test_profile_no_title", "", "This profile is only used for Unit Tests", false},
		{"xccdf_org.ssgproject.content_profile_test_profile_no_description", "Test Profile No Description", "", false},
		{"xccdf_org.ssgproject.content_profile_absent", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.dsProfileID, func(t *testing.T) {
			dsProfile, err := getDsProfile(doc, tt.dsProfileID)
			if err != nil {
				t.Fatalf("failed to get profile: %v", err)
			}

			result, err := initProfile(dsProfile, tt.dsProfileID)
			if (err != nil) != tt.wantErr {
				t.Errorf("initProfile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result.ID != tt.dsProfileID {
				t.Errorf("got ID %s, want %s", result.ID, tt.dsProfileID)
			}
			if result.Title != nil && result.Title.Value != tt.expectedTitle {
				t.Errorf("got title %s, want %s", result.Title.Value, tt.expectedTitle)
			}
			if result.Description != nil && result.Description.Value != tt.expectedDescription {
				t.Errorf("got description %s, want %s", result.Description.Value, tt.expectedDescription)
			}
		})
	}
}

// TestGetDsProfile tests the GetDsProfile function.
func TestGetDsProfile(t *testing.T) {
	tests := []struct {
		profileId string
		dsPath    string
		expected  *xccdf.ProfileElement
		wantErr   bool
	}{
		{
			profileId: "test_profile",
			dsPath:    filepath.Join(testDataDir, "ssg-rhel-ds.xml"),
			expected: &xccdf.ProfileElement{
				ID: "xccdf_org.ssgproject.content_profile_test_profile",
				Title: &xccdf.TitleOrDescriptionElement{
					Override: true,
					Value:    "Test Profile",
				},
				Description: &xccdf.TitleOrDescriptionElement{
					Override: true,
					Value:    "This profile is only used for Unit Tests",
				},
			},
			wantErr: false,
		},
		{
			profileId: "absent_profile",
			dsPath:    filepath.Join(testDataDir, "ssg-rhel-ds.xml"),
			expected:  nil,
			wantErr:   true,
		},
		{
			profileId: "absent_datastream",
			dsPath:    filepath.Join(testDataDir, "absent.xml"),
			expected:  nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.profileId, func(t *testing.T) {
			result, err := GetDsProfile(tt.profileId, tt.dsPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDsProfile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != nil && tt.expected != nil {
				if result.ID != tt.expected.ID {
					t.Errorf("got ID %s, want %s", result.ID, tt.expected.ID)
				}
				if result.Title != nil && result.Title.Value != tt.expected.Title.Value {
					t.Errorf("got title %s, want %s", result.Title.Value, tt.expected.Title.Value)
				}
				if result.Description != nil && result.Description.Value != tt.expected.Description.Value {
					t.Errorf("got description %s, want %s", result.Description.Value, tt.expected.Description.Value)
				}
			} else if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGetDsVariablesValues tests the GetDsVariablesValues function.
func TestGetDsVariablesValues(t *testing.T) {
	tests := []struct {
		dsPath   string
		expected int
		wantErr  bool
	}{
		{filepath.Join(testDataDir, "ssg-rhel-ds.xml"), 71, false},
		{filepath.Join(testDataDir, "absent.xml"), 0, true},
		{filepath.Join(testDataDir, "invalid.xml"), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.dsPath, func(t *testing.T) {
			result, err := GetDsVariablesValues(tt.dsPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDsVariablesValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(result) != tt.expected {
				t.Errorf("got %d variables, want %d", len(result), tt.expected)
			}
			for _, variable := range result {
				if variable.Options == nil {
					t.Errorf("got variable without options: %v", variable)
				}
				defaultOption := false
				for _, option := range variable.Options {
					if option.Selector == "default" {
						defaultOption = true
						break
					}
				}
				if !defaultOption {
					t.Errorf("default value was is not present: %v", variable)
				}
			}
		})
	}
}

// TestGetValueFromOption tests the getValueFromOption function.
func TestGetValueFromOption(t *testing.T) {
	tests := []struct {
		variables  []DsVariables
		variableId string
		selector   string
		expected   string
		wantErr    bool
	}{
		{
			variables: []DsVariables{
				{
					ID: "var1",
					Options: []DsVariableOptions{
						{
							Selector: "selector1",
							Value:    "value1",
						},
					},
				},
				{
					ID: "var2",
					Options: []DsVariableOptions{
						{
							Selector: "selector2",
							Value:    "value2",
						},
					},
				},
			},
			variableId: "var1",
			selector:   "selector1",
			expected:   "value1",
			wantErr:    false,
		},
		{
			variables: []DsVariables{
				{
					ID: "var1",
					Options: []DsVariableOptions{
						{
							Selector: "selector1",
							Value:    "value1",
						},
					},
				},
				{
					ID: "var2",
					Options: []DsVariableOptions{
						{
							Selector: "selector2",
							Value:    "value2",
						},
					},
				},
			},
			variableId: "var2",
			selector:   "selector2",
			expected:   "value2",
			wantErr:    false,
		},
		{
			variables: []DsVariables{
				{
					ID: "var1",
					Options: []DsVariableOptions{
						{
							Selector: "selector1",
							Value:    "value1",
						},
					},
				},
			},
			variableId: "var1",
			selector:   "nonexistent",
			expected:   "",
			wantErr:    true,
		},
		{
			variables: []DsVariables{
				{
					ID: "var1",
					Options: []DsVariableOptions{
						{
							Selector: "selector1",
							Value:    "value1",
						},
					},
				},
			},
			variableId: "nonexistent",
			selector:   "selector1",
			expected:   "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.variableId+"_"+tt.selector, func(t *testing.T) {
			result, err := getValueFromOption(tt.variables, tt.variableId, tt.selector)
			if (err != nil) != tt.wantErr {
				t.Errorf("getValueFromOption() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

// compareProfileElements is a helper function for TestResolveDsVariableOptions.
// It compares two ProfileElement objects.
func compareProfileElements(a, b *xccdf.ProfileElement) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.Values) != len(b.Values) {
		return false
	}
	for i := range a.Values {
		if a.Values[i].IDRef != b.Values[i].IDRef || a.Values[i].Value != b.Values[i].Value {
			return false
		}
	}
	return true
}

// TestResolveDsVariableOptions tests the ResolveDsVariableOptions function.
func TestResolveDsVariableOptions(t *testing.T) {
	tests := []struct {
		profile   *xccdf.ProfileElement
		variables []DsVariables
		expected  *xccdf.ProfileElement
		wantErr   bool
	}{
		{
			profile: &xccdf.ProfileElement{
				Values: []xccdf.SetValueElement{
					{IDRef: "var1", Value: "selector1"},
					{IDRef: "var2", Value: "selector2"},
				},
			},
			variables: []DsVariables{
				{
					ID: "var1",
					Options: []DsVariableOptions{
						{Selector: "selector1", Value: "value1"},
					},
				},
				{
					ID: "var2",
					Options: []DsVariableOptions{
						{Selector: "selector2", Value: "value2"},
					},
				},
			},
			expected: &xccdf.ProfileElement{
				Values: []xccdf.SetValueElement{
					{IDRef: "var1", Value: "value1"},
					{IDRef: "var2", Value: "value2"},
				},
			},
			wantErr: false,
		},
		{
			profile: &xccdf.ProfileElement{
				Values: []xccdf.SetValueElement{
					{IDRef: "var1", Value: "selector1"},
					{IDRef: "var3", Value: "selector3"},
				},
			},
			variables: []DsVariables{
				{
					ID: "var1",
					Options: []DsVariableOptions{
						{Selector: "selector1", Value: "value1"},
					},
				},
				{
					ID: "var2",
					Options: []DsVariableOptions{
						{Selector: "selector2", Value: "value2"},
					},
				},
			},
			expected: &xccdf.ProfileElement{
				Values: []xccdf.SetValueElement{
					{IDRef: "var1", Value: "value1"},
					{IDRef: "var3", Value: "selector3"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.profile.ID, func(t *testing.T) {
			result, err := ResolveDsVariableOptions(tt.profile, tt.variables)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveDsVariableOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !compareProfileElements(result, tt.expected) {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGetDsRules tests the GetDsRules function.
func TestGetDsRules(t *testing.T) {
	tests := []struct {
		dsPath   string
		expected int
		wantErr  bool
	}{
		{filepath.Join(testDataDir, "ssg-rhel-ds.xml"), 376, false},
		{filepath.Join(testDataDir, "absent.xml"), 0, true},
		{filepath.Join(testDataDir, "invalid.xml"), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.dsPath, func(t *testing.T) {
			result, err := GetDsRules(tt.dsPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDsRules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(result) != tt.expected {
				t.Errorf("got %d rules, want %d", len(result), tt.expected)
			}
			if !tt.wantErr {
				for _, rule := range result {
					if rule.ID == "" {
						t.Errorf("got rule with empty ID: %v", rule)
					}
					if rule.Title == "" {
						t.Errorf("got rule with empty title: %v", rule)
					}
					if rule.Description == "" {
						t.Errorf("got rule with empty description: %v", rule)
					}
				}
			}
		})
	}
}
