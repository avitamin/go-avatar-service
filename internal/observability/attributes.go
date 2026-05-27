package observability

import "go.opentelemetry.io/otel/attribute"

type attributeOption struct {
	key   string
	value string
}

func toAttributes(opts []attributeOption) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(opts))
	for _, opt := range opts {
		attrs = append(attrs, attribute.String(opt.key, opt.value))
	}
	return attrs
}
