package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/ms-henglu/terraform-provider-azurermg/internal/azure"
	"github.com/ms-henglu/terraform-provider-azurermg/internal/clients"
	"github.com/ms-henglu/terraform-provider-azurermg/internal/services/parse"
	"github.com/ms-henglu/terraform-provider-azurermg/internal/tf"
	"github.com/ms-henglu/terraform-provider-azurermg/utils"
)

func ResourceAzureGenericResource() *schema.Resource {
	return &schema.Resource{
		Create: resourceAzureGenericResourceCreateUpdate,
		Read:   resourceAzureGenericResourceRead,
		Update: resourceAzureGenericResourceCreateUpdate,
		Delete: resourceAzureGenericResourceDelete,

		Importer: tf.DefaultImporter(func(id string) error {
			_, err := parse.ResourceID(id)
			return err
		}),

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Read:   schema.DefaultTimeout(5 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"url": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},

			"api_version": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},

			"location": azure.SchemaLocation(),

			"identity": azure.SchemaIdentity(),

			"body": {
				Type:             schema.TypeString,
				Required:         true,
				ValidateFunc:     validation.StringIsJSON,
				DiffSuppressFunc: tf.SuppressJsonOrderingDifference,
			},

			"create_method": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "PUT",
				ValidateFunc: validation.StringInSlice([]string{
					http.MethodPost,
					http.MethodPut,
				}, false),
			},

			"update_method": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "PUT",
				ValidateFunc: validation.StringInSlice([]string{
					http.MethodPost,
					http.MethodPut,
					http.MethodPatch,
				}, false),
			},

			"response_body": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"tags": azure.SchemaTags(),
		},
		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, i interface{}) error {
			if d.HasChange("identity") {
				d.SetNewComputed("response_body")
			}
			old, new := d.GetChange("body")
			if utils.NormalizeJson(old) != utils.NormalizeJson(new) {
				d.SetNewComputed("response_body")
			}
			if d.HasChange("tags") {
				d.SetNewComputed("response_body")
			}
			return nil
		},
	}
}

func resourceAzureGenericResourceCreateUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).ResourceClient
	ctx, cancel := tf.ForCreateUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id := parse.NewResourceID(d.Get("url").(string), d.Get("api_version").(string))

	if d.IsNewResource() {
		existing, response, err := client.Get(ctx, id.Url, id.ApiVersion)
		if err != nil {
			if response.StatusCode != http.StatusNotFound {
				return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
			}
		}
		if len(utils.GetId(existing)) > 0 {
			return tf.ImportAsExistsError("azurermg_resource", id.ID())
		}
	}

	var requestBody interface{}
	err := json.Unmarshal([]byte(d.Get("body").(string)), &requestBody)
	if err != nil {
		return err
	}

	if tags, ok := d.GetOk("tags"); ok {
		bodyWithTags := azure.ExpandTags(tags.(map[string]interface{}))
		requestBody = utils.GetMergedJson(requestBody, bodyWithTags)
	}
	if location, ok := d.GetOk("location"); ok {
		bodyWithLocation := azure.ExpandLocation(location.(string))
		requestBody = utils.GetMergedJson(requestBody, bodyWithLocation)
	}
	if identity, ok := d.GetOk("identity"); ok {
		j, _ := json.Marshal(identity)
		log.Printf("[INFO] identity: %v\n", string(j))
		bodyWithIdentity, err := azure.ExpandIdentity(identity.([]interface{}))
		if err != nil {
			return err
		}
		requestBody = utils.GetMergedJson(requestBody, bodyWithIdentity)
	}

	var method string
	switch {
	case d.IsNewResource():
		method = d.Get("create_method").(string)
	default:
		method = d.Get("update_method").(string)
	}

	j, _ := json.Marshal(requestBody)
	log.Printf("[INFO] body: %v\n", string(j))
	_, _, err = client.CreateUpdate(ctx, id.Url, id.ApiVersion, requestBody, method)
	if err != nil {
		return fmt.Errorf("creating/updating %q: %+v", id, err)
	}

	d.SetId(id.ID())

	return resourceAzureGenericResourceRead(d, meta)
}

func resourceAzureGenericResourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).ResourceClient
	ctx, cancel := tf.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.ResourceID(d.Id())
	if err != nil {
		return err
	}

	responseBody, response, err := client.Get(ctx, id.Url, id.ApiVersion)
	if err != nil {
		if response.StatusCode == http.StatusNotFound {
			log.Printf("[INFO] Error reading %q - removing from state", id.ID())
			d.SetId("")
			return nil
		}

		return fmt.Errorf("reading %q: %+v", id, err)
	}

	d.Set("url", id.Url)
	d.Set("api_version", id.ApiVersion)

	bodyJson := d.Get("body").(string)
	var requestBody interface{}
	err = json.Unmarshal([]byte(bodyJson), &requestBody)
	if err != nil {
		return err
	}
	data, err := json.Marshal(utils.GetUpdatedJson(requestBody, responseBody))
	if err != nil {
		return err
	}
	d.Set("body", string(data))
	d.Set("tags", azure.FlattenTags(responseBody))
	d.Set("location", azure.FlattenLocation(responseBody))
	d.Set("identity", azure.FlattenIdentity(responseBody))

	responseBodyJson, err := json.Marshal(responseBody)
	d.Set("response_body", string(responseBodyJson))
	return nil
}

func resourceAzureGenericResourceDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).ResourceClient
	ctx, cancel := tf.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.ResourceID(d.Id())
	if err != nil {
		return err
	}

	_, _, err = client.Delete(ctx, id.Url, id.ApiVersion)
	if err != nil {
		return fmt.Errorf("deleting %q: %+v", id, err)
	}

	return nil
}
