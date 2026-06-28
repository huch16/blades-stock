package types

import (
	"context"

	"github.com/go-kratos/blades/tools"
	"github.com/google/jsonschema-go/jsonschema"
)

type ParamDef struct {
	Type      string
	Desc      string
	Required  bool
	Enum      []string
	ElemInfo  *ParamDef
	SubParams map[string]*ParamDef
}

func paramToSchema(name string, p *ParamDef) *jsonschema.Schema {
	if p == nil {
		return nil
	}
	s := &jsonschema.Schema{}
	switch p.Type {
	case "string":
		s.Type = "string"
		if len(p.Enum) > 0 {
			s.Enum = make([]any, len(p.Enum))
			for i, e := range p.Enum {
				s.Enum[i] = e
			}
		}
	case "integer", "number":
		s.Type = "number"
	case "boolean":
		s.Type = "boolean"
	case "array":
		s.Type = "array"
		if p.ElemInfo != nil {
			s.Items = paramToSchema(name, p.ElemInfo)
		}
	case "object":
		s.Type = "object"
		if len(p.SubParams) > 0 {
			props := make(map[string]*jsonschema.Schema)
			reqs := []string{}
			for k, v := range p.SubParams {
				props[k] = paramToSchema(k, v)
				if v.Required {
					reqs = append(reqs, k)
				}
			}
			s.Properties = props
			s.Required = reqs
		}
	default:
		s.Type = "string"
	}
	if p.Desc != "" {
		s.Description = p.Desc
	}
	return s
}

func BuildInputSchema(params map[string]*ParamDef) *jsonschema.Schema {
	if len(params) == 0 {
		return nil
	}
	props := make(map[string]*jsonschema.Schema)
	reqs := []string{}
	for k, p := range params {
		props[k] = paramToSchema(k, p)
		if p.Required {
			reqs = append(reqs, k)
		}
	}
	return &jsonschema.Schema{
		Type:       "object",
		Properties: props,
		Required:   reqs,
	}
}

type ToolFunc func(args string) (string, error)

func NewTool(name, description string, params map[string]*ParamDef, handler ToolFunc) tools.Tool {
	inputSchema := BuildInputSchema(params)
	return &bladesTool{
		name:        name,
		description: description,
		inputSchema: inputSchema,
		handler:     handler,
	}
}

type bladesTool struct {
	name        string
	description string
	inputSchema *jsonschema.Schema
	handler     ToolFunc
}

func (t *bladesTool) Name() string                     { return t.name }
func (t *bladesTool) Description() string              { return t.description }
func (t *bladesTool) InputSchema() *jsonschema.Schema  { return t.inputSchema }
func (t *bladesTool) OutputSchema() *jsonschema.Schema { return nil }
func (t *bladesTool) Handle(_ context.Context, input string) (string, error) {
	return t.handler(input)
}
