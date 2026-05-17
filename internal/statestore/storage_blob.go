package statestore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/terraform-provider-azapi/internal/clients"
	"github.com/hashicorp/terraform-plugin-framework/statestore"
	ststschema "github.com/hashicorp/terraform-plugin-framework/statestore/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// StorageBlobStateStore implements a Terraform state store backed by an Azure
// Storage Account blob container. It mirrors the semantics of the
// `azurerm` Terraform backend:
//
//   - The "default" workspace stores its state at "<key>".
//   - Non-default workspaces store state at "<key>env:<workspace>".
//   - Locking is implemented by creating an exclusive "<blob>.tflock" blob
//     containing the lock info as JSON. Acquisition uses a conditional
//     "If-None-Match: *" write so that concurrent clients cannot create the
//     same lock blob.
//
// The state store re-uses the credential, cloud configuration and HTTP
// transport from the provider client.
type StorageBlobStateStore struct {
	containerClient *container.Client

	// key is the configured "key" attribute, used as the blob name for the
	// "default" workspace.
	key string

	// snapshot is set from configuration and controls whether a blob
	// snapshot is taken before each state write.
	snapshot bool
}

// StorageBlobModel maps to the state_store configuration block.
type StorageBlobModel struct {
	StorageAccountName types.String `tfsdk:"storage_account_name"`
	ContainerName      types.String `tfsdk:"container_name"`
	Key                types.String `tfsdk:"key"`
	Endpoint           types.String `tfsdk:"endpoint"`
	Snapshot           types.Bool   `tfsdk:"snapshot"`
}

var (
	_ statestore.StateStore              = &StorageBlobStateStore{}
	_ statestore.StateStoreWithConfigure = &StorageBlobStateStore{}
)

func NewStorageBlobStateStore() statestore.StateStore {
	return &StorageBlobStateStore{}
}

func (s *StorageBlobStateStore) Metadata(_ context.Context, req statestore.MetadataRequest, resp *statestore.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_blob"
}

func (s *StorageBlobStateStore) Schema(_ context.Context, _ statestore.SchemaRequest, resp *statestore.SchemaResponse) {
	resp.Schema = ststschema.Schema{
		MarkdownDescription: "Stores Terraform state in an Azure Storage Account blob container.",
		Attributes: map[string]ststschema.Attribute{
			"storage_account_name": ststschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the storage account that hosts the blob container.",
			},
			"container_name": ststschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the blob container in which to store the state.",
			},
			"key": ststschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the blob that stores the state for the `default` workspace. Non-default workspaces are stored as `<key>env:<workspace>`.",
			},
			"endpoint": ststschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Override the blob service endpoint. If not specified, `https://<storage_account_name>.blob.core.windows.net` is used.",
			},
			"snapshot": ststschema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether to create a snapshot of the existing state blob before overwriting it. Defaults to `false`.",
			},
		},
	}
}

func (s *StorageBlobStateStore) Initialize(ctx context.Context, req statestore.InitializeRequest, resp *statestore.InitializeResponse) {
	// Provider-level data is the *clients.Client populated during the
	// provider's Configure phase. We need this to obtain a TokenCredential
	// and the configured cloud.
	client, ok := req.ProviderData.(*clients.Client)
	if !ok || client == nil || client.Option == nil {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data",
			fmt.Sprintf("State store expected provider data of type *clients.Client, got: %T. The provider was not properly configured before initializing the state store.", req.ProviderData),
		)
		return
	}

	var cfg StorageBlobModel
	if diags := req.Config.Get(ctx, &cfg); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	account := strings.TrimSpace(cfg.StorageAccountName.ValueString())
	containerName := strings.TrimSpace(cfg.ContainerName.ValueString())
	if account == "" || containerName == "" {
		resp.Diagnostics.AddError("Invalid Configuration", "`storage_account_name` and `container_name` must both be set.")
		return
	}

	endpoint := strings.TrimSpace(cfg.Endpoint.ValueString())
	if endpoint == "" {
		suffix := "blob.core.windows.net"
		// Best-effort: derive the suffix from the resource manager endpoint, which has
		// no public mapping to the storage suffix in the SDK. The common known
		// clouds are handled explicitly.
		if rm, ok := client.Option.CloudCfg.Services[cloud.ResourceManager]; ok {
			switch {
			case strings.Contains(rm.Endpoint, "chinacloudapi.cn"):
				suffix = "blob.core.chinacloudapi.cn"
			case strings.Contains(rm.Endpoint, "usgovcloudapi.net"):
				suffix = "blob.core.usgovcloudapi.net"
			}
		}
		endpoint = fmt.Sprintf("https://%s.%s", account, suffix)
	}
	if _, err := url.Parse(endpoint); err != nil {
		resp.Diagnostics.AddError("Invalid Endpoint", fmt.Sprintf("Could not parse blob endpoint %q: %s", endpoint, err))
		return
	}

	svc, err := azblob.NewClient(strings.TrimRight(endpoint, "/")+"/", client.Option.Cred, &azblob.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud: client.Option.CloudCfg,
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to Create Storage Client", err.Error())
		return
	}

	resp.StateStoreData = &StorageBlobStateStore{
		containerClient: svc.ServiceClient().NewContainerClient(containerName),
		key:             strings.TrimSpace(cfg.Key.ValueString()),
		snapshot:        cfg.Snapshot.ValueBool(),
	}
}

