package storage

import (
	"context"
	"github.com/Azure/terraform-provider-azapi/internal/services/myvalidator"
	"github.com/Azure/terraform-provider-azapi/internal/typed/framework"
	"github.com/Azure/terraform-provider-azapi/internal/typed/modelconv"
	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tffwdocs "github.com/magodo/terraform-plugin-framework-docs"
)

type AzApiStorageAccountResource struct {
	hooks servicehooks.ResourceHooks
}

var _ framework.Resource = AzApiStorageAccountResource{}

func (r AzApiStorageAccountResource) AzureResourceType() string {
	return "Microsoft.Storage/storageAccounts@2025-06-01"
}

func (r AzApiStorageAccountResource) TFResourceType() string {
	return "azapi_storage_account"
}

func (r AzApiStorageAccountResource) GetSchema(ctx context.Context) schema.Schema {
	schema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"parent_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					myvalidator.StringIsResourceID(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"location": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Required. Indicates the type of storage account.",
			},
			"properties": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "The parameters used to create the storage account.",
				Attributes: map[string]schema.Attribute{
					"access_tier": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Required for storage accounts where kind = BlobStorage. The access tier is used for billing. The 'Premium' access tier is the default value for premium block blobs storage account type and it cannot be changed for the premium block blobs storage account type.",
						Validators: []validator.String{
							stringvalidator.OneOf("Hot", "Cool", "Premium", "Cold"),
						},
					},
					"allow_blob_public_access": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Allow or disallow public access to all blobs or containers in the storage account. The default interpretation is false for this property.",
					},
					"allow_cross_tenant_replication": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Allow or disallow cross AAD tenant object replication. Set this property to true for new or existing accounts only if object replication policies will involve storage accounts in different AAD tenants. The default interpretation is false for new accounts to follow best security practices by default.",
					},
					"allow_shared_key_access": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Indicates whether the storage account permits requests to be authorized with the account access key via Shared Key. If false, then all requests, including shared access signatures, must be authorized with Azure Active Directory (Azure AD). The default value is null, which is equivalent to true.",
					},
					"allowed_copy_scope": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Restrict copy to and from Storage Accounts within an AAD tenant or with Private Links to the same VNet.",
					},
					"azure_files_identity_based_authentication": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Provides the identity based authentication settings for Azure Files.",
						Attributes: map[string]schema.Attribute{
							"directory_service_options": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Indicates the directory service used. Note that this enum may be extended in the future.",
							},
							"active_directory_properties": schema.SingleNestedAttribute{
								Optional:            true,
								MarkdownDescription: "Additional information about the directory service. Required if directoryServiceOptions is AD (AD DS authentication). Optional for directoryServiceOptions AADDS (Entra DS authentication) and AADKERB (Entra authentication).",
								Attributes: map[string]schema.Attribute{
									"account_type": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "Specifies the Active Directory account type for Azure Storage. If directoryServiceOptions is set to AD (AD DS authentication), this property is optional. If provided, samAccountName should also be provided. For directoryServiceOptions AADDS (Entra DS authentication) or AADKERB (Entra authentication), this property can be omitted.",
									},
									"azure_storage_sid": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "Specifies the security identifier (SID) for Azure Storage. If directoryServiceOptions is set to AD (AD DS authentication), this property is required. Otherwise, it can be omitted.",
									},
									"domain_guid": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "Specifies the domain GUID. If directoryServiceOptions is set to AD (AD DS authentication), this property is required. If directoryServiceOptions is set to AADDS (Entra DS authentication), this property can be omitted. If directoryServiceOptions is set to AADKERB (Entra authentication), this property is optional; it is needed to support configuration of directory- and file-level permissions via Windows File Explorer, but is not required for authentication.",
									},
									"domain_name": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "Specifies the primary domain that the AD DNS server is authoritative for. This property is required if directoryServiceOptions is set to AD (AD DS authentication). If directoryServiceOptions is set to AADDS (Entra DS authentication), providing this property is optional, as it will be inferred automatically if omitted. If directoryServiceOptions is set to AADKERB (Entra authentication), this property is optional; it is needed to support configuration of directory- and file-level permissions via Windows File Explorer, but is not required for authentication.",
									},
									"domain_sid": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "Specifies the security identifier (SID) of the AD domain. If directoryServiceOptions is set to AD (AD DS authentication), this property is required. Otherwise, it can be omitted.",
									},
									"forest_name": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "Specifies the Active Directory forest to get. If directoryServiceOptions is set to AD (AD DS authentication), this property is required. Otherwise, it can be omitted.",
									},
									"net_bios_domain_name": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "Specifies the NetBIOS domain name. If directoryServiceOptions is set to AD (AD DS authentication), this property is required. Otherwise, it can be omitted.",
									},
									"sam_account_name": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "Specifies the Active Directory SAMAccountName for Azure Storage. If directoryServiceOptions is set to AD (AD DS authentication), this property is optional. If provided, accountType should also be provided. For directoryServiceOptions AADDS (Entra DS authentication) or AADKERB (Entra authentication), this property can be omitted.",
									},
								},
							},
							"default_share_permission": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Default share permission for users using Kerberos authentication if RBAC role is not assigned.",
							},
							"smb_o_auth_settings": schema.SingleNestedAttribute{
								Optional:            true,
								MarkdownDescription: "Required for Managed Identities access using OAuth over SMB.",
								Attributes: map[string]schema.Attribute{
									"is_smb_o_auth_enabled": schema.BoolAttribute{
										Optional:            true,
										MarkdownDescription: "Specifies if managed identities can access SMB shares using OAuth. The default interpretation is false for this property.",
									},
								},
							},
						},
					},
					"custom_domain": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "User domain assigned to the storage account. Name is the CNAME source. Only one custom domain is supported per storage account at this time. To clear the existing custom domain, use an empty string for the custom domain name property.",
						Attributes: map[string]schema.Attribute{
							"name": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Gets or sets the custom domain name assigned to the storage account. Name is the CNAME source.",
							},
							"use_sub_domain_name": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "Indicates whether indirect CName validation is enabled. Default value is false. This should only be set on updates.",
							},
						},
					},
					"default_to_o_auth_authentication": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "A boolean flag which indicates whether the default authentication is OAuth or not. The default interpretation is false for this property.",
					},
					"dns_endpoint_type": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Allows you to specify the type of endpoint. Set this to AzureDNSZone to create a large number of accounts in a single subscription, which creates accounts in an Azure DNS Zone and the endpoint URL will have an alphanumeric DNS Zone identifier.",
					},
					"dual_stack_endpoint_preference": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Maintains information about the Internet protocol opted by the user.",
						Attributes: map[string]schema.Attribute{
							"publish_ipv6_endpoint": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "A boolean flag which indicates whether IPv6 storage endpoints are to be published.",
							},
						},
					},
					"enable_extended_groups": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Enables extended group support with local users feature, if set to true",
					},
					"encryption": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Encryption settings to be used for server-side encryption for the storage account.",
						Attributes: map[string]schema.Attribute{
							"identity": schema.SingleNestedAttribute{
								Optional:            true,
								MarkdownDescription: "The identity to be used with service-side encryption at rest.",
								Attributes: map[string]schema.Attribute{
									"federated_identity_client_id": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "ClientId of the multi-tenant application to be used in conjunction with the user-assigned identity for cross-tenant customer-managed-keys server-side encryption on the storage account.",
									},
									"user_assigned_identity": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "Resource identifier of the UserAssigned identity to be associated with server-side encryption on the storage account.",
									},
								},
							},
							"key_source": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "The encryption keySource (provider). Possible values (case-insensitive):  Microsoft.Storage, Microsoft.Keyvault",
							},
							"keyvaultproperties": schema.SingleNestedAttribute{
								Optional:            true,
								MarkdownDescription: "Properties provided by key vault.",
								Attributes: map[string]schema.Attribute{
									"keyname": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "The name of KeyVault key.",
									},
									"keyvaulturi": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "The Uri of KeyVault.",
									},
									"keyversion": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "The version of KeyVault key.",
									},
									"current_versioned_key_expiration_timestamp": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "This is a read only property that represents the expiration time of the current version of the customer managed key used for encryption.",
									},
									"current_versioned_key_identifier": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "The object identifier of the current versioned Key Vault Key in use.",
									},
									"last_key_rotation_timestamp": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Timestamp of last rotation of the Key Vault Key.",
									},
								},
							},
							"require_infrastructure_encryption": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "A boolean indicating whether or not the service applies a secondary layer of encryption with platform managed keys for data at rest.",
							},
							"services": schema.SingleNestedAttribute{
								Optional:            true,
								MarkdownDescription: "List of services which support encryption.",
								Attributes: map[string]schema.Attribute{
									"blob": schema.SingleNestedAttribute{
										Optional:            true,
										MarkdownDescription: "The encryption function of the blob storage service.",
										Attributes: map[string]schema.Attribute{
											"enabled": schema.BoolAttribute{
												Optional:            true,
												MarkdownDescription: "A boolean indicating whether or not the service encrypts the data as it is stored. Encryption at rest is enabled by default today and cannot be disabled.",
											},
											"key_type": schema.StringAttribute{
												Optional:            true,
												MarkdownDescription: "Encryption key type to be used for the encryption service. 'Account' key type implies that an account-scoped encryption key will be used. 'Service' key type implies that a default service key is used.",
											},
											"last_enabled_time": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets a rough estimate of the date/time when the encryption was last enabled by the user. Data is encrypted at rest by default today and cannot be disabled.",
											},
										},
									},
									"file": schema.SingleNestedAttribute{
										Optional:            true,
										MarkdownDescription: "The encryption function of the file storage service.",
										Attributes: map[string]schema.Attribute{
											"enabled": schema.BoolAttribute{
												Optional:            true,
												MarkdownDescription: "A boolean indicating whether or not the service encrypts the data as it is stored. Encryption at rest is enabled by default today and cannot be disabled.",
											},
											"key_type": schema.StringAttribute{
												Optional:            true,
												MarkdownDescription: "Encryption key type to be used for the encryption service. 'Account' key type implies that an account-scoped encryption key will be used. 'Service' key type implies that a default service key is used.",
											},
											"last_enabled_time": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets a rough estimate of the date/time when the encryption was last enabled by the user. Data is encrypted at rest by default today and cannot be disabled.",
											},
										},
									},
									"queue": schema.SingleNestedAttribute{
										Optional:            true,
										MarkdownDescription: "The encryption function of the queue storage service.",
										Attributes: map[string]schema.Attribute{
											"enabled": schema.BoolAttribute{
												Optional:            true,
												MarkdownDescription: "A boolean indicating whether or not the service encrypts the data as it is stored. Encryption at rest is enabled by default today and cannot be disabled.",
											},
											"key_type": schema.StringAttribute{
												Optional:            true,
												MarkdownDescription: "Encryption key type to be used for the encryption service. 'Account' key type implies that an account-scoped encryption key will be used. 'Service' key type implies that a default service key is used.",
											},
											"last_enabled_time": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets a rough estimate of the date/time when the encryption was last enabled by the user. Data is encrypted at rest by default today and cannot be disabled.",
											},
										},
									},
									"table": schema.SingleNestedAttribute{
										Optional:            true,
										MarkdownDescription: "The encryption function of the table storage service.",
										Attributes: map[string]schema.Attribute{
											"enabled": schema.BoolAttribute{
												Optional:            true,
												MarkdownDescription: "A boolean indicating whether or not the service encrypts the data as it is stored. Encryption at rest is enabled by default today and cannot be disabled.",
											},
											"key_type": schema.StringAttribute{
												Optional:            true,
												MarkdownDescription: "Encryption key type to be used for the encryption service. 'Account' key type implies that an account-scoped encryption key will be used. 'Service' key type implies that a default service key is used.",
											},
											"last_enabled_time": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets a rough estimate of the date/time when the encryption was last enabled by the user. Data is encrypted at rest by default today and cannot be disabled.",
											},
										},
									},
								},
							},
						},
					},
					"geo_priority_replication_status": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Status indicating whether Geo Priority Replication is enabled for the account.",
						Attributes: map[string]schema.Attribute{
							"is_blob_enabled": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "Indicates whether Blob Geo Priority Replication is enabled for the storage account.",
							},
						},
					},
					"immutable_storage_with_versioning": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "The property is immutable and can only be set to true at the account creation time. When set to true, it enables object level immutability for all the new containers in the account by default.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "A boolean flag which enables account-level immutability. All the containers under such an account have object-level immutability enabled by default.",
							},
							"immutability_policy": schema.SingleNestedAttribute{
								Optional:            true,
								MarkdownDescription: "Specifies the default account-level immutability policy which is inherited and applied to objects that do not possess an explicit immutability policy at the object level. The object-level immutability policy has higher precedence than the container-level immutability policy, which has a higher precedence than the account-level immutability policy.",
								Attributes: map[string]schema.Attribute{
									"allow_protected_append_writes": schema.BoolAttribute{
										Optional:            true,
										MarkdownDescription: "This property can only be changed for disabled and unlocked time-based retention policies. When enabled, new blocks can be written to an append blob while maintaining immutability protection and compliance. Only new blocks can be added and any existing blocks cannot be modified or deleted.",
									},
									"immutability_period_since_creation_in_days": schema.Int64Attribute{
										Optional:            true,
										MarkdownDescription: "The immutability period for the blobs in the container since the policy creation, in days.",
										Validators: []validator.Int64{
											int64validator.Between(1, 146000),
										},
									},
									"state": schema.StringAttribute{
										Optional:            true,
										MarkdownDescription: "The ImmutabilityPolicy state defines the mode of the policy. Disabled state disables the policy, Unlocked state allows increase and decrease of immutability retention time and also allows toggling allowProtectedAppendWrites property, Locked state only allows the increase of the immutability retention time. A policy can only be created in a Disabled or Unlocked state and can be toggled between the two states. Only a policy in an Unlocked state can transition to a Locked state which cannot be reverted.",
									},
								},
							},
						},
					},
					"is_hns_enabled": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Account HierarchicalNamespace enabled if sets to true.",
					},
					"is_local_user_enabled": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Enables local users feature, if set to true",
					},
					"is_nfs_v3_enabled": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "NFS 3.0 protocol support enabled if set to true.",
					},
					"is_sftp_enabled": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Enables Secure File Transfer Protocol, if set to true",
					},
					"key_policy": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "KeyPolicy assigned to the storage account.",
						Attributes: map[string]schema.Attribute{
							"key_expiration_period_in_days": schema.Int64Attribute{
								Required:            true,
								MarkdownDescription: "The key expiration period in days.",
							},
						},
					},
					"large_file_shares_state": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Allow large file shares if sets to Enabled. It cannot be disabled once it is enabled.",
					},
					"minimum_tls_version": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Set the minimum TLS version to be permitted on requests to storage. The default interpretation is TLS 1.0 for this property.",
					},
					"network_acls": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Network rule set",
						Attributes: map[string]schema.Attribute{
							"default_action": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Specifies the default action of allow or deny when no other rules match.",
								Validators: []validator.String{
									stringvalidator.OneOf("Allow", "Deny"),
								},
							},
							"bypass": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Specifies whether traffic is bypassed for Logging/Metrics/AzureServices. Possible values are any combination of Logging|Metrics|AzureServices (For example, \"Logging, Metrics\"), or None to bypass none of those traffics.",
							},
							"ip_rules": schema.ListNestedAttribute{
								Optional:            true,
								MarkdownDescription: "Sets the IP ACL rules",
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"value": schema.StringAttribute{
											Required:            true,
											MarkdownDescription: "Specifies the IP or IP range in CIDR format.",
										},
										"action": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "The action of IP ACL rule.",
											Validators: []validator.String{
												stringvalidator.OneOf("Allow"),
											},
										},
									},
								},
							},
							"ipv6_rules": schema.ListNestedAttribute{
								Optional:            true,
								MarkdownDescription: "Sets the IPv6 ACL rules.",
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"value": schema.StringAttribute{
											Required:            true,
											MarkdownDescription: "Specifies the IP or IP range in CIDR format.",
										},
										"action": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "The action of IP ACL rule.",
											Validators: []validator.String{
												stringvalidator.OneOf("Allow"),
											},
										},
									},
								},
							},
							"resource_access_rules": schema.ListNestedAttribute{
								Optional:            true,
								MarkdownDescription: "Sets the resource access rules",
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"resource_id": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "Resource Id",
										},
										"tenant_id": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "Tenant Id",
										},
									},
								},
							},
							"virtual_network_rules": schema.ListNestedAttribute{
								Optional:            true,
								MarkdownDescription: "Sets the virtual network rules",
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"id": schema.StringAttribute{
											Required:            true,
											MarkdownDescription: "Resource ID of a subnet, for example: /subscriptions/{subscriptionId}/resourceGroups/{groupName}/providers/Microsoft.Network/virtualNetworks/{vnetName}/subnets/{subnetName}.",
										},
										"action": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "The action of virtual network rule.",
											Validators: []validator.String{
												stringvalidator.OneOf("Allow"),
											},
										},
										"state": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "Gets the state of virtual network rule.",
										},
									},
								},
							},
						},
					},
					"public_network_access": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Allow, disallow, or let Network Security Perimeter configuration to evaluate public network access to Storage Account. Value is optional but if passed in, must be 'Enabled', 'Disabled' or 'SecuredByPerimeter'.",
					},
					"routing_preference": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Maintains information about the network routing choice opted by the user for data transfer",
						Attributes: map[string]schema.Attribute{
							"publish_internet_endpoints": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "A boolean flag which indicates whether internet routing storage endpoints are to be published",
							},
							"publish_microsoft_endpoints": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "A boolean flag which indicates whether microsoft routing storage endpoints are to be published",
							},
							"routing_choice": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Routing Choice defines the kind of network routing opted by the user.",
							},
						},
					},
					"sas_policy": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "SasPolicy assigned to the storage account.",
						Attributes: map[string]schema.Attribute{
							"expiration_action": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The SAS Expiration Action defines the action to be performed when sasPolicy.sasExpirationPeriod is violated. The 'Log' action can be used for audit purposes and the 'Block' action can be used to block and deny the usage of SAS tokens that do not adhere to the sas policy expiration period.",
							},
							"sas_expiration_period": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The SAS expiration period, DD.HH:MM:SS.",
							},
						},
					},
					"supports_https_traffic_only": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Allows https traffic only to storage service if sets to true. The default value is true since API version 2019-04-01.",
					},
					"account_migration_in_progress": schema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "If customer initiated account migration is in progress, the value will be true else it will be null.",
					},
					"blob_restore_status": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Blob restore status",
						Attributes: map[string]schema.Attribute{
							"failure_reason": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Failure reason when blob restore is failed.",
							},
							"parameters": schema.SingleNestedAttribute{
								Computed:            true,
								MarkdownDescription: "Blob restore request parameters.",
								Attributes: map[string]schema.Attribute{
									"blob_ranges": schema.ListNestedAttribute{
										Computed:            true,
										MarkdownDescription: "Blob ranges to restore.",
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												"end_range": schema.StringAttribute{
													Computed:            true,
													MarkdownDescription: "Blob end range. This is exclusive. Empty means account end.",
												},
												"start_range": schema.StringAttribute{
													Computed:            true,
													MarkdownDescription: "Blob start range. This is inclusive. Empty means account start.",
												},
											},
										},
									},
									"time_to_restore": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Restore blob to the specified time.",
									},
								},
							},
							"restore_id": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Id for tracking blob restore request.",
							},
							"status": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "The status of blob restore progress. Possible values are: - InProgress: Indicates that blob restore is ongoing. - Complete: Indicates that blob restore has been completed successfully. - Failed: Indicates that blob restore is failed.",
							},
						},
					},
					"creation_time": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Gets the creation date and time of the storage account in UTC.",
					},
					"failover_in_progress": schema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "If the failover is in progress, the value will be true, otherwise, it will be null.",
					},
					"geo_replication_stats": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Geo Replication Stats",
						Attributes: map[string]schema.Attribute{
							"can_failover": schema.BoolAttribute{
								Computed:            true,
								MarkdownDescription: "A boolean flag which indicates whether or not account failover is supported for the account.",
							},
							"can_planned_failover": schema.BoolAttribute{
								Computed:            true,
								MarkdownDescription: "A boolean flag which indicates whether or not planned account failover is supported for the account.",
							},
							"last_sync_time": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "All primary writes preceding this UTC date/time value are guaranteed to be available for read operations. Primary writes following this point in time may or may not be available for reads. Element may be default value if value of LastSyncTime is not available, this can happen if secondary is offline or we are in bootstrap.",
							},
							"post_failover_redundancy": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "The redundancy type of the account after an account failover is performed.",
							},
							"post_planned_failover_redundancy": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "The redundancy type of the account after a planned account failover is performed.",
							},
							"status": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "The status of the secondary location. Possible values are: - Live: Indicates that the secondary location is active and operational. - Bootstrap: Indicates initial synchronization from the primary location to the secondary location is in progress.This typically occurs when replication is first enabled. - Unavailable: Indicates that the secondary location is temporarily unavailable.",
							},
						},
					},
					"is_sku_conversion_blocked": schema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "This property will be set to true or false on an event of ongoing migration. Default value is null.",
					},
					"key_creation_time": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Storage account keys creation time.",
						Attributes: map[string]schema.Attribute{
							"key1": schema.StringAttribute{
								Computed: true,
							},
							"key2": schema.StringAttribute{
								Computed: true,
							},
						},
					},
					"last_geo_failover_time": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Gets the timestamp of the most recent instance of a failover to the secondary location. Only the most recent timestamp is retained. This element is not returned if there has never been a failover instance. Only available if the accountType is Standard_GRS or Standard_RAGRS.",
					},
					"primary_endpoints": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Gets the URLs that are used to perform a retrieval of a public blob, queue, or table object. Note that Standard_ZRS and Premium_LRS accounts only return the blob endpoint.",
						Attributes: map[string]schema.Attribute{
							"blob": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the blob endpoint.",
							},
							"dfs": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the dfs endpoint.",
							},
							"file": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the file endpoint.",
							},
							"internet_endpoints": schema.SingleNestedAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the internet routing storage endpoints",
								Attributes: map[string]schema.Attribute{
									"blob": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the blob endpoint.",
									},
									"dfs": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the dfs endpoint.",
									},
									"file": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the file endpoint.",
									},
									"web": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the web endpoint.",
									},
								},
							},
							"ipv6_endpoints": schema.SingleNestedAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the IPv6 storage endpoints.",
								Attributes: map[string]schema.Attribute{
									"blob": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the blob endpoint.",
									},
									"dfs": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the dfs endpoint.",
									},
									"file": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the file endpoint.",
									},
									"internet_endpoints": schema.SingleNestedAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the internet routing storage endpoints",
										Attributes: map[string]schema.Attribute{
											"blob": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the blob endpoint.",
											},
											"dfs": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the dfs endpoint.",
											},
											"file": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the file endpoint.",
											},
											"web": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the web endpoint.",
											},
										},
									},
									"microsoft_endpoints": schema.SingleNestedAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the microsoft routing storage endpoints.",
										Attributes: map[string]schema.Attribute{
											"blob": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the blob endpoint.",
											},
											"dfs": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the dfs endpoint.",
											},
											"file": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the file endpoint.",
											},
											"queue": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the queue endpoint.",
											},
											"table": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the table endpoint.",
											},
											"web": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the web endpoint.",
											},
										},
									},
									"queue": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the queue endpoint.",
									},
									"table": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the table endpoint.",
									},
									"web": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the web endpoint.",
									},
								},
							},
							"microsoft_endpoints": schema.SingleNestedAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the microsoft routing storage endpoints.",
								Attributes: map[string]schema.Attribute{
									"blob": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the blob endpoint.",
									},
									"dfs": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the dfs endpoint.",
									},
									"file": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the file endpoint.",
									},
									"queue": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the queue endpoint.",
									},
									"table": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the table endpoint.",
									},
									"web": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the web endpoint.",
									},
								},
							},
							"queue": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the queue endpoint.",
							},
							"table": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the table endpoint.",
							},
							"web": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the web endpoint.",
							},
						},
					},
					"primary_location": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Gets the location of the primary data center for the storage account.",
					},
					"private_endpoint_connections": schema.ListNestedAttribute{
						Computed:            true,
						MarkdownDescription: "List of private endpoint connection associated with the specified storage account",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Fully qualified resource ID for the resource. E.g. \"/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/{resourceProviderNamespace}/{resourceType}/{resourceName}\"",
								},
							},
						},
					},
					"secondary_endpoints": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Gets the URLs that are used to perform a retrieval of a public blob, queue, or table object from the secondary location of the storage account. Only available if the SKU name is Standard_RAGRS.",
						Attributes: map[string]schema.Attribute{
							"blob": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the blob endpoint.",
							},
							"dfs": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the dfs endpoint.",
							},
							"file": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the file endpoint.",
							},
							"internet_endpoints": schema.SingleNestedAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the internet routing storage endpoints",
								Attributes: map[string]schema.Attribute{
									"blob": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the blob endpoint.",
									},
									"dfs": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the dfs endpoint.",
									},
									"file": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the file endpoint.",
									},
									"web": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the web endpoint.",
									},
								},
							},
							"ipv6_endpoints": schema.SingleNestedAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the IPv6 storage endpoints.",
								Attributes: map[string]schema.Attribute{
									"blob": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the blob endpoint.",
									},
									"dfs": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the dfs endpoint.",
									},
									"file": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the file endpoint.",
									},
									"internet_endpoints": schema.SingleNestedAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the internet routing storage endpoints",
										Attributes: map[string]schema.Attribute{
											"blob": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the blob endpoint.",
											},
											"dfs": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the dfs endpoint.",
											},
											"file": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the file endpoint.",
											},
											"web": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the web endpoint.",
											},
										},
									},
									"microsoft_endpoints": schema.SingleNestedAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the microsoft routing storage endpoints.",
										Attributes: map[string]schema.Attribute{
											"blob": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the blob endpoint.",
											},
											"dfs": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the dfs endpoint.",
											},
											"file": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the file endpoint.",
											},
											"queue": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the queue endpoint.",
											},
											"table": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the table endpoint.",
											},
											"web": schema.StringAttribute{
												Computed:            true,
												MarkdownDescription: "Gets the web endpoint.",
											},
										},
									},
									"queue": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the queue endpoint.",
									},
									"table": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the table endpoint.",
									},
									"web": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the web endpoint.",
									},
								},
							},
							"microsoft_endpoints": schema.SingleNestedAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the microsoft routing storage endpoints.",
								Attributes: map[string]schema.Attribute{
									"blob": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the blob endpoint.",
									},
									"dfs": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the dfs endpoint.",
									},
									"file": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the file endpoint.",
									},
									"queue": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the queue endpoint.",
									},
									"table": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the table endpoint.",
									},
									"web": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Gets the web endpoint.",
									},
								},
							},
							"queue": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the queue endpoint.",
							},
							"table": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the table endpoint.",
							},
							"web": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Gets the web endpoint.",
							},
						},
					},
					"secondary_location": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Gets the location of the geo-replicated secondary for the storage account. Only available if the accountType is Standard_GRS or Standard_RAGRS.",
					},
					"status_of_primary": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Gets the status indicating whether the primary location of the storage account is available or unavailable.",
						Validators: []validator.String{
							stringvalidator.OneOf("available", "unavailable"),
						},
					},
					"status_of_secondary": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Gets the status indicating whether the secondary location of the storage account is available or unavailable. Only available if the SKU name is Standard_GRS or Standard_RAGRS.",
						Validators: []validator.String{
							stringvalidator.OneOf("available", "unavailable"),
						},
					},
					"storage_account_sku_conversion_status": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "This property is readOnly and is set by server during asynchronous storage account sku conversion operations.",
						Attributes: map[string]schema.Attribute{
							"end_time": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "This property represents the sku conversion end time.",
							},
							"sku_conversion_status": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "This property indicates the current sku conversion status.",
							},
							"start_time": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "This property represents the sku conversion start time.",
							},
							"target_sku_name": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "This property represents the target sku name to which the account sku is being converted asynchronously.",
							},
						},
					},
				},
			},
			"sku": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Required. Gets or sets the SKU name.",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "The SKU name. Required for account creation; optional for update. Note that in older versions, SKU name was called accountType.",
					},
					"tier": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The SKU tier. This is based on the SKU name.",
						Validators: []validator.String{
							stringvalidator.OneOf("Standard", "Premium"),
						},
					},
				},
			},
			"extended_location": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Optional. Set the extended location of the resource. If not set, the storage account will be created in Azure main region. Otherwise it will be created in the specified extended location",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "The name of the extended location.",
					},
					"type": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "The type of the extended location.",
					},
				},
			},
			"identity": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "The identity of the resource.",
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "The identity type.",
					},
					"user_assigned_identities": schema.MapNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Gets or sets a list of key value pairs that describe the set of User Assigned identities that will be used with this storage account. The key is the ARM resource identifier of the identity. Only 1 User Assigned identity is permitted here.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"client_id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The client ID of the identity.",
								},
								"principal_id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The principal ID of the identity.",
								},
							},
						},
					},
					"principal_id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The principal ID of resource identity.",
					},
					"tenant_id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The tenant ID of resource.",
					},
				},
			},
			"placement": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Optional. Gets or sets the zonal placement details for the storage account.",
				Attributes: map[string]schema.Attribute{
					"zone_placement_policy": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "The availability zone pinning policy for the storage account.",
					},
				},
			},
			"tags": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Gets or sets a list of key value pairs that describe the resource. These tags can be used for viewing and grouping this resource (across resource groups). A maximum of 15 tags can be provided for a resource. Each tag must have a key with a length no greater than 128 characters and a value with a length no greater than 256 characters.",
			},
			"zones": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Optional. Gets or sets the pinned logical availability zone for the storage account.",
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Create: true,
				Read:   true,
				Update: true,
				Delete: true,
			}),
		},
	}
	if r.hooks.SchemaHook != nil {
		schema = r.hooks.SchemaHook(schema)
	}
	return schema
}

func (r AzApiStorageAccountResource) GetModelConvOption() *modelconv.Option {
	opt := modelconv.NewDefaultOption()
	if r.hooks.ModelConvOptionHook != nil {
		opt = r.hooks.ModelConvOptionHook(opt)
	}
	return &opt
}

func (r AzApiStorageAccountResource) RenderOption() tffwdocs.ResourceRenderOption {
	opt := tffwdocs.ResourceRenderOption{
		Subcategory: "storage",
		ImportId: &tffwdocs.ImportId{
			Format:    "<resource_id>",
			ExampleId: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Storage/storageAccounts/myStorageAccount",
		},
	}
	if r.hooks.RenderOptionHook != nil {
		opt = r.hooks.RenderOptionHook(opt)
	}
	return opt
}
