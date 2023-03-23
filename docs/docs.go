// Code generated by swaggo/swag. DO NOT EDIT.

package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/articles": {
            "get": {
                "description": "分页展示所有文章",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "articles"
                ],
                "summary": "展示所有文章",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/model.Article"
                            }
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "model.Article": {
            "type": "object",
            "properties": {
                "author": {
                    "$ref": "#/definitions/model.User"
                },
                "author_id": {
                    "type": "integer"
                },
                "category": {
                    "$ref": "#/definitions/model.Category"
                },
                "category_uid": {
                    "type": "string"
                },
                "comment_count": {
                    "type": "integer"
                },
                "created_at": {
                    "type": "string"
                },
                "detail": {
                    "$ref": "#/definitions/model.ArticleDetail"
                },
                "detail_uid": {
                    "type": "string"
                },
                "id": {
                    "type": "integer"
                },
                "is_allow_comment": {
                    "type": "boolean"
                },
                "like_count": {
                    "type": "integer"
                },
                "post_time": {
                    "type": "string"
                },
                "read_count": {
                    "type": "integer"
                },
                "status": {
                    "description": "1:公开 2: 置顶 3: 私密 4: 草稿 5: 等待复审 6: 回收站",
                    "type": "integer"
                },
                "summary": {
                    "type": "string"
                },
                "tags": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/model.Tag"
                    }
                },
                "title": {
                    "type": "string"
                },
                "uid": {
                    "type": "string"
                },
                "updated_at": {
                    "type": "string"
                }
            }
        },
        "model.ArticleDetail": {
            "type": "object",
            "properties": {
                "content": {
                    "type": "string"
                },
                "id": {
                    "type": "integer"
                },
                "uid": {
                    "type": "string"
                }
            }
        },
        "model.Category": {
            "type": "object",
            "properties": {
                "count": {
                    "type": "integer"
                },
                "created_at": {
                    "type": "string"
                },
                "father_uid": {
                    "type": "string"
                },
                "id": {
                    "type": "integer"
                },
                "name": {
                    "type": "string"
                },
                "type": {
                    "$ref": "#/definitions/model.CategoryType"
                },
                "uid": {
                    "type": "string"
                },
                "updated_at": {
                    "type": "string"
                }
            }
        },
        "model.CategoryType": {
            "type": "integer",
            "enum": [
                1,
                2
            ],
            "x-enum-varnames": [
                "CategoryTypeArticle",
                "CategoryTypeLink"
            ]
        },
        "model.Tag": {
            "type": "object",
            "properties": {
                "created_at": {
                    "type": "string"
                },
                "id": {
                    "type": "integer"
                },
                "name": {
                    "type": "string"
                },
                "uid": {
                    "type": "string"
                },
                "updated_at": {
                    "type": "string"
                }
            }
        },
        "model.User": {
            "type": "object",
            "properties": {
                "created_at": {
                    "type": "string"
                },
                "email": {
                    "type": "string"
                },
                "id": {
                    "type": "integer"
                },
                "lastLogin": {
                    "type": "string"
                },
                "name": {
                    "type": "string"
                },
                "nick_name": {
                    "type": "string"
                },
                "permission_type": {
                    "type": "integer"
                },
                "updated_at": {
                    "type": "string"
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "",
	Host:             "",
	BasePath:         "",
	Schemes:          []string{},
	Title:            "",
	Description:      "",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
