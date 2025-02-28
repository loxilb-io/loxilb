/*
 * Copyright (c) 2025 LoxiLB Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package handler

import (
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/restapi/operations/metadata"

	tk "github.com/loxilb-io/loxilib"
)

// Define SwaggerDoc struct
type SwaggerDoc struct {
	Paths       map[string]map[string]Operation `yaml:"paths"`
	Definitions map[string]interface{}          `yaml:"definitions"`
}

// Define Operation struct
type Operation struct {
	Parameters []Parameter `yaml:"parameters"`
}

// Define Parameter struct
type Parameter struct {
	Name     string                 `yaml:"name"`
	In       string                 `yaml:"in"`
	Required bool                   `yaml:"required"`
	Type     string                 `yaml:"type"`
	Schema   map[string]interface{} `yaml:"schema"`
}

// toStringMap: map[interface{}]interface{} -> map[string]interface{}
func toStringMap(in map[interface{}]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range in {
		if key, ok := k.(string); ok {
			if subMap, ok := v.(map[interface{}]interface{}); ok {
				out[key] = toStringMap(subMap)
			} else {
				out[key] = v
			}
		}
	}
	return out
}

// LoadSwaggerDoc is used to load a Swagger document from a file.
func LoadSwaggerDoc(filePath string) (*SwaggerDoc, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var doc SwaggerDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	// Convert map[interface{}]interface{} to map[string]interface{}
	if doc.Definitions != nil {
		for key, def := range doc.Definitions {
			if defMap, ok := def.(map[interface{}]interface{}); ok {
				doc.Definitions[key] = toStringMap(defMap)
			}
		}
	}

	return &doc, nil
}

// AutoGenerateMetaData is used to automatically generate metadata as a json format from a Swagger document.
func AutoGenerateMetaData(swaggerPath string) (map[string]interface{}, error) {
	doc, err := LoadSwaggerDoc(swaggerPath)
	if err != nil {
		return nil, err
	}

	meta := extractMetaData(doc)
	return meta, nil
}

// ConfigGetMetadata is used to get metadata from the Swagger document.
func ConfigGetMetadata(params metadata.GetMetaParams, principal interface{}) middleware.Responder {
	tk.LogIt(tk.LogTrace, "[API] Metadata %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	swaggerPath := "api/swagger.yml"
	jsonMeta, err := AutoGenerateMetaData(swaggerPath)
	if err != nil {
		tk.LogIt(tk.LogError, "메타데이터 생성 실패: %v\n", err)
	}
	return metadata.NewGetMetaOK().WithPayload(jsonMeta)
}

// extractMetaData is used to extract metadata from a Swagger document.
// Post or Put methods are processed.
func extractMetaData(doc *SwaggerDoc) map[string]interface{} {
	meta := make(map[string]interface{})
	for path, methods := range doc.Paths {
		for method, op := range methods {
			if strings.ToLower(method) == "post" || strings.ToLower(method) == "put" {

				fields := make(map[string]interface{})
				for _, param := range op.Parameters {
					if param.Type != "" {
						if param.Name == "attr" || param.Name == "user" {
							continue
						} else {
							fields[param.Name] = map[string]interface{}{
								"type":     param.Type,
								"required": param.Required,
							}
						}
						continue
					}
					if param.Schema != nil {
						var schema map[string]interface{}
						if ref, ok := param.Schema["$ref"].(string); ok {
							key := strings.TrimPrefix(ref, "#/definitions/")
							if defRaw, found := doc.Definitions[key]; found {
								if defMap, ok := defRaw.(map[string]interface{}); ok {
									schema = defMap
								}
							}
						} else {
							schema = param.Schema
						}
						if schema != nil {
							metaEntry := processSchema(schema, doc.Definitions, param.Required)
							if param.Name == "attr" || param.Name == "user" {
								if merged, ok := metaEntry.(map[string]interface{}); ok {
									for k, v := range merged {
										fields[k] = v
									}
								}
							} else {
								fields[param.Name] = metaEntry
							}
							continue
						}
						fields[param.Name] = map[string]interface{}{
							"type":     "object",
							"required": param.Required,
						}
						continue
					}
					fields[param.Name] = map[string]interface{}{
						"type":     "unknown",
						"required": param.Required,
					}
				}
				meta[path] = map[string]interface{}{
					"method": strings.ToUpper(method),
					"fields": fields,
				}
			}
		}
	}
	return meta
}

// processSchema is used to process the schema.
func processSchema(schema map[string]interface{}, defs map[string]interface{}, required bool) interface{} {
	if ref, ok := schema["$ref"].(string); ok {
		key := strings.TrimPrefix(ref, "#/definitions/")
		if defRaw, found := defs[key]; found {
			if defMap, ok := defRaw.(map[string]interface{}); ok {
				schema = defMap
			}
		}
	}

	typ, _ := schema["type"].(string)
	formatVal, _ := schema["format"].(string)
	desc, _ := schema["description"].(string)

	switch typ {
	case "object":
		if propsRaw, ok := schema["properties"]; ok {
			props, ok := propsRaw.(map[string]interface{})
			if !ok {
				break
			}
			out := make(map[string]interface{})
			reqSet := map[string]bool{}
			if reqArr, ok := schema["required"].([]interface{}); ok {
				for _, r := range reqArr {
					if rStr, ok := r.(string); ok {
						reqSet[rStr] = true
					}
				}
			}
			if attrRaw, exists := props["attr"]; exists {
				if attrSchema, ok := attrRaw.(map[string]interface{}); ok {
					merged := processSchema(attrSchema, defs, reqSet["attr"])
					if mergedMap, ok := merged.(map[string]interface{}); ok {
						for k, v := range mergedMap {
							out[k] = v
						}
					}
				}
			}
			for propName, propRaw := range props {
				if propName == "attr" {
					continue
				}
				if propSchema, ok := propRaw.(map[string]interface{}); ok {
					out[propName] = processSchema(propSchema, defs, reqSet[propName])
				}
			}
			if formatVal != "" {
				out["format"] = formatVal
			}
			if desc != "" {
				out["description"] = desc
			}
			if enumVal, ok := schema["enum"]; ok {
				out["enum"] = enumVal
			}
			return out
		}
		defOut := map[string]interface{}{
			"type":     typ,
			"required": required,
		}
		if formatVal != "" {
			defOut["format"] = formatVal
		}
		if desc != "" {
			defOut["description"] = desc
		}
		if enumVal, ok := schema["enum"]; ok {
			defOut["enum"] = enumVal
		}
		return defOut
	case "array":
		var itemResult interface{}
		if itemsRaw, ok := schema["items"]; ok {
			if itemsMap, ok := itemsRaw.(map[string]interface{}); ok {
				if t, ok := itemsMap["type"].(string); !ok || t == "" {
					if _, exists := itemsMap["properties"]; exists {
						itemsMap["type"] = "object"
					} else {
						itemsMap["type"] = "unknown"
					}
				}
				itemResult = processSchema(itemsMap, defs, false)
			} else {
				itemResult = map[string]interface{}{"type": "unknown"}
			}
		} else {
			itemResult = map[string]interface{}{"type": "unknown"}
		}
		arrOut := map[string]interface{}{
			"type":     "array",
			"required": required,
			"items":    itemResult,
		}
		if formatVal != "" {
			arrOut["format"] = formatVal
		}
		if desc != "" {
			arrOut["description"] = desc
		}
		if enumVal, ok := schema["enum"]; ok {
			arrOut["enum"] = enumVal
		}
		return arrOut
	default:
		defCaseOut := map[string]interface{}{
			"type":     typ,
			"required": required,
		}
		if formatVal != "" {
			defCaseOut["format"] = formatVal
		}
		if desc != "" {
			defCaseOut["description"] = desc
		}
		if enumVal, ok := schema["enum"]; ok {
			defCaseOut["enum"] = enumVal
		}
		return defCaseOut
	}
	// fallback
	return map[string]interface{}{
		"type":     "unknown",
		"required": required,
	}
}