func (s *StorageBlobStateStore) Configure(_ context.Context, req statestore.ConfigureRequest, resp *statestore.ConfigureResponse) {
	if req.StateStoreData == nil {
		return
	}
	data, ok := req.StateStoreData.(*StorageBlobStateStore)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected StateStore Data",
			fmt.Sprintf("Expected *initializedData, got %T", req.StateStoreData),
		)
		return
	}
	*s = *data
}

func (s *StorageBlobStateStore) BlobClient(name string) *blob.Client {
	return s.containerClient.NewBlobClient(name)
}

func isBlobNotFound(err error) bool {
	return bloberror.HasCode(err, bloberror.BlobNotFound)
}

func (s *StorageBlobStateStore) GetStates(ctx context.Context, _ statestore.GetStatesRequest, resp *statestore.GetStatesResponse) {
	// We cannot enumerate "states" without knowing the configured key. To
	// support all keys present in the container, list all blobs and infer
	// workspaces from the "env:" separator convention. This matches the
	// behaviour of the AzureRM backend's `Workspaces()` method.
	pager := s.containerClient.NewListBlobsFlatPager(nil)
	seen := map[string]struct{}{"default": {}}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Failed to List State Blobs", err.Error())
			return
		}
		for _, item := range page.Segment.BlobItems {
			if item == nil || item.Name == nil {
				continue
			}
			name := *item.Name
			if strings.HasSuffix(name, ".tflock") {
				continue
			}
			if _, ws, ok := strings.Cut(name, "env:"); ok && ws != "" {
				seen[ws] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for ws := range seen {
		out = append(out, ws)
	}
	resp.StateIDs = out
}

// resolveBlob returns the blob name for the given state ID, applying the same
// `<key>` / `<key>env:<workspace>` convention used by the AzureRM backend.
func (s *StorageBlobStateStore) resolveBlob(stateID string) string {
	if stateID == "" || stateID == "default" {
		return s.key
	}
	return fmt.Sprintf("%senv:%s", s.key, stateID)
}

func (s *StorageBlobStateStore) Read(ctx context.Context, req statestore.ReadRequest, resp *statestore.ReadResponse) {
	name := s.resolveBlob(req.StateID)
	dl, err := s.BlobClient(name).DownloadStream(ctx, nil)
	if err != nil {
		if isBlobNotFound(err) {
			// An empty response indicates "no state".
			return
		}
		resp.Diagnostics.AddError("Failed to Read State", fmt.Sprintf("blob %q: %s", name, err))
		return
	}
	body := dl.Body
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to Read State Body", err.Error())
		return
	}
	resp.StateBytes = data
}

