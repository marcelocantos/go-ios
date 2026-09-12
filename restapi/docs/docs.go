// Package docs is a compile stub so `go build .` works without a prior
// `swag init`. Deploy still regenerates the full swagger spec.
package docs

import "github.com/swaggo/swag"

const docTemplate = `{"swagger":"2.0","info":{"title":"Go-iOS API","version":"0.01"},"basePath":"/api/v1","paths":{}}`

var SwaggerInfo = &swag.Spec{
	Version:          "0.01",
	Host:             "localhost:8080",
	BasePath:         "/api/v1",
	Title:            "Go-iOS API",
	Description:      "Exposes go-ios features as REST API calls.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
