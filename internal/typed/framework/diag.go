package framework

import "github.com/hashicorp/terraform-plugin-framework/diag"

var diagResourceNotFound = diag.NewErrorDiagnostic("resource not found", "This resource is not found in Azure")
