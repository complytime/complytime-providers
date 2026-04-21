// SPDX-License-Identifier: Apache-2.0

// Package xccdftype provides XCCDF XML types for tailoring file generation.
// These types replace the upstream compliance-operator/pkg/xccdf dependency,
// preserving the same struct layout and XML tags so existing marshalling logic
// continues to work unchanged.
package xccdftype

import "encoding/xml"

const (
	XMLHeader string = `<?xml version="1.0" encoding="UTF-8"?>`
	XCCDFURI  string = "http://checklists.nist.gov/xccdf/1.2"
)

type TailoringElement struct {
	XMLName         xml.Name `xml:"xccdf-1.2:Tailoring"`
	XMLNamespaceURI string   `xml:"xmlns:xccdf-1.2,attr"`
	ID              string   `xml:"id,attr"`
	Benchmark       BenchmarkElement
	Version         VersionElement
	Profile         ProfileElement
}

type BenchmarkElement struct {
	XMLName xml.Name `xml:"xccdf-1.2:benchmark"`
	Href    string   `xml:"href,attr"`
}

type VersionElement struct {
	XMLName xml.Name `xml:"xccdf-1.2:version"`
	Time    string   `xml:"time,attr"`
	Value   string   `xml:",chardata"`
}

type ProfileElement struct {
	XMLName     xml.Name                   `xml:"xccdf-1.2:Profile"`
	ID          string                     `xml:"id,attr"`
	Extends     string                     `xml:"extends,attr,omitempty"`
	Title       *TitleOrDescriptionElement `xml:"xccdf-1.2:title"`
	Description *TitleOrDescriptionElement `xml:"xccdf-1.2:description"`
	Selections  []SelectElement
	Values      []SetValueElement
}

type TitleOrDescriptionElement struct {
	Override bool   `xml:"override,attr"`
	Value    string `xml:",chardata"`
}

type SelectElement struct {
	XMLName  xml.Name `xml:"xccdf-1.2:select"`
	IDRef    string   `xml:"idref,attr"`
	Selected bool     `xml:"selected,attr"`
}

type SetValueElement struct {
	XMLName xml.Name `xml:"xccdf-1.2:set-value"`
	IDRef   string   `xml:"idref,attr"`
	Value   string   `xml:",chardata"`
}
