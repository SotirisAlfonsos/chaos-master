// GENERATED BY THE COMMAND ABOVE; DO NOT EDIT
// This file was generated by swaggo/swag

package docs

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/alecthomas/template"
	"github.com/swaggo/swag"
)

var doc = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{.Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/docker": {
            "post": {
                "description": "Perform start or stop action on a container. If random is specified you do not have to provide a target",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Failure injections"
                ],
                "summary": "Inject docker failures",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Specify to perform action for container on random target",
                        "name": "do",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "description": "Specify to perform a start or a stop on the specified container",
                        "name": "action",
                        "in": "query",
                        "required": true
                    },
                    {
                        "description": "Specify the job name, container name and target",
                        "name": "requestPayload",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/docker.RequestPayload"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/docker.ResponsePayload"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/docker.ResponsePayload"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/docker.ResponsePayload"
                        }
                    }
                }
            }
        },
        "/master/status": {
            "get": {
                "description": "Bot status",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Status"
                ],
                "summary": "get master status",
                "responses": {
                    "200": {
                        "description": "Bots status",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "500": {
                        "description": "ok",
                        "schema": {
                            "type": "string"
                        }
                    }
                }
            }
        },
        "/recover/alertmanager": {
            "post": {
                "description": "Alertmanager webhook to recover from failures",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Recover"
                ],
                "summary": "recover from failures",
                "parameters": [
                    {
                        "description": "Create request payload that contains the recovery details",
                        "name": "requestPayload",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/v1.requestPayload"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/v1.RecoverResponsePayload"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/v1.RecoverResponsePayload"
                        }
                    }
                }
            }
        },
        "/service": {
            "post": {
                "description": "Perform start or stop action on a service",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Failure injections"
                ],
                "summary": "Inject service failures",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Specify to perform a start or a stop on the specified service",
                        "name": "action",
                        "in": "query",
                        "required": true
                    },
                    {
                        "description": "Specify the job name, service name and target",
                        "name": "requestPayload",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/service.RequestPayload"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/service.ResponsePayload"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/service.ResponsePayload"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/service.ResponsePayload"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "docker.RequestPayload": {
            "type": "object",
            "properties": {
                "containerName": {
                    "type": "string"
                },
                "job": {
                    "type": "string"
                },
                "target": {
                    "type": "string"
                }
            }
        },
        "docker.ResponsePayload": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "string"
                },
                "message": {
                    "type": "string"
                },
                "status": {
                    "type": "integer"
                }
            }
        },
        "service.RequestPayload": {
            "type": "object",
            "properties": {
                "job": {
                    "type": "string"
                },
                "serviceName": {
                    "type": "string"
                },
                "target": {
                    "type": "string"
                }
            }
        },
        "service.ResponsePayload": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "string"
                },
                "message": {
                    "type": "string"
                },
                "status": {
                    "type": "integer"
                }
            }
        },
        "v1.Alert": {
            "type": "object",
            "properties": {
                "labels": {
                    "$ref": "#/definitions/v1.Labels"
                },
                "status": {
                    "type": "string"
                }
            }
        },
        "v1.Labels": {
            "type": "object",
            "properties": {
                "recoverAll": {
                    "type": "boolean"
                },
                "recoverJob": {
                    "type": "string"
                },
                "recoverTarget": {
                    "type": "string"
                }
            }
        },
        "v1.RecoverMessage": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "string"
                },
                "message": {
                    "type": "string"
                },
                "status": {
                    "type": "string"
                }
            }
        },
        "v1.RecoverResponsePayload": {
            "type": "object",
            "properties": {
                "recoverMessages": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/v1.RecoverMessage"
                    }
                },
                "status": {
                    "type": "integer"
                }
            }
        },
        "v1.requestPayload": {
            "type": "object",
            "properties": {
                "alerts": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/v1.Alert"
                    }
                }
            }
        }
    }
}`

type swaggerInfo struct {
	Version     string
	Host        string
	BasePath    string
	Schemes     []string
	Title       string
	Description string
}

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = swaggerInfo{
	Version:     "1.0",
	Host:        "localhost:8090",
	BasePath:    "/chaos/api/v1",
	Schemes:     []string{},
	Title:       "Chaos Master API",
	Description: "This is the chaos master API.",
}

type s struct{}

func (s *s) ReadDoc() string {
	sInfo := SwaggerInfo
	sInfo.Description = strings.Replace(sInfo.Description, "\n", "\\n", -1)

	t, err := template.New("swagger_info").Funcs(template.FuncMap{
		"marshal": func(v interface{}) string {
			a, _ := json.Marshal(v)
			return string(a)
		},
	}).Parse(doc)
	if err != nil {
		return doc
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, sInfo); err != nil {
		return doc
	}

	return tpl.String()
}

func init() {
	swag.Register(swag.Name, &s{})
}
