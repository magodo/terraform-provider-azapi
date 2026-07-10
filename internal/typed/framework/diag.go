package framework

import "github.com/hashicorp/terraform-plugin-framework/diag"

var DiagResourceNotFound = diag.NewErrorDiagnostic("resource not found", "This resource is not found in Azure")
