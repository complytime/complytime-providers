// SPDX-License-Identifier: Apache-2.0

package xccdf

import (
	"fmt"
	"os"
	"strconv"

	"github.com/antchfx/xmlquery"

	xccdf "github.com/complytime/complytime-providers/cmd/openscap-provider/xccdftype"
)

const (
	profileIDPrefix string = "xccdf_org.ssgproject.content_profile_"
	ruleIDPrefix    string = "xccdf_org.ssgproject.content_rule_"
	varIDPrefix     string = "xccdf_org.ssgproject.content_value_"
)

// The following structs can later be proposed to compliance-operator/pkg/xccdf
type DsVariableOptions struct {
	Selector string `xml:"selector,attr"`
	Value    string `xml:",chardata"`
}

type DsVariables struct {
	ID          string `xml:"idref,attr"`
	Title       string `xml:",chardata"`
	Description string `xml:",chardata"`
	Options     []DsVariableOptions
}

type DsRules struct {
	ID          string `xml:"idref,attr"`
	Title       string `xml:",chardata"`
	Description string `xml:",chardata"`
	Selected    bool   `xml:"selected,attr"`
}

func loadDataStream(dsPath string) (*xmlquery.Node, error) {
	file, err := os.Open(dsPath)
	if err != nil {
		return nil, fmt.Errorf("error opening datastream file: %w", err)
	}
	defer file.Close()

	dsDom, err := xmlquery.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("error parsing datastream file: %w", err)
	}

	return dsDom, nil
}

func getDsProfileID(profileId string) string {
	return profileIDPrefix + profileId
}

func getDsRuleID(ruleId string) string {
	return ruleIDPrefix + ruleId
}

func getDsVarID(varId string) string {
	return varIDPrefix + varId
}

func getDsElement(dsDom *xmlquery.Node, dsElement string) (*xmlquery.Node, error) {
	if dsDom == nil {
		return nil, fmt.Errorf("dsDom is nil")
	}
	// NOTE: If the element is not found in dsDom, returns nil and not an error
	return xmlquery.Query(dsDom, dsElement)
}

func getDsElementAttrValue(dsElement *xmlquery.Node, attrName string) (string, error) {
	for _, attr := range dsElement.Attr {
		if attr.Name.Local == attrName {
			return dsElement.SelectAttr(attrName), nil
		}
	}
	return "", fmt.Errorf("attribute not found: %s", attrName)
}

func getDsOptionalAttrValue(dsElement *xmlquery.Node, optionalAttrName string) string {
	attrValue, err := getDsElementAttrValue(dsElement, optionalAttrName)
	if err != nil {
		return ""
	}
	return attrValue
}

func getDsElements(dsDom *xmlquery.Node, dsElement string) ([]*xmlquery.Node, error) {
	if dsDom == nil {
		return nil, fmt.Errorf("dsDom is nil")
	}
	return xmlquery.QueryAll(dsDom, dsElement)
}

func getDsProfile(dsDom *xmlquery.Node, dsProfileID string) (*xmlquery.Node, error) {
	return getDsElement(dsDom, fmt.Sprintf("//xccdf-1.2:Profile[@id='%s']", dsProfileID))
}

func getDsElementTitle(dsProfile *xmlquery.Node) (*xmlquery.Node, error) {
	profileTitle, err := getDsElement(dsProfile, "xccdf-1.2:title")
	if err != nil {
		return nil, fmt.Errorf("error finding 'title' element: %w", err)
	}
	return profileTitle, nil
}

func getDsElementDescription(dsProfile *xmlquery.Node) (*xmlquery.Node, error) {
	profileDescription, err := getDsElement(dsProfile, "xccdf-1.2:description")
	if err != nil {
		return nil, fmt.Errorf("error finding 'description' element: %w", err)
	}
	return profileDescription, nil
}

func populateProfileInfo(dsProfile *xmlquery.Node, parsedProfile *xccdf.ProfileElement) (*xccdf.ProfileElement, error) {
	profileTitle, err := getDsElementTitle(dsProfile)
	if err != nil {
		return parsedProfile, fmt.Errorf("error populating profile title: %w", err)
	}
	if parsedProfile.Title == nil {
		parsedProfile.Title = &xccdf.TitleOrDescriptionElement{}
	}
	if profileTitle == nil {
		// log that profile title was not found.
		// It is a valid case but may not be expected so better to log it.
		parsedProfile.Title.Override = false
		parsedProfile.Title.Value = ""
	} else {
		parsedProfile.Title.Override = true
		parsedProfile.Title.Value = profileTitle.InnerText()
	}

	profileDescription, err := getDsElementDescription(dsProfile)
	if err != nil {
		return parsedProfile, fmt.Errorf("error populating profile description: %w", err)
	}
	if parsedProfile.Description == nil {
		parsedProfile.Description = &xccdf.TitleOrDescriptionElement{}
	}
	if profileDescription == nil {
		// log that profile description was not found.
		// It is a valid case but may not be expected so better to log it.
		parsedProfile.Description.Override = false
		parsedProfile.Description.Value = ""
	} else {
		parsedProfile.Description.Override = true
		parsedProfile.Description.Value = profileDescription.InnerText()
	}

	return parsedProfile, nil
}

