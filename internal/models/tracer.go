package models

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("github.com/halkeye/omnitar/internal/models")