func (s *StorageBlobStateStore) Write(ctx context.Context, req statestore.WriteRequest, resp *statestore.WriteResponse) {
	name := s.resolveBlob(req.StateID)
	if s.snapshot {
		bc := s.BlobClient(name)
		if _, err := bc.CreateSnapshot(ctx, nil); err != nil && !isBlobNotFound(err) {
			resp.Diagnostics.AddError("Failed to Snapshot State", err.Error())
			return
		}
	}
	_, err := s.containerClient.NewBlockBlobClient(name).UploadBuffer(ctx, req.StateBytes, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to Write State", fmt.Sprintf("blob %q: %s", name, err))
	}
}

func (s *StorageBlobStateStore) DeleteState(ctx context.Context, req statestore.DeleteStateRequest, resp *statestore.DeleteStateResponse) {
	if req.StateID == "" || req.StateID == "default" {
		resp.Diagnostics.AddError("Cannot Delete Default Workspace", "The default workspace cannot be deleted from a state store.")
		return
	}
	name := s.resolveBlob(req.StateID)
	if _, err := s.BlobClient(name).Delete(ctx, nil); err != nil && !isBlobNotFound(err) {
		resp.Diagnostics.AddError("Failed to Delete State", fmt.Sprintf("blob %q: %s", name, err))
	}
}

func (s *StorageBlobStateStore) Lock(ctx context.Context, req statestore.LockRequest, resp *statestore.LockResponse) {
	lockName := s.resolveBlob(req.StateID) + ".tflock"
	info := statestore.NewLockInfo(req)
	body, err := json.Marshal(info)
	if err != nil {
		resp.Diagnostics.AddError("Failed to Encode Lock Info", err.Error())
		return
	}

	// Conditional create: fail if the lock blob already exists.
	noneMatch := azcore.ETagAny
	_, err = s.containerClient.NewBlockBlobClient(lockName).UploadBuffer(ctx, body, &azblob.UploadBufferOptions{
		AccessConditions: &blob.AccessConditions{
			ModifiedAccessConditions: &blob.ModifiedAccessConditions{
				IfNoneMatch: &noneMatch,
			},
		},
	})
	if err != nil {
		if bloberror.HasCode(err, bloberror.BlobAlreadyExists) ||
			bloberror.HasCode(err, bloberror.ConditionNotMet) ||
			bloberror.HasCode(err, bloberror.LeaseIDMissing) {
			// Read the existing lock info to provide a helpful diagnostic.
			existing, readErr := s.readLockInfo(ctx, lockName)
			if readErr == nil {
				resp.Diagnostics.Append(statestore.WorkspaceAlreadyLockedDiagnostic(req, existing))
				return
			}
		}
		resp.Diagnostics.AddError("Failed to Acquire Lock", fmt.Sprintf("blob %q: %s", lockName, err))
		return
	}
	resp.LockID = info.ID
}

func (s *StorageBlobStateStore) Unlock(ctx context.Context, req statestore.UnlockRequest, resp *statestore.UnlockResponse) {
	lockName := s.resolveBlob(req.StateID) + ".tflock"

	existing, err := s.readLockInfo(ctx, lockName)
	if err != nil {
		if isBlobNotFound(err) {
			resp.Diagnostics.AddError("Lock Not Found", fmt.Sprintf("No lock blob %q exists for state %q.", lockName, req.StateID))
			return
		}
		resp.Diagnostics.AddError("Failed to Read Lock", err.Error())
		return
	}
	if existing.ID != req.LockID {
		resp.Diagnostics.AddError(
			"Lock ID Mismatch",
			fmt.Sprintf("The lock held on state %q (ID %q) does not match the requested unlock ID %q.", req.StateID, existing.ID, req.LockID),
		)
		return
	}
	if _, err := s.BlobClient(lockName).Delete(ctx, nil); err != nil && !isBlobNotFound(err) {
		resp.Diagnostics.AddError("Failed to Release Lock", err.Error())
	}
}

func (s *StorageBlobStateStore) readLockInfo(ctx context.Context, name string) (statestore.LockInfo, error) {
	var info statestore.LockInfo
	dl, err := s.BlobClient(name).DownloadStream(ctx, nil)
	if err != nil {
		return info, err
	}
	defer dl.Body.Close()
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, dl.Body); err != nil {
		return info, err
	}
	if buf.Len() == 0 {
		return info, errors.New("empty lock blob")
	}
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		return info, fmt.Errorf("decoding lock info: %w", err)
	}
	return info, nil
}
