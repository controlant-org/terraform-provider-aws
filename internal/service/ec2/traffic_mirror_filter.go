package ec2

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/terraform-providers/terraform-provider-aws/internal/client"
	"github.com/terraform-providers/terraform-provider-aws/internal/flex"
	"github.com/terraform-providers/terraform-provider-aws/internal/keyvaluetags"
	"github.com/terraform-providers/terraform-provider-aws/internal/tags"
)

func ResourceTrafficMirrorFilter() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsEc2TrafficMirrorFilterCreate,
		Read:   resourceAwsEc2TrafficMirrorFilterRead,
		Update: resourceAwsEc2TrafficMirrorFilterUpdate,
		Delete: resourceAwsEc2TrafficMirrorFilterDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		CustomizeDiff: tags.SetTagsDiff,
		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"network_services": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					ValidateFunc: validation.StringInSlice([]string{
						"amazon-dns",
					}, false),
				},
			},
			"tags":     tags.TagsSchema(),
			"tags_all": tags.TagsSchemaComputed(),
		},
	}
}

func resourceAwsEc2TrafficMirrorFilterCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*client.AWSClient).EC2Conn
	defaultTagsConfig := meta.(*client.AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(keyvaluetags.New(d.Get("tags").(map[string]interface{})))

	input := &ec2.CreateTrafficMirrorFilterInput{}

	if description, ok := d.GetOk("description"); ok {
		input.Description = aws.String(description.(string))
	}

	if len(tags) > 0 {
		input.TagSpecifications = ec2TagSpecificationsFromKeyValueTags(tags, ec2.ResourceTypeTrafficMirrorFilter)
	}

	out, err := conn.CreateTrafficMirrorFilter(input)
	if err != nil {
		return fmt.Errorf("Error while creating traffic filter %s", err)
	}

	d.SetId(aws.StringValue(out.TrafficMirrorFilter.TrafficMirrorFilterId))

	if v, ok := d.GetOk("network_services"); ok {
		input := &ec2.ModifyTrafficMirrorFilterNetworkServicesInput{
			TrafficMirrorFilterId: aws.String(d.Id()),
			AddNetworkServices:    flex.ExpandStringSet(v.(*schema.Set)),
		}

		_, err := conn.ModifyTrafficMirrorFilterNetworkServices(input)
		if err != nil {
			return fmt.Errorf("error modifying EC2 Traffic Mirror Filter (%s) network services: %w", d.Id(), err)
		}

	}

	return resourceAwsEc2TrafficMirrorFilterRead(d, meta)
}

func resourceAwsEc2TrafficMirrorFilterUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*client.AWSClient).EC2Conn

	if d.HasChange("network_services") {
		input := &ec2.ModifyTrafficMirrorFilterNetworkServicesInput{
			TrafficMirrorFilterId: aws.String(d.Id()),
		}

		o, n := d.GetChange("network_services")
		newServices := n.(*schema.Set).Difference(o.(*schema.Set))
		if newServices.Len() > 0 {
			input.AddNetworkServices = flex.ExpandStringSet(newServices)
		}

		removeServices := o.(*schema.Set).Difference(n.(*schema.Set))
		if removeServices.Len() > 0 {
			input.RemoveNetworkServices = flex.ExpandStringSet(removeServices)
		}

		_, err := conn.ModifyTrafficMirrorFilterNetworkServices(input)
		if err != nil {
			return fmt.Errorf("error modifying EC2 Traffic Mirror Filter (%s) network services: %w", d.Id(), err)
		}
	}

	if d.HasChange("tags_all") {
		o, n := d.GetChange("tags_all")

		if err := keyvaluetags.Ec2UpdateTags(conn, d.Id(), o, n); err != nil {
			return fmt.Errorf("error updating EC2 Traffic Mirror Filter (%s) tags: %s", d.Id(), err)
		}
	}

	return resourceAwsEc2TrafficMirrorFilterRead(d, meta)
}

func resourceAwsEc2TrafficMirrorFilterRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*client.AWSClient).EC2Conn
	defaultTagsConfig := meta.(*client.AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*client.AWSClient).IgnoreTagsConfig

	input := &ec2.DescribeTrafficMirrorFiltersInput{
		TrafficMirrorFilterIds: aws.StringSlice([]string{d.Id()}),
	}

	out, err := conn.DescribeTrafficMirrorFilters(input)

	if tfawserr.ErrMessageContains(err, "InvalidTrafficMirrorFilterId.NotFound", "") {
		log.Printf("[WARN] EC2 Traffic Mirror Filter (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return fmt.Errorf("Error describing traffic mirror filter %v: %v", d.Id(), err)
	}

	if len(out.TrafficMirrorFilters) == 0 {
		log.Printf("[WARN] EC2 Traffic Mirror Filter (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	trafficMirrorFilter := out.TrafficMirrorFilters[0]
	d.Set("description", trafficMirrorFilter.Description)

	tags := keyvaluetags.Ec2KeyValueTags(trafficMirrorFilter.Tags).IgnoreAws().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return fmt.Errorf("error setting tags: %w", err)
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return fmt.Errorf("error setting tags_all: %w", err)
	}

	if err := d.Set("network_services", aws.StringValueSlice(trafficMirrorFilter.NetworkServices)); err != nil {
		return fmt.Errorf("error setting network_services for filter %v: %s", d.Id(), err)
	}

	arn := arn.ARN{
		Partition: meta.(*client.AWSClient).Partition,
		Service:   ec2.ServiceName,
		Region:    meta.(*client.AWSClient).Region,
		AccountID: meta.(*client.AWSClient).AccountID,
		Resource:  fmt.Sprintf("traffic-mirror-filter/%s", d.Id()),
	}.String()

	d.Set("arn", arn)

	return nil
}

func resourceAwsEc2TrafficMirrorFilterDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*client.AWSClient).EC2Conn

	input := &ec2.DeleteTrafficMirrorFilterInput{
		TrafficMirrorFilterId: aws.String(d.Id()),
	}

	_, err := conn.DeleteTrafficMirrorFilter(input)
	if err != nil {
		return fmt.Errorf("Error deleting traffic mirror filter %v: %v", d.Id(), err)
	}

	return nil
}