func populateProfileVariables(dsProfile *xmlquery.Node, parsedProfile *xccdf.ProfileElement) (*xccdf.ProfileElement, error) {
	if parsedProfile.Values == nil {
		parsedProfile.Values = []xccdf.SetValueElement{}
	}

	profileVariables, err := getDsElements(dsProfile, "xccdf-1.2:refine-value")
	if err != nil {
		return parsedProfile, fmt.Errorf("error finding 'refine-value' elements in profile: %w", err)
	}

	for _, variable := range profileVariables {
		varIdRef, err := getDsElementAttrValue(variable, "idref")
		if err != nil {
			return parsedProfile, fmt.Errorf("error getting value of 'idref' attribute: %w", err)
		}
		varSelector, err := getDsElementAttrValue(variable, "selector")
		if err != nil {
			return parsedProfile, fmt.Errorf("error getting value of 'selector' attribute: %w", err)
		}

		parsedProfile.Values = append(parsedProfile.Values, xccdf.SetValueElement{
			IDRef: varIdRef,
			Value: varSelector,
		})
	}
	return parsedProfile, nil
}

func populateProfileRules(dsProfile *xmlquery.Node, parsedProfile *xccdf.ProfileElement) (*xccdf.ProfileElement, error) {
	if parsedProfile.Selections == nil {
		parsedProfile.Selections = []xccdf.SelectElement{}
	}

	profileRules, err := getDsElements(dsProfile, "xccdf-1.2:select")
	if err != nil {
		return parsedProfile, fmt.Errorf("error finding 'select' elements in profile: %w", err)
	}

	for _, rule := range profileRules {
		ruleIdRef, err := getDsElementAttrValue(rule, "idref")
		if err != nil {
			return parsedProfile, fmt.Errorf("error getting value of 'idref' attribute: %w", err)
		}
		ruleSelected, err := getDsElementAttrValue(rule, "selected")
		if err != nil {
			return nil, fmt.Errorf("error getting value of 'selected' attribute: %w", err)
		}
		selectedBoolean, err := strconv.ParseBool(ruleSelected)
		if err != nil {
			return nil, fmt.Errorf("error converting the 'selected' attribute from string to boolean: %w", err)
		}

		parsedProfile.Selections = append(parsedProfile.Selections, xccdf.SelectElement{
			IDRef:    ruleIdRef,
			Selected: selectedBoolean,
		})
	}
	return parsedProfile, nil
}

func initProfile(dsProfile *xmlquery.Node, dsProfileId string) (*xccdf.ProfileElement, error) {
	parsedProfile := new(xccdf.ProfileElement)
	parsedProfile.ID = dsProfileId

	parsedProfile, err := populateProfileInfo(dsProfile, parsedProfile)
	if err != nil {
		return parsedProfile, fmt.Errorf("error populating profile title and description: %w", err)
	}

	parsedProfile, err = populateProfileRules(dsProfile, parsedProfile)
	if err != nil {
		return parsedProfile, fmt.Errorf("error populating profile rules: %w", err)
	}

	parsedProfile, err = populateProfileVariables(dsProfile, parsedProfile)
	if err != nil {
		return parsedProfile, fmt.Errorf("error populating profile variables: %w", err)
	}

	return parsedProfile, nil
}

func GetDsProfile(profileId string, dsPath string) (*xccdf.ProfileElement, error) {
	dsDom, err := loadDataStream(dsPath)
	if err != nil {
		return nil, fmt.Errorf("error loading datastream: %w", err)
	}

	dsProfileID := getDsProfileID(profileId)
	dsProfile, err := getDsProfile(dsDom, dsProfileID)
	if err != nil {
		return nil, fmt.Errorf("error processing profile %s in datastream: %w", profileId, err)
	}

	if dsProfile == nil {
		return nil, fmt.Errorf("profile not found: %s", dsProfileID)
	}

	parsedProfile, err := initProfile(dsProfile, dsProfileID)
	if err != nil {
		return nil, fmt.Errorf("error initializing a parsed profile for %s: %w", profileId, err)
	}

	return parsedProfile, nil
}

