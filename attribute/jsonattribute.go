package attribute

import (
	"encoding/json"
	"fmt"

	"github.com/getyoti/yoti-go-sdk/anchor"
	"github.com/getyoti/yoti-go-sdk/yotiprotoattr_v3"
)

//JSONAttribute is a Yoti attribute which returns an interface as its value
type JSONAttribute struct {
	*yotiprotoattr_v3.Attribute // Value returns the value of a JSON attribute in the form of an interface
	value                       interface{}
	anchors                     []*anchor.Anchor
	sources                     []*anchor.Anchor
	verifiers                   []*anchor.Anchor
}

//NewJSON creates a new JSON attribute
func NewJSON(a *yotiprotoattr_v3.Attribute) (*JSONAttribute, error) {
	interfaceValue, err := UnmarshallJSON(a.Value)
	if err != nil {
		err = fmt.Errorf("Unable to parse JSON value: %q. Error: %q", a.Value, err)
		return nil, err
	}

	parsedAnchors := anchor.ParseAnchors(a.Anchors)

	return &JSONAttribute{
		Attribute: &yotiprotoattr_v3.Attribute{
			Name:        a.Name,
			ContentType: a.ContentType,
		},
		value:     interfaceValue,
		anchors:   parsedAnchors,
		sources:   anchor.GetSources(parsedAnchors),
		verifiers: anchor.GetVerifiers(parsedAnchors),
	}, nil
}

// UnmarshallJSON unmarshalls JSON into an interface
func UnmarshallJSON(byteValue []byte) (result interface{}, err error) {
	var unmarshalledJSON interface{}
	err = json.Unmarshal(byteValue, &unmarshalledJSON)

	if err != nil {
		return nil, err
	}

	return unmarshalledJSON, err
}

// Value returns the value of the JSONAttribute as an interface
func (a *JSONAttribute) Value() interface{} {
	return a.value
}

// Anchors are the metadata associated with an attribute. They describe
// how an attribute has been provided to Yoti (SOURCE Anchor) and how
// it has been verified (VERIFIER Anchor).
func (a *JSONAttribute) Anchors() []*anchor.Anchor {
	return a.anchors
}

// Sources returns the anchors which identify how and when an attribute value was acquired.
func (a *JSONAttribute) Sources() []*anchor.Anchor {
	return a.sources
}

// Verifiers returns the anchors which identify how and when an attribute value was verified by another provider.
func (a *JSONAttribute) Verifiers() []*anchor.Anchor {
	return a.verifiers
}