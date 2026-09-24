// Package openapi embeds the asset (ITAM) browser API contract.
package openapi

import _ "embed"

// Asset is the OpenAPI 3.1 document served and validated by the service.
//
//go:embed asset.yaml
var Asset []byte
