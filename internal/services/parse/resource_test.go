package parse

import "testing"

func TestResourceIDFormatter(t *testing.T) {
	id, err := NewResourceID("/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.EventHub/clusters/cluster1", "Microsoft.EventHub/clusters@2020-12-01")
	if err != nil {
		t.Fatal(err)
	}
	actual := id.ID()
	expected := "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.EventHub/clusters/cluster1"
	if actual != expected {
		t.Fatalf("Expected %q but got %q", expected, actual)
	}
}

func Test_ResourceID(t *testing.T) {
	testData := []struct {
		Input            string
		Error            bool
		ResourceDefExist bool
		Expected         *ResourceId
	}{

		{
			// tenant scope
			Input:            "/providers/Microsoft.Management/managementGroups/test?api-version=2021-04-01",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2021-04-01",
				AzureResourceType: "Microsoft.Management/managementGroups",
				AzureResourceId:   "/providers/Microsoft.Management/managementGroups/test",
				Name:              "test",
				ParentId:          "",
			},
		},

		{
			// subscription scope
			Input:            "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/test?api-version=2021-04-01",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2021-04-01",
				AzureResourceType: "Microsoft.Resources/resourceGroups",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/test",
				Name:              "test",
				ParentId:          "/subscriptions/12345678-1234-9876-4563-123456789012",
			},
		},

		{
			// management group scope
			Input:            "/providers/Microsoft.Management/managementGroups/myMgmtGroup/providers/Microsoft.Authorization/policyDefinitions/test?api-version=2021-06-01",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2021-06-01",
				AzureResourceType: "Microsoft.Authorization/policyDefinitions",
				AzureResourceId:   "/providers/Microsoft.Management/managementGroups/myMgmtGroup/providers/Microsoft.Authorization/policyDefinitions/test",

				Name:     "test",
				ParentId: "/providers/Microsoft.Management/managementGroups/myMgmtGroup",
			},
		},

		{
			// resource group scope, top level resource
			Input:            "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/test?api-version=2020-11-01-preview",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2020-11-01-preview",
				AzureResourceType: "Microsoft.ContainerRegistry/registries",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/test",
				Name:              "test",
				ParentId:          "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1",
			},
		},

		{
			// resource group scope, child resource
			Input:            "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/scopeMaps/test?api-version=2020-11-01-preview",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2020-11-01-preview",
				AzureResourceType: "Microsoft.ContainerRegistry/registries/scopeMaps",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/scopeMaps/test",
				Name:              "test",
				ParentId:          "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry",
			},
		},

		{
			// extension scope
			Input:            "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/providers/Microsoft.CostManagement/reports/test?api-version=2018-08-01-preview",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2018-08-01-preview",
				AzureResourceType: "Microsoft.CostManagement/reports",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/providers/Microsoft.CostManagement/reports/test",
				Name:              "test",
				ParentId:          "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry",
			},
		},

		{
			// Microsoft.CostManagement/reports supports both extension and resource group scopes
			Input:            "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.CostManagement/reports/test?api-version=2018-08-01-preview",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2018-08-01-preview",
				AzureResourceType: "Microsoft.CostManagement/reports",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.CostManagement/reports/test",
				Name:              "test",
				ParentId:          "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1",
			},
		},

		{
			// Unknown scope, according to parent_id, it should be an extension resource
			Input:            "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/providers/Microsoft.Insights/diagnosticSettings/test?api-version=2016-09-01",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2016-09-01",
				AzureResourceType: "Microsoft.Insights/diagnosticSettings",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/providers/Microsoft.Insights/diagnosticSettings/test",
				Name:              "test",
				ParentId:          "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry",
			},
		},

		{
			// Unknown types, according to parent_id, it should be an extension resource
			Input:            "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/providers/Microsoft.Foo/Bar/test?api-version=2016-09-01",
			ResourceDefExist: false,
			Expected: &ResourceId{
				ApiVersion:        "2016-09-01",
				AzureResourceType: "Microsoft.Foo/Bar",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/providers/Microsoft.Foo/Bar/test",
				Name:              "test",
				ParentId:          "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry",
			},
		},

		{
			// Unknown types, according to parent_id, it should be a child resource
			Input:            "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/foo/test?api-version=2020-11-01-preview",
			ResourceDefExist: false,
			Expected: &ResourceId{
				ApiVersion:        "2020-11-01-preview",
				AzureResourceType: "Microsoft.ContainerRegistry/registries/foo",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/foo/test",
				Name:              "test",
				ParentId:          "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry",
			},
		},

		{
			// empty
			Input: "",
			Error: true,
		},

		{
			// invalid api-version
			Input: "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1",
			Error: true,
		},
	}

	for _, v := range testData {
		t.Logf("[DEBUG] Testing %q", v.Input)

		actual, err := ResourceID(v.Input)
		if err != nil {
			if v.Error {
				continue
			}

			t.Fatalf("Expect a value but got an error: %s", err)
		}
		if v.Error {
			t.Fatal("Expect an error but didn't get one")
		}

		if actual.AzureResourceId != v.Expected.AzureResourceId {
			t.Fatalf("Expected %q but got %q for AzureResourceId", v.Expected.AzureResourceId, actual.AzureResourceId)
		}
		if actual.ApiVersion != v.Expected.ApiVersion {
			t.Fatalf("Expected %q but got %q for ApiVersion", v.Expected.ApiVersion, actual.ApiVersion)
		}
		if actual.AzureResourceType != v.Expected.AzureResourceType {
			t.Fatalf("Expected %q but got %q for AzureResourceType", v.Expected.AzureResourceType, actual.AzureResourceType)
		}
		if v.ResourceDefExist && actual.ResourceDef == nil {
			t.Fatal("Expected a resource def but got nil")
		}
	}
}

