package wafv2_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/wafv2"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/terraform-providers/terraform-provider-aws/internal/acctest"
	"github.com/terraform-providers/terraform-provider-aws/internal/client"
	"github.com/terraform-providers/terraform-provider-aws/internal/service/apigateway"
)

func TestAccAwsWafv2WebACLAssociation_basic(t *testing.T) {
	testName := fmt.Sprintf("web-acl-association-%s", sdkacctest.RandString(5))
	resourceName := "aws_wafv2_web_acl_association.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			testAccAPIGatewayTypeEDGEPreCheck(t)
			testAccPreCheckAWSWafv2ScopeRegional(t)
		},
		ErrorCheck:   acctest.ErrorCheck(t, wafv2.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSWafv2WebACLAssociationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsWafv2WebACLAssociationConfig(testName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSWafv2WebACLAssociationExists(resourceName),
					acctest.MatchResourceAttrRegionalARNNoAccount(resourceName, "resource_arn", "apigateway", regexp.MustCompile(fmt.Sprintf("/restapis/.*/stages/%s", testName))),
					acctest.MatchResourceAttrRegionalARN(resourceName, "web_acl_arn", "wafv2", regexp.MustCompile(fmt.Sprintf("regional/webacl/%s/.*", testName))),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccAWSWafv2WebACLAssociationImportStateIdFunc(resourceName),
			},
		},
	})
}

func TestAccAwsWafv2WebACLAssociation_Disappears(t *testing.T) {
	testName := fmt.Sprintf("web-acl-association-%s", sdkacctest.RandString(5))
	resourceName := "aws_wafv2_web_acl_association.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			testAccAPIGatewayTypeEDGEPreCheck(t)
			testAccPreCheckAWSWafv2ScopeRegional(t)
		},
		ErrorCheck:   acctest.ErrorCheck(t, wafv2.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSWafv2WebACLAssociationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsWafv2WebACLAssociationConfig(testName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSWafv2WebACLAssociationExists(resourceName),
					acctest.CheckResourceDisappears(acctest.Provider, ResourceWebACLAssociation(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccAwsWafv2WebACLAssociationConfig(testName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSWafv2WebACLAssociationExists(resourceName),
					acctest.CheckResourceDisappears(acctest.Provider, apigateway.ResourceStage(), "aws_api_gateway_stage.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckAWSWafv2WebACLAssociationDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_wafv2_web_acl_association" {
			continue
		}

		conn := acctest.Provider.Meta().(*client.AWSClient).WAFV2Conn
		resp, err := conn.GetWebACLForResource(&wafv2.GetWebACLForResourceInput{
			ResourceArn: aws.String(rs.Primary.Attributes["resource_arn"]),
		})

		if err == nil {
			if resp == nil || resp.WebACL == nil {
				return fmt.Errorf("Error getting WAFv2 WebACLAssociation")
			}

			id := fmt.Sprintf("%s,%s", aws.StringValue(resp.WebACL.ARN), rs.Primary.Attributes["resource_arn"])
			if id == rs.Primary.ID {
				return fmt.Errorf("WAFv2 WebACLAssociation %s still exists", rs.Primary.ID)
			}
			return nil
		}

		// Return nil if the Web ACL Association is already destroyed
		if tfawserr.ErrMessageContains(err, wafv2.ErrCodeWAFNonexistentItemException, "") {
			return nil
		}

		return err
	}

	return nil
}

func testAccCheckAWSWafv2WebACLAssociationExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("Not found: %s", name)
		}
		return nil
	}
}

func testAccAwsWafv2WebACLAssociationConfig(name string) string {
	return fmt.Sprintf(`
resource "aws_api_gateway_stage" "test" {
  stage_name    = "%s"
  rest_api_id   = aws_api_gateway_rest_api.test.id
  deployment_id = aws_api_gateway_deployment.test.id
}

resource "aws_api_gateway_rest_api" "test" {
  name = "%s"
}

resource "aws_api_gateway_deployment" "test" {
  rest_api_id = aws_api_gateway_rest_api.test.id
  depends_on  = [aws_api_gateway_integration.test]
}

resource "aws_api_gateway_integration" "test" {
  rest_api_id = aws_api_gateway_rest_api.test.id
  resource_id = aws_api_gateway_resource.test.id
  http_method = aws_api_gateway_method.test.http_method
  type        = "MOCK"
}

resource "aws_api_gateway_resource" "test" {
  rest_api_id = aws_api_gateway_rest_api.test.id
  parent_id   = aws_api_gateway_rest_api.test.root_resource_id
  path_part   = "mytestresource"
}

resource "aws_api_gateway_method" "test" {
  rest_api_id   = aws_api_gateway_rest_api.test.id
  resource_id   = aws_api_gateway_resource.test.id
  http_method   = "GET"
  authorization = "NONE"
}

resource "aws_wafv2_web_acl" "test" {
  name  = "%s"
  scope = "REGIONAL"

  default_action {
    allow {}
  }

  visibility_config {
    cloudwatch_metrics_enabled = false
    metric_name                = "friendly-metric-name"
    sampled_requests_enabled   = false
  }
}

resource "aws_wafv2_web_acl_association" "test" {
  resource_arn = aws_api_gateway_stage.test.arn
  web_acl_arn  = aws_wafv2_web_acl.test.arn
}
`, name, name, name)
}

func testAccAWSWafv2WebACLAssociationImportStateIdFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", resourceName)
		}

		return fmt.Sprintf("%s,%s", rs.Primary.Attributes["web_acl_arn"], rs.Primary.Attributes["resource_arn"]), nil
	}
}