func GetDsVariablesValues(dsPath string) ([]DsVariables, error) {
	dsDom, err := loadDataStream(dsPath)
	if err != nil {
		return nil, fmt.Errorf("error loading datastream: %w", err)
	}

	dsVariables, err := getDsElements(dsDom, "//xccdf-1.2:Value")
	if err != nil {
		return nil, fmt.Errorf("error getting variables from datastream: %w", err)
	}

	dsVariablesValues := []DsVariables{}
	for _, variable := range dsVariables {
		varId, err := getDsElementAttrValue(variable, "id")
		if err != nil {
			return nil, fmt.Errorf("error getting value of 'id' attribute: %w", err)
		}

		varTitle, err := getDsElementTitle(variable)
		if err != nil {
			return nil, fmt.Errorf("error getting variable title: %w", err)
		}

		varDescription, err := getDsElementDescription(variable)
		if err != nil {
			return nil, fmt.Errorf("error getting variable description: %w", err)
		}

		varOptions, err := getDsElements(variable, "xccdf-1.2:value")
		if err != nil {
			return nil, fmt.Errorf("error getting variable options: %w", err)
		}

		dsVarOptions := []DsVariableOptions{}
		for _, option := range varOptions {
			selectorId := getDsOptionalAttrValue(option, "selector")
			if selectorId == "" {
				selectorId = "default"
			}
			selectorValue := option.InnerText()
			dsVarOptions = append(dsVarOptions, DsVariableOptions{
				Selector: selectorId,
				Value:    selectorValue,
			})
		}

		dsVariablesValues = append(dsVariablesValues, DsVariables{
			ID:          varId,
			Title:       varTitle.InnerText(),
			Description: varDescription.InnerText(),
			Options:     dsVarOptions,
		})
	}
	return dsVariablesValues, nil
}

func getValueFromOption(variables []DsVariables, variableId string, selector string) (string, error) {
	for _, variable := range variables {
		if variable.ID == variableId {
			for _, option := range variable.Options {
				if option.Selector == selector {
					return option.Value, nil
				}
			}
		}
	}
	return "", fmt.Errorf("variable not found: %s", variableId)
}

func ResolveDsVariableOptions(profile *xccdf.ProfileElement, variables []DsVariables) (*xccdf.ProfileElement, error) {
	for i, value := range profile.Values {
		resolvedValue, err := getValueFromOption(variables, value.IDRef, value.Value)
		if err != nil {
			return profile, fmt.Errorf("error resolving variable options: %w", err)
		}
		profile.Values[i].Value = resolvedValue
	}
	return profile, nil
}

func GetDsRules(dsPath string) ([]DsRules, error) {
	dsDom, err := loadDataStream(dsPath)
	if err != nil {
		return nil, fmt.Errorf("error loading datastream: %w", err)
	}

	dsRules, err := getDsElements(dsDom, "//xccdf-1.2:Rule")
	if err != nil {
		return nil, fmt.Errorf("error getting rules from datastream: %w", err)
	}

	dsRulesInfo := []DsRules{}
	for _, rule := range dsRules {
		ruleId, err := getDsElementAttrValue(rule, "id")
		if err != nil {
			return nil, fmt.Errorf("error getting value of 'id' attribute: %w", err)
		}

		ruleTitle, err := getDsElementTitle(rule)
		if err != nil {
			return nil, fmt.Errorf("error getting rule title: %w", err)
		}

		ruleDescription, err := getDsElementDescription(rule)
		if err != nil {
			return nil, fmt.Errorf("error getting rule description: %w", err)
		}

		dsRuleSelected, err := getDsElementAttrValue(rule, "selected")
		if err != nil {
			return nil, fmt.Errorf("error getting value of 'selected' attribute: %w", err)
		}
		selectedBoolean, err := strconv.ParseBool(dsRuleSelected)
		if err != nil {
			return nil, fmt.Errorf("error converting 'selected' attribute from string to boolean: %w", err)
		}

		dsRulesInfo = append(dsRulesInfo, DsRules{
			ID:          ruleId,
			Title:       ruleTitle.InnerText(),
			Description: ruleDescription.InnerText(),
			Selected:    selectedBoolean,
		})
	}
	return dsRulesInfo, nil
}

// Getting rule information
// Copied from https://github.com/ComplianceAsCode/compliance-operator/blob/fed54b4b761374578016d79d97bcb7636bf9d920/pkg/utils/parse_arf_result.go#L170
func NewRuleHashTable(dsDom *xmlquery.Node) NodeByIdHashTable {
	return newHashTableFromRootAndQuery(dsDom, "//ds:component/xccdf-1.2:Benchmark", "//xccdf-1.2:Rule")
}

func newHashTableFromRootAndQuery(dsDom *xmlquery.Node, root, query string) NodeByIdHashTable {
	benchmarkDom := dsDom.SelectElement(root)
	rules := benchmarkDom.SelectElements(query)
	return newByIdHashTable(rules)
}

type NodeByIdHashTable map[string]*xmlquery.Node

//type nodeByIdHashVariablesTable map[string][]string

func newByIdHashTable(nodes []*xmlquery.Node) NodeByIdHashTable {
	table := make(NodeByIdHashTable)
	for i := range nodes {
		ruleDefinition := nodes[i]
		ruleId := ruleDefinition.SelectAttr("id")

		table[ruleId] = ruleDefinition
	}

	return table
}