func Test_BuildResourceID(t *testing.T) {
	testData := []struct {
		Name             string
		ParentId         string
		ResourceType     string
		ResourceDefExist bool
		Error            bool
		Expected         *ResourceId
	}{
		{
			// tenant scope
			Name:             "test",
			ParentId:         "",
			ResourceType:     "Microsoft.Management/managementGroups@2021-04-01",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2021-04-01",
				AzureResourceType: "Microsoft.Management/managementGroups",
				AzureResourceId:   "/providers/Microsoft.Management/managementGroups/test",
			},
		},

		{
			// subscription scope
			Name:             "test",
			ParentId:         "/subscriptions/12345678-1234-9876-4563-123456789012",
			ResourceType:     "Microsoft.Resources/resourceGroups@2021-04-01",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2021-04-01",
				AzureResourceType: "Microsoft.Resources/resourceGroups",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/test",
			},
		},

		{
			// management group scope
			Name:             "test",
			ParentId:         "/providers/Microsoft.Management/managementGroups/myMgmtGroup",
			ResourceType:     "Microsoft.Authorization/policyDefinitions@2021-06-01",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2021-06-01",
				AzureResourceType: "Microsoft.Authorization/policyDefinitions",
				AzureResourceId:   "/providers/Microsoft.Management/managementGroups/myMgmtGroup/providers/Microsoft.Authorization/policyDefinitions/test",
			},
		},

		{
			// resource group scope, top level resource
			Name:             "test",
			ParentId:         "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1",
			ResourceType:     "Microsoft.ContainerRegistry/registries@2020-11-01-preview",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2020-11-01-preview",
				AzureResourceType: "Microsoft.ContainerRegistry/registries",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/test",
			},
		},

		{
			// resource group scope, child resource
			Name:             "test",
			ParentId:         "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry",
			ResourceType:     "Microsoft.ContainerRegistry/registries/scopeMaps@2020-11-01-preview",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2020-11-01-preview",
				AzureResourceType: "Microsoft.ContainerRegistry/registries/scopeMaps",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/scopeMaps/test",
			},
		},

		{
			// extension scope
			Name:             "test",
			ParentId:         "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry",
			ResourceType:     "Microsoft.CostManagement/reports@2018-08-01-preview",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2018-08-01-preview",
				AzureResourceType: "Microsoft.CostManagement/reports",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/providers/Microsoft.CostManagement/reports/test",
			},
		},

		{
			// Microsoft.CostManagement/reports supports both extension and resource group scopes
			Name:             "test",
			ParentId:         "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1",
			ResourceType:     "Microsoft.CostManagement/reports@2018-08-01-preview",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2018-08-01-preview",
				AzureResourceType: "Microsoft.CostManagement/reports",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.CostManagement/reports/test",
			},
		},

		{
			// Unknown scope, according to parent_id, it should be an extension resource
			Name:             "test",
			ParentId:         "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry",
			ResourceType:     "Microsoft.Insights/diagnosticSettings@2016-09-01",
			ResourceDefExist: true,
			Expected: &ResourceId{
				ApiVersion:        "2016-09-01",
				AzureResourceType: "Microsoft.Insights/diagnosticSettings",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/providers/Microsoft.Insights/diagnosticSettings/test",
			},
		},

		{
			// Unknown types, according to parent_id, it should be an extension resource
			Name:             "test",
			ParentId:         "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry",
			ResourceType:     "Microsoft.Foo/Bar@2016-09-01",
			ResourceDefExist: false,
			Expected: &ResourceId{
				ApiVersion:        "2016-09-01",
				AzureResourceType: "Microsoft.Foo/Bar",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/providers/Microsoft.Foo/Bar/test",
			},
		},

		{
			// Unknown types, according to parent_id, it should be a child resource
			Name:             "test",
			ParentId:         "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry",
			ResourceType:     "Microsoft.ContainerRegistry/registries/foo@2020-11-01-preview",
			ResourceDefExist: false,
			Expected: &ResourceId{
				ApiVersion:        "2020-11-01-preview",
				AzureResourceType: "Microsoft.ContainerRegistry/registries/foo",
				AzureResourceId:   "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1/providers/Microsoft.ContainerRegistry/registries/myRegistry/foo/test",
			},
		},

		{
			// invalid parent_id, should be container registry's id
			Name:         "test",
			ParentId:     "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/group1",
			ResourceType: "Microsoft.ContainerRegistry/registries/scopeMaps@2020-11-01-preview",
			Error:        true,
		},

		{
			// invalid parent_id, should be resource group's id
			Name:         "test",
			ParentId:     "/subscriptions/12345678-1234-9876-4563-123456789012",
			ResourceType: "Microsoft.ContainerRegistry/registries@2020-11-01-preview",
			Error:        true,
		},

		{
			// invalid parent_id, should be empty
			Name:         "test",
			ParentId:     "/subscriptions/12345678-1234-9876-4563-123456789012",
			ResourceType: "Microsoft.Management/managementGroups@2021-04-01",
			Error:        true,
		},

		{
			// invalid parent_id, should be subscriptions/{subscription_id}
			Name:         "test",
			ParentId:     "",
			ResourceType: "Microsoft.Resources/resourceGroups@2021-04-01",
			Error:        true,
		},

		{
			// invalid parent_id
			Name:         "test",
			ParentId:     "/providers/Microsoft.Management",
			ResourceType: "Microsoft.Authorization/policyDefinitions@2021-06-01",
			Error:        true,
		},
	}

	for _, v := range testData {
		t.Logf("[DEBUG] Testing %q %q %q", v.Name, v.ParentId, v.ResourceType)

		actual, err := BuildResourceID(v.Name, v.ParentId, v.ResourceType)
		if err != nil {
			if v.Error {
				continue
			}

			t.Fatalf("Expect a value but got an error: %s", err)
		}
		if v.Error {
			t.Fatal("Expect an error but didn't get one")
		}

		if actual.AzureResourceId != v.Expected.AzureResourceId {
			t.Fatalf("Expected %q but got %q for AzureResourceId", v.Expected.AzureResourceId, actual.AzureResourceId)
		}
		if actual.ApiVersion != v.Expected.ApiVersion {
			t.Fatalf("Expected %q but got %q for ApiVersion", v.Expected.ApiVersion, actual.ApiVersion)
		}
		if actual.AzureResourceType != v.Expected.AzureResourceType {
			t.Fatalf("Expected %q but got %q for AzureResourceType", v.Expected.AzureResourceType, actual.AzureResourceType)
		}
		if v.ResourceDefExist && actual.ResourceDef == nil {
			t.Fatal("Expected a resource def but got nil")
		}
	}
}
