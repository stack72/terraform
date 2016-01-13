package azurerm

import (
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceArmVirtualNetworkGateway() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmVirtualNetworkGatewayCreate,
		Read:   resourceArmVirtualNetworkGatewayRead,
		Update: resourceArmVirtualNetworkGatewayUpdate,
		Delete: resourceArmVirtualNetworkGatewayDelete,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"location": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"resource_group_name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"gateway_type": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},

			"gateway_size": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},

			"bgp_enabled": &schema.Schema{
				Type:     schema.TypeBool,
				Required: true,
			},

			"vpn_client_address_pool": &schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"default_sites": &schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"vip_address": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceArmVirtualNetworkGatewayCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient)
	vnetGatewayClient := client.vnetGatewayClient

	log.Printf("[INFO] preparing arguments for Azure ARM virtual network gateway creation.")

	name := d.Get("name").(string)
	location := d.Get("location").(string)
	resGroup := d.Get("resource_group_name").(string)
	vnetGatewayType := d.Get("gateway_type").(string)
	vnetGatewaySize := d.Get("gateway_size").(string)
	bgp_enabled := d.Get("bgp_enabled").(bool)
	defaultSites := d.Get("default_site").(string)

	vpnClientAddress := d.Get("vpn_client_address_pool")
	var vpn_prefixes []string
	addresses := vpnClientAddress.(*schema.Set).List()
	for _, address := range addresses {
		prefix := address.(string)
		vpn_prefixes = append(vpn_prefixes, prefix)
	}

	vpnConfiguration := network.VpnClientConfiguration{
		VpnClientAddressPool: &network.AddressSpace{},
	}

	vnetGateway := network.VirtualNetworkGateway{
		Name:     &name,
		Location: &location,
		Properties: &network.VirtualNetworkGatewayPropertiesFormat{
			GatewayType:            network.VirtualNetworkGatewayType(vnetGatewayType),
			EnableBgp:              &bgp_enabled,
			VpnClientConfiguration: &vpnConfiguration,
		},
	}

	if v, ok := d.GetOk("default_sites"); ok {
		var default_sites []string
		sites := v.(*schema.Set).List()
		for _, site := range sites {
			str := site.(string)
			default_sites = append(default_sites, str)
		}
		//vnetGateway.Properties. = &dnsServers
	}

	resp, err := vnetGatewayClient.CreateOrUpdate(resGroup, name, vnetGateway)
	if err != nil {
		return err
	}

	d.SetId(*resp.ID)

	log.Printf("[DEBUG] Waiting for Virtual Network Gateway (%s) to become available", name)
	stateConf := &resource.StateChangeConf{
		Pending: []string{"Accepted", "Updating"},
		Target:  "Succeeded",
		Refresh: virtualNetworkGatewayGroupStateRefreshFunc(client, resGroup, name),
		Timeout: 10 * time.Minute,
	}
	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf("Error waiting for Virtual Network Gateway (%s) to become available: %s", name, err)
	}

	return resourceArmVirtualNetworkGatewayRead(d, meta)
}

func resourceArmVirtualNetworkGatewayRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceArmVirtualNetworkGatewayDelete(d *schema.ResourceData, meta interface{}) error {
	vnetGatewayClient := meta.(*ArmClient).vnetGatewayClient

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	name := id.Path["virtualNetworkGateways"]

	_, err = vnetGatewayClient.Delete(resGroup, name)

	return err
}

func virtualNetworkGatewayGroupStateRefreshFunc(client *ArmClient, resourceGroupName string, virtualNetworkGatewayName string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		res, err := client.vnetGatewayClient.Get(resourceGroupName, virtualNetworkGatewayName)
		if err != nil {
			return nil, "", fmt.Errorf("Error issuing read request in virtualNetworkGatewayGroupStateRefreshFunc to Azure ARM for virtual network gateway '%s' (RG: '%s'): %s", virtualNetworkGatewayName, resourceGroupName, err)
		}

		return res, *res.Properties.ProvisioningState, nil
	}
}
