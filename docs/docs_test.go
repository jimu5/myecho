package docs

import "testing"

func TestSwaggerInfoIsRegisteredSpec(t *testing.T) {
	if SwaggerInfo == nil {
		t.Fatal("SwaggerInfo is nil")
	}
	if SwaggerInfo.InstanceName() != "swagger" {
		t.Fatalf("InstanceName() = %q, want swagger", SwaggerInfo.InstanceName())
	}
	if SwaggerInfo.SwaggerTemplate == "" {
		t.Fatal("SwaggerTemplate is empty")
	}
}
